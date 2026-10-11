package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cap"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTurnstileCheckCapJsFailsClosedAndConsumesToken(t *testing.T) {
	previousEnabled, previousProvider := common.BotProtectionEnabled, common.BotProtectionProvider
	previousCap, previousTurnstile := common.CapJsCheckEnabled, common.TurnstileCheckEnabled
	previousRedis := common.RedisEnabled
	common.BotProtectionEnabled = true
	common.BotProtectionProvider = common.BotProtectionProviderCapJs
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.BotProtectionEnabled, common.BotProtectionProvider = previousEnabled, previousProvider
		common.CapJsCheckEnabled, common.TurnstileCheckEnabled = previousCap, previousTurnstile
		common.RedisEnabled = previousRedis
	})

	valid := "builtin:middleware-valid:proof"
	expired := "builtin:middleware-expired:proof"
	require.NoError(t, cap.StoreRedeemToken(valid, time.Now().Add(time.Minute).UnixMilli()))
	require.NoError(t, cap.StoreRedeemToken(expired, time.Now().Add(-time.Minute).UnixMilli()))
	router := gin.New()
	router.GET("/protected", TurnstileCheck(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for _, tc := range []struct {
		name, token string
		allowed     bool
	}{
		{name: "missing"},
		{name: "malformed", token: "builtin:missing"},
		{name: "unissued", token: "builtin:unknown:proof"},
		{name: "expired", token: expired},
		{name: "valid", token: valid, allowed: true},
		{name: "replay", token: valid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/protected?turnstile="+url.QueryEscape(tc.token), nil))
			if tc.allowed {
				assert.Equal(t, http.StatusNoContent, response.Code)
				return
			}
			assert.Equal(t, http.StatusOK, response.Code)
			var body struct {
				Success    bool   `json:"success"`
				MessageKey string `json:"message_key"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
			assert.False(t, body.Success)
			assert.NotEmpty(t, body.MessageKey)
		})
	}
}

func TestVerifyCapJsTokenRemoteRequiresServerSuccess(t *testing.T) {
	previousEndpoint, previousSecret := common.CapJsApiEndpoint, common.CapJsSecretKey
	t.Cleanup(func() { common.CapJsApiEndpoint, common.CapJsSecretKey = previousEndpoint, previousSecret })
	common.CapJsSecretKey = "middleware-remote-secret"
	for _, tc := range []struct {
		name, response string
		allowed        bool
	}{
		{name: "rejected", response: `{"success":false}`},
		{name: "missing verdict", response: `{}`},
		{name: "malformed verdict", response: `not-json`},
		{name: "verified", response: `{"success":true}`, allowed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/siteverify", r.URL.Path)
				var payload map[string]string
				require.NoError(t, common.DecodeJson(r.Body, &payload))
				assert.Equal(t, common.CapJsSecretKey, payload["secret"])
				assert.Equal(t, "remote:challenge:proof", payload["response"])
				_, _ = w.Write([]byte(tc.response))
			}))
			defer server.Close()
			common.CapJsApiEndpoint = server.URL
			ok, _ := verifyCapJsTokenRemote("remote:challenge:proof")
			assert.Equal(t, tc.allowed, ok)
		})
	}
}
