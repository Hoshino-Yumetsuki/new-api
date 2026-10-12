package claude

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/tokenkit"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func commonPointer[T any](value T) *T {
	return &value
}

func TestResponseOpenAI2ClaudeToolUseInputIsObject(t *testing.T) {
	tests := []struct {
		name string
		args string
		want map[string]any
	}{
		{name: "object", args: `{"q":"x"}`, want: map[string]any{"q": "x"}},
		{name: "empty", args: "", want: map[string]any{}},
		{name: "invalid", args: "{", want: map[string]any{}},
		{name: "null", args: "null", want: map[string]any{}},
		{name: "array", args: `["x"]`, want: map[string]any{}},
		{name: "string", args: `"x"`, want: map[string]any{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := dto.Message{Role: "assistant"}
			msg.SetToolCalls([]dto.ToolCallRequest{
				{
					ID:   "call_1",
					Type: "function",
					Function: dto.FunctionRequest{
						Name:      "lookup",
						Arguments: tt.args,
					},
				},
			})
			resp := relayconvert.ResponseOpenAI2Claude(&dto.OpenAITextResponse{
				Id:    "chatcmpl_1",
				Model: "gpt-test",
				Choices: []dto.OpenAITextResponseChoice{
					{Message: msg, FinishReason: "tool_calls"},
				},
			}, nil)

			require.Len(t, resp.Content, 1)
			assert.Equal(t, "tool_use", resp.Content[0].Type)
			assert.Equal(t, tt.want, resp.Content[0].Input)
		})
	}
}

func TestClaudeStreamHandlerEventDataFraming(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	events := []struct{ name, data string }{
		{"message_start", `{"type":"message_start","message":{"id":"msg_stream","model":"claude-3-5-sonnet","type":"message","role":"assistant","content":[],"usage":{"input_tokens":100,"output_tokens":1,"cache_read_input_tokens":300,"cache_creation_input_tokens":50}}}`},
		{"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`},
		{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"literal event: message_startdata: stays unchanged"}}`},
		{"content_block_stop", `{"type":"content_block_stop","index":0}`},
		{"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":20}}`},
		{"message_stop", `{"type":"message_stop"}`},
	}
	for _, tc := range []struct {
		name       string
		newline    string
		joined     int
		fragmented bool
	}{
		{name: "standard LF", newline: "\n"},
		{name: "standard CRLF", newline: "\r\n"},
		{name: "joined message start", newline: "\n", joined: 1},
		{name: "all event data pairs joined", newline: "\n", joined: len(events)},
		{name: "joined pairs across reads and EOF", newline: "\r\n", joined: len(events), fragmented: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var upstream strings.Builder
			// Only an event field may contain the missing event/data separator.
			for _, line := range []string{
				`: data: {"type":"error","error":{"type":"api_error"}}`,
				`id: data: {"type":"error","error":{"type":"api_error"}}`,
				`retry: data: {"type":"error","error":{"type":"api_error"}}`,
			} {
				upstream.WriteString(line + tc.newline)
			}
			for i, event := range events {
				upstream.WriteString("event: " + event.name)
				if i >= tc.joined {
					upstream.WriteString(tc.newline)
				}
				upstream.WriteString("data: " + event.data + tc.newline + tc.newline)
			}
			var body io.Reader = strings.NewReader(upstream.String())
			if tc.fragmented {
				body = iotest.OneByteReader(strings.NewReader(strings.TrimRight(upstream.String(), "\r\n")))
			}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages?beta=true", nil)
			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-3-5-sonnet"},
				RelayFormat: types.RelayFormatClaude,
				IsStream:    true,
				DisablePing: true,
			}
			info.SetEstimatePromptTokens(1_000_000)
			usage, apiErr := ClaudeStreamHandler(c, &http.Response{Body: io.NopCloser(body)}, info)
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			assert.Equal(t, 100, usage.PromptTokens)
			assert.Equal(t, 20, usage.CompletionTokens)
			assert.Equal(t, 300, usage.PromptTokensDetails.CachedTokens)
			assert.Equal(t, 50, usage.PromptTokensDetails.CachedCreationTokens)
			require.NotNil(t, usage.BillingUsage)
			billable, ok := usage.BillingUsage.CanonicalUsage()
			require.True(t, ok)
			assert.Equal(t, 100, billable.PromptTokens)
			assert.Equal(t, 20, billable.CompletionTokens)
			assert.Equal(t, 300, billable.PromptTokensDetails.CachedTokens)
			assert.Equal(t, 50, billable.PromptTokensDetails.CachedCreationTokens)
			assert.False(t, usage.BillingUsage.Estimated)
			assert.False(t, common.GetContextKeyBool(c, constant.ContextKeyLocalCountTokens))
			assert.Equal(t, string(relaycommon.ResponseOutcomeCompleted), info.StreamStatus.ResponseOutcome())

			var eventName string
			var received []string
			for line := range strings.SplitSeq(w.Body.String(), "\n") {
				if name, ok := strings.CutPrefix(line, "event: "); ok {
					eventName = name
				} else if data, ok := strings.CutPrefix(line, "data: "); ok {
					var event dto.ClaudeResponse
					require.NoError(t, common.UnmarshalJsonStr(data, &event))
					assert.Equal(t, event.Type, eventName)
					received = append(received, event.Type)
					if event.Type == "content_block_delta" {
						require.NotNil(t, event.Delta)
						require.NotNil(t, event.Delta.Text)
						assert.Equal(t, "literal event: message_startdata: stays unchanged", *event.Delta.Text)
					}
					eventName = ""
				}
			}
			assert.Equal(t, []string{"message_start", "content_block_start", "content_block_delta", "content_block_stop", "message_delta", "message_stop"}, received)
		})
	}
}

