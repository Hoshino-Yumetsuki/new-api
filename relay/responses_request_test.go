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
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/requestplugin"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/request_plugin_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
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

func TestPrepareResponsesRequestDeepSeekEffectiveReasoning(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	previous := settings.PassThroughRequestEnabled
	t.Cleanup(func() { settings.PassThroughRequestEnabled = previous })
	settings.PassThroughRequestEnabled = false
	const input = `[{"type":"function_call","call_id":"c1","name":"lookup","arguments":"{}"}]`
	for _, tc := range []struct {
		name, effort, override, endpoint         string
		passthrough, replaceInput, wantReasoning bool
		globalPassthrough                        bool
	}{
		{name: "override enables thinking", effort: "none", override: "high", wantReasoning: true},
		{name: "override disables thinking", effort: "high", override: "none"},
		{name: "override replaces history", effort: "high", override: "high", replaceInput: true, wantReasoning: true},
		{name: "no override", effort: "high", wantReasoning: true},
		{name: "thinking disabled", effort: "none"},
		{name: "third party endpoint", effort: "none", override: "high", endpoint: "https://other.example"},
		{name: "passthrough bypasses overrides and normalization", effort: "none", override: "high", passthrough: true},
		{name: "global passthrough bypasses normalization", effort: "high", globalPassthrough: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings.PassThroughRequestEnabled = tc.globalPassthrough
			endpoint := tc.endpoint
			if endpoint == "" {
				endpoint = "https://API.DEEPSEEK.COM:443/v1"
			}
			raw := `{"model":"deepseek-flash","reasoning":{"effort":"` + tc.effort + `"},"input":` + input + `}`
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(raw))
			c.Request.Header.Set("Content-Type", "application/json")
			t.Cleanup(func() { common.CleanupBodyStorage(c) })
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "deepseek-flash")
			common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeDeepSeek)
			common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, endpoint)
			common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: tc.passthrough})
			if tc.override != "" {
				ops := []any{map[string]any{"path": "reasoning.effort", "mode": "set", "value": tc.override}, map[string]any{"path": "vendor_extension", "mode": "set", "value": "preserved"}}
				if tc.replaceInput {
					var replacement any
					require.NoError(t, common.Unmarshal([]byte(input), &replacement))
					ops = append(ops, map[string]any{"path": "input", "mode": "set", "value": replacement})
				}
				common.SetContextKey(c, constant.ContextKeyChannelParamOverride, map[string]any{"operations": ops})
			}
			var req dto.OpenAIResponsesRequest
			require.NoError(t, common.Unmarshal([]byte(raw), &req))
			info := relaycommon.GenRelayInfoResponses(c, &req)
			_, body, closer, apiErr := PrepareResponsesRequest(c, info, &req)
			require.Nil(t, apiErr)
			t.Cleanup(func() { require.NoError(t, closer.Close()) })
			wire, err := io.ReadAll(body)
			require.NoError(t, err)
			if tc.passthrough || tc.globalPassthrough {
				assert.JSONEq(t, raw, string(wire))
				return
			}
			wantEffort := tc.effort
			if tc.override != "" {
				wantEffort = tc.override
				assert.Equal(t, "preserved", gjson.GetBytes(wire, "vendor_extension").String())
			}
			assert.Equal(t, wantEffort, gjson.GetBytes(wire, "reasoning.effort").String())
			if tc.wantReasoning {
				assert.Equal(t, "reasoning", gjson.GetBytes(wire, "input.0.type").String())
				assert.Equal(t, " ", gjson.GetBytes(wire, "input.0.content.0.text").String())
				assert.Equal(t, "function_call", gjson.GetBytes(wire, "input.1.type").String())
			} else {
				assert.JSONEq(t, input, gjson.GetBytes(wire, "input").Raw)
			}
		})
	}
}

// finalizedEgressPlugin rejects a body whose first input item is reasoning,
// which only the DeepSeek finalizer adds to the test request.
const finalizedEgressPlugin = `export const meta = {apiVersion: 1, key: "rp-finalized-egress", name: "Finalized egress", version: "1.0.0", author: {name: "Test"}};
export function egress(ctx, req) {
  const input = req.body && req.body.input;
  if (Array.isArray(input) && input.length && input[0].type === "reasoning") {
    return {action: "reject", code: "finalized.seen", message: "Egress saw the finalized body"};
  }
  return {action: "continue"};
}`

func TestPrepareResponsesRequestEgressReadsTheFinalizedBody(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	previous := settings.PassThroughRequestEnabled
	t.Cleanup(func() { settings.PassThroughRequestEnabled = previous })
	settings.PassThroughRequestEnabled = false
	require.NoError(t, i18n.Init())
	plugin, err := jsplugin.DefaultRegistry.Register(finalizedEgressPlugin, jsplugin.Options{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = jsplugin.DefaultRegistry.Unregister(plugin.Meta.Key) })
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{request_plugin_setting.InstancesOption: `[{"id":"fe","name":"Finalized","plugin":"rp-finalized-egress","enabled":true,"global":true}]`}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{request_plugin_setting.InstancesOption: "[]"}))
	})

	raw := `{"model":"deepseek-flash","reasoning":{"effort":"high"},"input":[{"type":"function_call","call_id":"c1","name":"lookup","arguments":"{}"}]}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	t.Cleanup(func() { common.CleanupBodyStorage(c) })
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "deepseek-flash")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeDeepSeek)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://api.deepseek.com")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{})
	_, _, apiErr := requestplugin.Ingress(c, types.RelayFormatOpenAIResponses)
	require.Nil(t, apiErr)
	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal([]byte(raw), &req))
	info := relaycommon.GenRelayInfoResponses(c, &req)
	_, _, closer, apiErr := PrepareResponsesRequest(c, info, &req)
	if closer != nil {
		t.Cleanup(func() { _ = closer.Close() })
	}
	require.NotNil(t, apiErr, "egress must read the reasoning item the finalizer added")
	assert.Equal(t, types.ErrorCodeRequestRejected, apiErr.GetErrorCode())
}
