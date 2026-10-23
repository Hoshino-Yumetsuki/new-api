package relay

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareResponsesRequestCompactionInput(t *testing.T) {
	const history = `[
		{"type":"configuration_update","reasoning":{"effort":"high"}},
		{"role":"user","content":[{"type":"input_text","text":"configuration_update"}]},
		{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"},
		{"type":"configuration_update","reasoning":{"effort":"low"}},
		{"type":"function_call_output","call_id":"call_1","output":"found"},
		{"type":"compaction","encrypted_content":"opaque-history"},
		{"type":"future_item","vendor":{"type":"configuration_update","value":9007199254740993}}
	]`
	const compactHistory = `[
		{"role":"user","content":[{"type":"input_text","text":"configuration_update"}]},
		{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_1","output":"found"},
		{"type":"compaction","encrypted_content":"opaque-history"},
		{"type":"future_item","vendor":{"type":"configuration_update","value":9007199254740993}}
	]`
	for _, channelType := range []int{constant.ChannelTypeOpenAI, constant.ChannelTypeCodex} {
		channelName := "openai"
		if channelType == constant.ChannelTypeCodex {
			channelName = "codex"
		}
		t.Run(channelName, func(t *testing.T) {
			for _, tc := range []struct {
				name        string
				input       string
				want        string
				responses   bool
				passthrough bool
			}{
				{name: "compact history", input: history, want: compactHistory},
				{name: "only configuration updates", input: `[{"type":"configuration_update","reasoning":{"effort":"high"}}]`, want: `[]`},
				{name: "text input", input: `"configuration_update"`, want: `"configuration_update"`},
				{name: "empty history", input: `[]`, want: `[]`},
				{name: "null input", input: `null`, want: `null`},
				{name: "unchanged history", input: compactHistory, want: compactHistory},
				{name: "regular responses retains controls", input: history, want: history, responses: true},
				{name: "passthrough retains controls", input: history, want: history, passthrough: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					path := "/v1/responses/compact"
					if tc.responses {
						path = "/v1/responses"
					}
					request := &dto.OpenAIResponsesRequest{
						Model:        "gpt-5-codex",
						Input:        json.RawMessage(tc.input),
						Instructions: json.RawMessage(`"Keep the user instructions"`),
					}
					rawBody, err := common.Marshal(request)
					require.NoError(t, err)
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(rawBody)))
					c.Request.Header.Set("Content-Type", "application/json")
					common.SetContextKey(c, constant.ContextKeyOriginalModel, request.Model)
					common.SetContextKey(c, constant.ContextKeyChannelType, channelType)
					common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: tc.passthrough})
					info := relaycommon.GenRelayInfoResponses(c, request)
					if !tc.responses {
						info.RelayMode = relayconstant.RelayModeResponsesCompact
					}
					_, body, closer, apiErr := PrepareResponsesRequest(c, info, request)
					require.Nil(t, apiErr)
					t.Cleanup(func() { assert.NoError(t, closer.Close()) })
					if tc.passthrough {
						storage, err := common.GetBodyStorage(c)
						require.NoError(t, err)
						t.Cleanup(func() { assert.NoError(t, storage.Close()) })
					}
					outbound, err := io.ReadAll(body)
					require.NoError(t, err)
					var converted dto.OpenAIResponsesRequest
					require.NoError(t, common.Unmarshal(outbound, &converted))
					assert.JSONEq(t, tc.want, string(converted.Input))
					if strings.Contains(tc.input, "9007199254740993") {
						assert.Contains(t, string(converted.Input), "9007199254740993", "unknown item payloads must retain numeric precision")
					}
					assert.Equal(t, request.Instructions, converted.Instructions)
					assert.Equal(t, json.RawMessage(tc.input), request.Input, "channel attempts must not mutate replay history")
				})
			}
		})
	}
}