type claudeStreamReadError struct{}

func (claudeStreamReadError) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func TestClaudeStreamHandlerInterruptedUsage(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	for _, tc := range []struct {
		name       string
		startUsage string
		tail       string
		readError  bool
		wantInput  int
		wantOutput int
		wantCache  int
		wantError  bool
		wantLocal  bool
	}{
		{name: "partial usage survives EOF", startUsage: `{"input_tokens":100,"output_tokens":1,"cache_read_input_tokens":300}`, wantInput: 100, wantOutput: 1, wantCache: 300},
		{name: "partial usage survives transport failure", startUsage: `{"input_tokens":100,"output_tokens":1,"cache_read_input_tokens":300}`, readError: true, wantInput: 100, wantOutput: 1, wantCache: 300},
		{name: "cached input permits zero uncached input", startUsage: `{"input_tokens":0,"output_tokens":1,"cache_read_input_tokens":300}`, readError: true, wantOutput: 1, wantCache: 300},
		{name: "cache-only usage survives transport failure", startUsage: `{"input_tokens":0,"output_tokens":0,"cache_read_input_tokens":300}`, readError: true, wantCache: 300},
		{name: "explicit zero usage is not missing usage", startUsage: `{"input_tokens":0,"output_tokens":0}`, readError: true},
		{name: "terminal usage survives missing message stop", startUsage: `{"input_tokens":100,"output_tokens":1,"cache_read_input_tokens":300}`, tail: "data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":20}}\n\n", readError: true, wantInput: 100, wantOutput: 20, wantCache: 300},
		{name: "terminal zero output remains authoritative", startUsage: `{"input_tokens":0,"output_tokens":1,"cache_read_input_tokens":300}`, tail: "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":0}}\n\n", readError: true, wantCache: 300},
		{name: "partial text estimates only missing output", startUsage: `{"input_tokens":0,"output_tokens":1,"cache_read_input_tokens":300}`, tail: "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n", readError: true, wantOutput: 2, wantCache: 300, wantLocal: true},
		{name: "unmetered interrupted text is not billable", tail: "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n", readError: true, wantError: true},
		{name: "unmetered text without terminal event is not billable at EOF", tail: "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n", wantError: true},
		{name: "completed text permits missing usage fallback", tail: "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\n\n", wantInput: 1_000_000, wantOutput: 2, wantLocal: true},
		{name: "ping only interrupted stream is not billable", tail: "data: {\"type\":\"ping\"}\n\n", readError: true, wantError: true},
		{name: "unrecognized response is not billable", tail: "data: {\"unexpected\":true}\n\n", wantError: true},
		{name: "truncated JSON remains an error", tail: "data: {\"type\":\"message_start\"\n", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.tail
			if tc.startUsage != "" {
				body = `data: {"type":"message_start","message":{"id":"msg_partial","model":"claude-test","usage":` + tc.startUsage + "}}\n\n" + body
			}
			var reader io.Reader = strings.NewReader(body)
			if tc.readError {
				reader = io.MultiReader(reader, claudeStreamReadError{})
			}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-test"},
				RelayFormat: types.RelayFormatClaude,
				IsStream:    true,
				DisablePing: true,
			}
			info.SetEstimatePromptTokens(1_000_000)
			usage, apiErr := ClaudeStreamHandler(c, &http.Response{Body: io.NopCloser(reader)}, info)
			if tc.wantError {
				require.NotNil(t, apiErr)
				assert.Nil(t, usage)
				assert.False(t, service.ShouldRetryRelayError(c, apiErr, 2), "do not start another generation after an interrupted stream")
				return
			}
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			if tc.readError {
				assert.Equal(t, relaycommon.StreamEndReasonScannerErr, info.StreamStatus.EndReason)
			}
			assert.NotEqual(t, string(relaycommon.ResponseOutcomeCompleted), info.StreamStatus.ResponseOutcome())
			assert.Equal(t, tc.wantInput, usage.PromptTokens)
			assert.Equal(t, tc.wantOutput, usage.CompletionTokens)
			assert.Equal(t, tc.wantCache, usage.PromptTokensDetails.CachedTokens)
			assert.Equal(t, tc.wantLocal, common.GetContextKeyBool(c, constant.ContextKeyLocalCountTokens))
			if tc.wantInput+tc.wantOutput+tc.wantCache > 0 {
				require.NotNil(t, usage.BillingUsage)
				billable, ok := usage.BillingUsage.CanonicalUsage()
				require.True(t, ok)
				assert.Equal(t, tc.wantInput, billable.PromptTokens)
				assert.Equal(t, tc.wantOutput, billable.CompletionTokens)
				assert.Equal(t, tc.wantCache, billable.PromptTokensDetails.CachedTokens)
			}
		})
	}
}

func TestClaudeResponsesStreamHandlerRetainsCacheOnlyUsage(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	for _, tc := range []struct {
		name      string
		tail      string
		readError bool
		failed    bool
	}{
		{name: "interrupted EOF", failed: true},
		{name: "transport failure", readError: true, failed: true},
		{name: "upstream error", tail: "data: {\"type\":\"error\",\"error\":{\"type\":\"api_error\",\"message\":\"upstream failed\"}}\n\n", failed: true},
		{name: "malformed event", tail: "data: {\"type\":\n\n", failed: true},
		{name: "completed", tail: "data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":0}}\n\ndata: {\"type\":\"message_stop\"}\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_cache\",\"model\":\"claude-test\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"usage\":{\"input_tokens\":0,\"output_tokens\":0,\"cache_read_input_tokens\":300}}}\n\n" + tc.tail
			var reader io.Reader = strings.NewReader(body)
			if tc.readError {
				reader = io.MultiReader(reader, claudeStreamReadError{})
			}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-test"},
				RelayFormat: types.RelayFormatOpenAIResponses,
				IsStream:    true,
				DisablePing: true,
			}
			info.SetEstimatePromptTokens(1_000_000)
			usage, apiErr := ClaudeResponsesStreamHandler(c, &http.Response{Body: io.NopCloser(reader)}, info)
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			assert.Zero(t, usage.PromptTokens)
			assert.Zero(t, usage.CompletionTokens)
			assert.Equal(t, 300, usage.PromptTokensDetails.CachedTokens)
			require.NotNil(t, usage.BillingUsage)
			billable, ok := usage.BillingUsage.CanonicalUsage()
			require.True(t, ok)
			assert.Equal(t, 300, billable.PromptTokensDetails.CachedTokens)
			if tc.failed {
				assert.Equal(t, string(relaycommon.ResponseOutcomeFailed), info.StreamStatus.ResponseOutcome())
				assert.Contains(t, w.Body.String(), "event: response.failed")
				assert.NotContains(t, w.Body.String(), "event: response.completed")
			} else {
				assert.Equal(t, string(relaycommon.ResponseOutcomeCompleted), info.StreamStatus.ResponseOutcome())
				assert.Contains(t, w.Body.String(), "event: response.completed")
				assert.NotContains(t, w.Body.String(), "event: response.failed")
			}
		})
	}
}

func TestFormatClaudeResponseInfo_MessageStart(t *testing.T) {
	claudeInfo := &ClaudeResponseInfo{
		Usage: &dto.Usage{},
	}
	claudeResponse := &dto.ClaudeResponse{
		Type: "message_start",
		Message: &dto.ClaudeMediaMessage{
			Id:    "msg_123",
			Model: "claude-3-5-sonnet",
			Usage: &dto.ClaudeUsage{
				InputTokens:              100,
				OutputTokens:             1,
				CacheCreationInputTokens: 50,
				CacheReadInputTokens:     30,
			},
		},
	}

	ok := FormatClaudeResponseInfo(claudeResponse, nil, claudeInfo)
	if !ok {
		t.Fatal("expected true")
	}
	if claudeInfo.Usage.PromptTokens != 100 {
		t.Errorf("PromptTokens = %d, want 100", claudeInfo.Usage.PromptTokens)
	}
	if claudeInfo.Usage.PromptTokensDetails.CachedTokens != 30 {
		t.Errorf("CachedTokens = %d, want 30", claudeInfo.Usage.PromptTokensDetails.CachedTokens)
	}
	if claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens != 50 {
		t.Errorf("CachedCreationTokens = %d, want 50", claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens)
	}
	if claudeInfo.ResponseId != "msg_123" {
		t.Errorf("ResponseId = %s, want msg_123", claudeInfo.ResponseId)
	}
	if claudeInfo.Model != "claude-3-5-sonnet" {
		t.Errorf("Model = %s, want claude-3-5-sonnet", claudeInfo.Model)
	}
}

func TestFormatClaudeResponseInfo_MessageDelta_FullUsage(t *testing.T) {
	// message_start 先积累 usage
	claudeInfo := &ClaudeResponseInfo{
		Usage: &dto.Usage{
			PromptTokens: 100,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens:         30,
				CachedCreationTokens: 50,
			},
			CompletionTokens: 1,
		},
	}

	// message_delta 带完整 usage（原生 Anthropic 场景）
	claudeResponse := &dto.ClaudeResponse{
		Type: "message_delta",
		Usage: &dto.ClaudeUsage{
			InputTokens:              100,
			OutputTokens:             200,
			CacheCreationInputTokens: 50,
			CacheReadInputTokens:     30,
		},
	}

	ok := FormatClaudeResponseInfo(claudeResponse, nil, claudeInfo)
	if !ok {
		t.Fatal("expected true")
	}
	if claudeInfo.Usage.PromptTokens != 100 {
		t.Errorf("PromptTokens = %d, want 100", claudeInfo.Usage.PromptTokens)
	}
	if claudeInfo.Usage.CompletionTokens != 200 {
		t.Errorf("CompletionTokens = %d, want 200", claudeInfo.Usage.CompletionTokens)
	}
	if claudeInfo.Usage.TotalTokens != 300 {
		t.Errorf("TotalTokens = %d, want 300", claudeInfo.Usage.TotalTokens)
	}
	if !claudeInfo.Done {
		t.Error("expected Done = true")
	}
}

func TestFormatClaudeResponseInfo_MessageDelta_OnlyOutputTokens(t *testing.T) {
	// 模拟 Bedrock: message_start 已积累 usage
	claudeInfo := &ClaudeResponseInfo{
		Usage: &dto.Usage{
			PromptTokens: 100,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens:         30,
				CachedCreationTokens: 50,
			},
			CompletionTokens:            1,
			ClaudeCacheCreation5mTokens: 10,
			ClaudeCacheCreation1hTokens: 20,
		},
	}

	// Bedrock 的 message_delta 只有 output_tokens，缺少 input_tokens 和 cache 字段
	claudeResponse := &dto.ClaudeResponse{
		Type: "message_delta",
		Usage: &dto.ClaudeUsage{
			OutputTokens: 200,
			// InputTokens, CacheCreationInputTokens, CacheReadInputTokens 都是 0
		},
	}

	ok := FormatClaudeResponseInfo(claudeResponse, nil, claudeInfo)
	if !ok {
		t.Fatal("expected true")
	}
	// PromptTokens 应保持 message_start 的值（因为 message_delta 的 InputTokens=0，不更新）
	if claudeInfo.Usage.PromptTokens != 100 {
		t.Errorf("PromptTokens = %d, want 100", claudeInfo.Usage.PromptTokens)
	}
	if claudeInfo.Usage.CompletionTokens != 200 {
		t.Errorf("CompletionTokens = %d, want 200", claudeInfo.Usage.CompletionTokens)
	}
	if claudeInfo.Usage.TotalTokens != 300 {
		t.Errorf("TotalTokens = %d, want 300", claudeInfo.Usage.TotalTokens)
	}
	// cache 字段应保持 message_start 的值
	if claudeInfo.Usage.PromptTokensDetails.CachedTokens != 30 {
		t.Errorf("CachedTokens = %d, want 30", claudeInfo.Usage.PromptTokensDetails.CachedTokens)
	}
	if claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens != 50 {
		t.Errorf("CachedCreationTokens = %d, want 50", claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens)
	}
	if claudeInfo.Usage.ClaudeCacheCreation5mTokens != 10 {
		t.Errorf("ClaudeCacheCreation5mTokens = %d, want 10", claudeInfo.Usage.ClaudeCacheCreation5mTokens)
	}
	if claudeInfo.Usage.ClaudeCacheCreation1hTokens != 20 {
		t.Errorf("ClaudeCacheCreation1hTokens = %d, want 20", claudeInfo.Usage.ClaudeCacheCreation1hTokens)
	}
	if !claudeInfo.Done {
		t.Error("expected Done = true")
	}
}

func TestFormatClaudeResponseInfo_NilClaudeInfo(t *testing.T) {
	claudeResponse := &dto.ClaudeResponse{Type: "message_start"}
	ok := FormatClaudeResponseInfo(claudeResponse, nil, nil)
	if ok {
		t.Error("expected false for nil claudeInfo")
	}
}

func TestFormatClaudeResponseInfo_ContentBlockDelta(t *testing.T) {
	text := "hello"
	claudeInfo := &ClaudeResponseInfo{
		Usage:        &dto.Usage{},
		ResponseText: strings.Builder{},
	}
	claudeResponse := &dto.ClaudeResponse{
		Type: "content_block_delta",
		Delta: &dto.ClaudeMediaMessage{
			Text: &text,
		},
	}

	ok := FormatClaudeResponseInfo(claudeResponse, nil, claudeInfo)
	if !ok {
		t.Fatal("expected true")
	}
	if claudeInfo.ResponseText.String() != "hello" {
		t.Errorf("ResponseText = %q, want %q", claudeInfo.ResponseText.String(), "hello")
	}
}

// A stream cut off during an agent tool call must still estimate the call's
// output tokens when the final usage never arrives.
func TestClaudeStreamFallbackCountsInterruptedToolUse(t *testing.T) {
	claudeInfo := &ClaudeResponseInfo{Usage: &dto.Usage{}}
	events := []*dto.ClaudeResponse{
		{Type: "message_start", Message: &dto.ClaudeMediaMessage{Usage: &dto.ClaudeUsage{InputTokens: 100}}},
		{Type: "content_block_start", ContentBlock: &dto.ClaudeMediaMessage{Type: "tool_use", Id: "toolu_1", Name: "read_file"}},
		{Type: "content_block_delta", Delta: &dto.ClaudeMediaMessage{Type: "input_json_delta", PartialJson: commonPointer(`{"path":`)}},
		{Type: "content_block_delta", Delta: &dto.ClaudeMediaMessage{Type: "input_json_delta", PartialJson: commonPointer(`"main.go"}`)}},
	}
	for _, event := range events {
		require.True(t, FormatClaudeResponseInfo(event, nil, claudeInfo))
	}
	require.Equal(t, `read_file{"path":"main.go"}`, claudeInfo.ResponseText.String())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-sonnet-5"},
	}
	info.SetEstimatePromptTokens(100)
	require.Nil(t, HandleStreamFinalResponse(c, info, claudeInfo))

	// Text estimate plus the per-tool_use framing the upstream bills.
	assert.Equal(t, tokenkit.Estimate("claude-sonnet-5", `read_file{"path":"main.go"}`)+40, claudeInfo.Usage.CompletionTokens)
	assert.Equal(t, 100, claudeInfo.Usage.PromptTokens)
}

func TestBuildOpenAIStyleUsageFromClaudeUsage(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens:     100,
		CompletionTokens: 20,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         30,
			CachedCreationTokens: 50,
		},
		ClaudeCacheCreation5mTokens: 10,
		ClaudeCacheCreation1hTokens: 20,
		UsageSemantic:               "anthropic",
	}

	openAIUsage := buildOpenAIStyleUsageFromClaudeUsage(usage)

	if openAIUsage.PromptTokens != 180 {
		t.Fatalf("PromptTokens = %d, want 180", openAIUsage.PromptTokens)
	}
	if openAIUsage.InputTokens != 180 {
		t.Fatalf("InputTokens = %d, want 180", openAIUsage.InputTokens)
	}
	if openAIUsage.TotalTokens != 200 {
		t.Fatalf("TotalTokens = %d, want 200", openAIUsage.TotalTokens)
	}
	if openAIUsage.UsageSemantic != "openai" {
		t.Fatalf("UsageSemantic = %s, want openai", openAIUsage.UsageSemantic)
	}
	if openAIUsage.UsageSource != "anthropic" {
		t.Fatalf("UsageSource = %s, want anthropic", openAIUsage.UsageSource)
	}
}

func TestBuildOpenAIStyleUsageFromClaudeUsagePreservesCacheCreationRemainder(t *testing.T) {
	tests := []struct {
		name                    string
		cachedCreationTokens    int
		cacheCreationTokens5m   int
		cacheCreationTokens1h   int
		expectedTotalInputToken int
	}{
		{
			name:                    "prefers aggregate when it includes remainder",
			cachedCreationTokens:    50,
			cacheCreationTokens5m:   10,
			cacheCreationTokens1h:   20,
			expectedTotalInputToken: 180,
		},
		{
			name:                    "falls back to split tokens when aggregate missing",
			cachedCreationTokens:    0,
			cacheCreationTokens5m:   10,
			cacheCreationTokens1h:   20,
			expectedTotalInputToken: 160,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := &dto.Usage{
				PromptTokens:     100,
				CompletionTokens: 20,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens:         30,
					CachedCreationTokens: tt.cachedCreationTokens,
				},
				ClaudeCacheCreation5mTokens: tt.cacheCreationTokens5m,
				ClaudeCacheCreation1hTokens: tt.cacheCreationTokens1h,
				UsageSemantic:               "anthropic",
			}

			openAIUsage := buildOpenAIStyleUsageFromClaudeUsage(usage)

			if openAIUsage.PromptTokens != tt.expectedTotalInputToken {
				t.Fatalf("PromptTokens = %d, want %d", openAIUsage.PromptTokens, tt.expectedTotalInputToken)
			}
			if openAIUsage.InputTokens != tt.expectedTotalInputToken {
				t.Fatalf("InputTokens = %d, want %d", openAIUsage.InputTokens, tt.expectedTotalInputToken)
			}
		})
	}
}

func TestBuildOpenAIStyleUsageFromClaudeUsageDefaultsAggregateCacheCreationTo5m(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens:     100,
		CompletionTokens: 20,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         30,
			CachedCreationTokens: 50,
		},
		UsageSemantic: "anthropic",
	}

	openAIUsage := buildOpenAIStyleUsageFromClaudeUsage(usage)

	require.Equal(t, 50, openAIUsage.ClaudeCacheCreation5mTokens)
	require.Equal(t, 0, openAIUsage.ClaudeCacheCreation1hTokens)
}

func applyOpenAIChatReasoningThroughHandlerOrder(t *testing.T, original dto.GeneralOpenAIRequest) (*dto.GeneralOpenAIRequest, *relaycommon.RelayInfo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	info := &relaycommon.RelayInfo{
		OriginModelName: original.Model,
		Request:         &original,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: original.Model,
		},
	}
	outbound, err := common.DeepCopy(&original)
	require.NoError(t, err)
	require.NoError(t, helper.ModelMappedHelper(c, info, outbound))
	err = helper.ApplyReasoningModelSuffix(nil, info, outbound)
	require.NoError(t, err)
	return outbound, info
}

func TestOpenAIChatRequestToClaudeMessages_ClaudeOpus48HighUsesAdaptiveThinking(t *testing.T) {
	original := dto.GeneralOpenAIRequest{
		Model:       "claude-opus-4-8-high",
		Temperature: commonPointer(0.7),
		TopP:        commonPointer(0.9),
		TopK:        commonPointer(40),
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "hello",
			},
		},
	}

	outbound, info := applyOpenAIChatReasoningThroughHandlerOrder(t, original)
	claudeRequest, err := relayconvert.OpenAIChatRequestToClaudeMessages(nil, info, *outbound)
	require.NoError(t, err)
	require.Equal(t, "claude-opus-4-8", claudeRequest.Model)
	require.NotNil(t, claudeRequest.Thinking)
	require.Equal(t, "adaptive", claudeRequest.Thinking.Type)
	require.Equal(t, "summarized", claudeRequest.Thinking.Display)
	require.JSONEq(t, `{"effort":"high"}`, string(claudeRequest.OutputConfig))
	require.Nil(t, claudeRequest.Temperature)
	require.Nil(t, claudeRequest.TopP)
	require.Nil(t, claudeRequest.TopK)
}

func TestOpenAIChatRequestToClaudeMessages_ClaudeOpus48ThinkingUsesAdaptiveHighEffort(t *testing.T) {
	original := dto.GeneralOpenAIRequest{
		Model:       "claude-opus-4-8-thinking",
		Temperature: commonPointer(0.7),
		TopP:        commonPointer(0.9),
		TopK:        commonPointer(40),
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "hello",
			},
		},
	}

	outbound, info := applyOpenAIChatReasoningThroughHandlerOrder(t, original)
	claudeRequest, err := relayconvert.OpenAIChatRequestToClaudeMessages(nil, info, *outbound)
	require.NoError(t, err)
	require.Equal(t, "claude-opus-4-8", claudeRequest.Model)
	require.NotNil(t, claudeRequest.Thinking)
	require.Equal(t, "adaptive", claudeRequest.Thinking.Type)
	require.Equal(t, "summarized", claudeRequest.Thinking.Display)
	require.JSONEq(t, `{"effort":"high"}`, string(claudeRequest.OutputConfig))
	require.Nil(t, claudeRequest.Temperature)
	require.Nil(t, claudeRequest.TopP)
	require.Nil(t, claudeRequest.TopK)
}

func TestClaudeResponsePassesInputTransformationsThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-opus-5-5"},
	}
	transformations := `"input_transformations":[{"type":"thinking_dropped","path":"messages.1.content.0","reason":"prefix_binding_mismatch"}]`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	body := `{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5","content":[{"type":"text","text":"hi"}],` +
		`"stop_reason":"end_turn",` + transformations + `,"usage":{"input_tokens":5,"output_tokens":1}}`
	require.Nil(t, HandleClaudeResponseData(c, info, &ClaudeResponseInfo{Usage: &dto.Usage{}}, nil, []byte(body)))
	assert.Equal(t, body, w.Body.String())

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	event := `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5","content":[],` +
		transformations + `,"usage":{"input_tokens":5,"output_tokens":1}}}`
	require.Nil(t, HandleStreamResponseData(c, info, &ClaudeResponseInfo{Usage: &dto.Usage{}}, event))
	assert.Contains(t, w.Body.String(), "data: "+event+"\n")
}
