package advancedcustom

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAdaptorUsesExactRouteAndQueryAuth(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/messages",
				UpstreamPath: "https://upstream.example/v1/chat/completions?existing=1",
				Converter:    relayconvert.ConverterClaudeMessagesToOpenAIChat,
				Auth: &dto.AdvancedCustomRouteAuth{
					Type:  dto.AdvancedCustomAuthTypeQuery,
					Name:  "api_key",
					Value: "{api_key}",
				},
			},
		},
	})
	info.RequestURLPath = "/v1/messages?client=1"

	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)

	parsedURL, err := url.Parse(requestURL)
	require.NoError(t, err)
	assert.Equal(t, "https", parsedURL.Scheme)
	assert.Equal(t, "upstream.example", parsedURL.Host)
	assert.Equal(t, "/v1/chat/completions", parsedURL.Path)
	assert.Equal(t, "1", parsedURL.Query().Get("existing"))
	assert.Equal(t, "sk-test", parsedURL.Query().Get("api_key"))
}

func TestAdaptorJoinsUpstreamPathWithChannelBaseURL(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "/proxy/v1/chat/completions?existing=1",
				Converter:    relayconvert.ConverterNone,
				Auth: &dto.AdvancedCustomRouteAuth{
					Type:  dto.AdvancedCustomAuthTypeQuery,
					Name:  "api_key",
					Value: "{api_key}",
				},
			},
		},
	})
	info.ChannelBaseUrl = "https://gateway.example/base"

	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)

	parsedURL, err := url.Parse(requestURL)
	require.NoError(t, err)
	assert.Equal(t, "https", parsedURL.Scheme)
	assert.Equal(t, "gateway.example", parsedURL.Host)
	assert.Equal(t, "/base/proxy/v1/chat/completions", parsedURL.Path)
	assert.Equal(t, "1", parsedURL.Query().Get("existing"))
	assert.Equal(t, "sk-test", parsedURL.Query().Get("api_key"))
}

func TestAdaptorReturnsErrorWhenUpstreamPathNeedsMissingBaseURL(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "/v1/chat/completions",
				Converter:    relayconvert.ConverterNone,
			},
		},
	})
	info.ChannelBaseUrl = ""

	_, err := adaptor.GetRequestURL(info)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base URL is required")
}

func TestAdaptorSetupRequestHeaderUsesDefaultBearerAuth(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "https://upstream.example/v1/chat/completions",
				Converter:    relayconvert.ConverterNone,
			},
		},
	})
	c := advancedCustomGinContext("/v1/chat/completions")
	header := http.Header{}

	require.NoError(t, adaptor.SetupRequestHeader(c, &header, info))
	assert.Equal(t, "Bearer sk-test", header.Get("Authorization"))
}

func TestAdaptorSetupRequestHeaderUsesConfiguredHeaderAuth(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "https://upstream.example/v1/chat/completions",
				Converter:    relayconvert.ConverterNone,
				Auth: &dto.AdvancedCustomRouteAuth{
					Type:  dto.AdvancedCustomAuthTypeHeader,
					Name:  "x-api-key",
					Value: "{api_key}",
				},
			},
		},
	})
	c := advancedCustomGinContext("/v1/chat/completions")
	header := http.Header{}

	require.NoError(t, adaptor.SetupRequestHeader(c, &header, info))
	assert.Empty(t, header.Get("Authorization"))
	assert.Equal(t, "sk-test", header.Get("x-api-key"))
}

func TestAdaptorSetupRequestHeaderAddsClaudeDefaultHeaders(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/messages",
				UpstreamPath: "https://api.anthropic.com/v1/messages",
				Converter:    relayconvert.ConverterNone,
				Auth: &dto.AdvancedCustomRouteAuth{
					Type:  dto.AdvancedCustomAuthTypeHeader,
					Name:  "x-api-key",
					Value: "{api_key}",
				},
			},
		},
	})
	info.RelayFormat = types.RelayFormatClaude
	c := advancedCustomGinContext("/v1/messages")
	header := http.Header{}

	require.NoError(t, adaptor.SetupRequestHeader(c, &header, info))
	assert.Equal(t, "sk-test", header.Get("x-api-key"))
	assert.Equal(t, "2023-06-01", header.Get("anthropic-version"))
}

func TestAdaptorReturnsErrorWhenNoRouteMatchesPath(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/messages",
				UpstreamPath: "https://upstream.example/v1/chat/completions",
				Converter:    relayconvert.ConverterClaudeMessagesToOpenAIChat,
			},
		},
	})
	info.RequestURLPath = "/v1/chat/completions"

	_, err := adaptor.GetRequestURL(info)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support request path")
}

func TestAdaptorReplacesModelPlaceholderInRouteURL(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent",
				Converter:    relayconvert.ConverterOpenAIChatToGeminiContent,
				Auth: &dto.AdvancedCustomRouteAuth{
					Type:  dto.AdvancedCustomAuthTypeQuery,
					Name:  "key",
					Value: "{api_key}",
				},
			},
		},
	})
	info.UpstreamModelName = "gemini-2.5-flash"

	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)

	parsedURL, err := url.Parse(requestURL)
	require.NoError(t, err)
	assert.Equal(t, "/v1beta/models/gemini-2.5-flash:generateContent", parsedURL.Path)
	assert.Equal(t, "sk-test", parsedURL.Query().Get("key"))
	assert.Empty(t, parsedURL.Query().Get("alt"))
}

func TestAdaptorSwitchesGeminiGenerateContentURLForStream(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent?existing=1",
				Converter:    relayconvert.ConverterOpenAIChatToGeminiContent,
				Auth: &dto.AdvancedCustomRouteAuth{
					Type:  dto.AdvancedCustomAuthTypeQuery,
					Name:  "key",
					Value: "{api_key}",
				},
			},
		},
	})
	info.UpstreamModelName = "gemini-2.5-pro"
	info.IsStream = true

	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)

	parsedURL, err := url.Parse(requestURL)
	require.NoError(t, err)
	assert.Equal(t, "/v1beta/models/gemini-2.5-pro:streamGenerateContent", parsedURL.Path)
	assert.Equal(t, "sse", parsedURL.Query().Get("alt"))
	assert.Equal(t, "1", parsedURL.Query().Get("existing"))
	assert.Equal(t, "sk-test", parsedURL.Query().Get("key"))
}

func TestAdaptorMatchesGeminiIncomingPathTemplate(t *testing.T) {
	tests := []struct {
		name            string
		requestURLPath  string
		wantRequestPath string
	}{
		{
			name:            "generate content",
			requestURLPath:  "/v1beta/models/gemini-2.5-flash:generateContent",
			wantRequestPath: "/v1/chat/completions",
		},
		{
			name:            "stream generate content",
			requestURLPath:  "/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse",
			wantRequestPath: "/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adaptor := &Adaptor{}
			info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
				Routes: []dto.AdvancedCustomRoute{
					{
						IncomingPath: "/v1beta/models/{model}:generateContent",
						UpstreamPath: "https://upstream.example/v1/chat/completions",
						Converter:    relayconvert.ConverterGeminiContentToOpenAIChat,
					},
				},
			})
			info.RequestURLPath = tt.requestURLPath

			requestURL, err := adaptor.GetRequestURL(info)
			require.NoError(t, err)

			parsedURL, err := url.Parse(requestURL)
			require.NoError(t, err)
			assert.Equal(t, tt.wantRequestPath, parsedURL.Path)
		})
	}
}

func TestAdaptorBuildModelListRequestUsesConfiguredRouteAuth(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/models",
				UpstreamPath: "/provider/models",
				Converter:    relayconvert.ConverterNone,
				Auth: &dto.AdvancedCustomRouteAuth{
					Type:  dto.AdvancedCustomAuthTypeHeader,
					Name:  "x-api-key",
					Value: "token {api_key}",
				},
			},
		},
	})
	info.RequestURLPath = "/v1/models"

	requestURL, header, err := adaptor.BuildModelListRequest(info)
	require.NoError(t, err)

	parsedURL, err := url.Parse(requestURL)
	require.NoError(t, err)
	assert.Equal(t, "fallback.example", parsedURL.Host)
	assert.Equal(t, "/provider/models", parsedURL.Path)
	assert.Equal(t, "token sk-test", header.Get("x-api-key"))
	assert.Empty(t, header.Get("Authorization"))
}

func TestAdaptorBuildModelListRequestUsesConfiguredQueryAuth(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/models",
				UpstreamPath: "https://upstream.example/v1/models?existing=1",
				Converter:    relayconvert.ConverterNone,
				Auth: &dto.AdvancedCustomRouteAuth{
					Type:  dto.AdvancedCustomAuthTypeQuery,
					Name:  "key",
					Value: "{api_key}",
				},
			},
		},
	})
	info.RequestURLPath = "/v1/models"

	requestURL, header, err := adaptor.BuildModelListRequest(info)
	require.NoError(t, err)

	parsedURL, err := url.Parse(requestURL)
	require.NoError(t, err)
	assert.Equal(t, "upstream.example", parsedURL.Host)
	assert.Equal(t, "/v1/models", parsedURL.Path)
	assert.Equal(t, "1", parsedURL.Query().Get("existing"))
	assert.Equal(t, "sk-test", parsedURL.Query().Get("key"))
	assert.Empty(t, header.Get("Authorization"))
}

func TestAdaptorBuildModelListRequestDefaultAndNoAuth(t *testing.T) {
	tests := []struct {
		name              string
		auth              *dto.AdvancedCustomRouteAuth
		wantAuthorization string
	}{
		{
			name:              "default bearer",
			wantAuthorization: "Bearer sk-test",
		},
		{
			name: "no authentication",
			auth: &dto.AdvancedCustomRouteAuth{
				Type: dto.AdvancedCustomAuthTypeNone,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
				Routes: []dto.AdvancedCustomRoute{
					{
						IncomingPath: dto.AdvancedCustomModelListPath,
						UpstreamPath: "/provider/models",
						Auth:         tt.auth,
					},
				},
			})
			info.RequestURLPath = "/unrelated/path"

			requestURL, header, err := (&Adaptor{}).BuildModelListRequest(info)
			require.NoError(t, err)
			assert.Equal(t, "https://fallback.example/provider/models", requestURL)
			assert.Equal(t, tt.wantAuthorization, header.Get("Authorization"))
		})
	}
}

func TestAdaptorBuildModelListRequestDoesNotReuseRelayRoute(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "/chat",
			},
			{
				IncomingPath: dto.AdvancedCustomModelListPath,
				UpstreamPath: "/provider/models",
			},
		},
	})

	chatURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://fallback.example/chat", chatURL)

	modelURL, header, err := adaptor.BuildModelListRequest(info)
	require.NoError(t, err)
	assert.Equal(t, "https://fallback.example/provider/models", modelURL)
	assert.Equal(t, "Bearer sk-test", header.Get("Authorization"))
}

func TestAdaptorBuildModelListRequestRequiresConfiguredRoute(t *testing.T) {
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "/v1/chat/completions",
			},
		},
	})

	_, _, err := (&Adaptor{}).BuildModelListRequest(info)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not configure a /v1/models route")
}

func TestAdaptorBuildBalanceRequestUsesConfiguredRoute(t *testing.T) {
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: dto.AdvancedCustomModelListPath,
				UpstreamPath: "/provider/models",
			},
			{
				IncomingPath: dto.AdvancedCustomBalancePath,
				UpstreamPath: "/provider/balance?existing=1",
				Auth: &dto.AdvancedCustomRouteAuth{
					Type:  dto.AdvancedCustomAuthTypeQuery,
					Name:  "token",
					Value: "prefix-{api_key}",
				},
			},
		},
	})

	requestURL, header, err := (&Adaptor{}).BuildBalanceRequest(info)
	require.NoError(t, err)

	parsedURL, err := url.Parse(requestURL)
	require.NoError(t, err)
	assert.Equal(t, "/provider/balance", parsedURL.Path)
	assert.Equal(t, "1", parsedURL.Query().Get("existing"))
	assert.Equal(t, "prefix-sk-test", parsedURL.Query().Get("token"))
	assert.Empty(t, header.Get("Authorization"))
}

func TestAdaptorBuildBalanceRequestRequiresConfiguredRoute(t *testing.T) {
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{{
			IncomingPath: dto.AdvancedCustomModelListPath,
			UpstreamPath: "/provider/models",
		}},
	})

	_, _, err := (&Adaptor{}).BuildBalanceRequest(info)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not configure a /v1/dashboard/billing/credit_grants route")
}

func TestAdaptorConvertsResponsesRequestToOpenAIChatUpstream(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    relayconvert.ConverterOpenAIResponsesToOpenAIChat,
			},
		},
	})
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model:        "gpt-test",
		Instructions: mustAdvancedCustomRawMessage(t, "system rules"),
		Input:        mustAdvancedCustomRawMessage(t, "hello"),
	})
	require.NoError(t, err)

	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	assert.Equal(t, "gpt-test", chatReq.Model)
	require.Len(t, chatReq.Messages, 2)
	assert.Equal(t, "system", chatReq.Messages[0].Role)
	assert.Equal(t, "system rules", chatReq.Messages[0].StringContent())
	assert.Equal(t, "user", chatReq.Messages[1].Role)
	assert.Equal(t, "hello", chatReq.Messages[1].StringContent())

	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	parsedURL, err := url.Parse(requestURL)
	require.NoError(t, err)
	assert.Equal(t, "/v1/chat/completions", parsedURL.Path)
}

func TestAdaptorSelectsDuplicateResponsesRoutesByModel(t *testing.T) {
	config := &dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    relayconvert.ConverterOpenAIResponsesToOpenAIChat,
				Models:       []string{"gpt-test"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    relayconvert.ConverterOpenAIResponsesToGemini,
				Models:       []string{"gemini-test"},
			},
		},
	}

	chatAdaptor := &Adaptor{}
	chatInfo := advancedCustomRelayInfo(config)
	chatInfo.RelayFormat = types.RelayFormatOpenAIResponses
	chatInfo.RelayMode = relayconstant.RelayModeResponses
	chatInfo.RequestURLPath = "/v1/responses"
	chatInfo.OriginModelName = "gpt-test"
	chatInfo.UpstreamModelName = "gpt-test"
	chatConverted, err := chatAdaptor.ConvertOpenAIResponsesRequest(advancedCustomGinContext("/v1/responses"), chatInfo, dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustAdvancedCustomRawMessage(t, "hello"),
	})
	require.NoError(t, err)
	_, ok := chatConverted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)

	geminiAdaptor := &Adaptor{}
	geminiInfo := advancedCustomRelayInfo(config)
	geminiInfo.RelayFormat = types.RelayFormatOpenAIResponses
	geminiInfo.RelayMode = relayconstant.RelayModeResponses
	geminiInfo.RequestURLPath = "/v1/responses"
	geminiInfo.OriginModelName = "gemini-test"
	geminiInfo.UpstreamModelName = "gemini-test"
	geminiInfo.IsStream = true
	geminiConverted, err := geminiAdaptor.ConvertOpenAIResponsesRequest(advancedCustomGinContext("/v1/responses"), geminiInfo, dto.OpenAIResponsesRequest{
		Model: "gemini-test",
		Input: mustAdvancedCustomRawMessage(t, "hello"),
	})
	require.NoError(t, err)
	_, ok = geminiConverted.(*dto.GeminiChatRequest)
	require.True(t, ok)

	requestURL, err := geminiAdaptor.GetRequestURL(geminiInfo)
	require.NoError(t, err)
	parsedURL, err := url.Parse(requestURL)
	require.NoError(t, err)
	assert.Equal(t, "/v1beta/models/gemini-test:streamGenerateContent", parsedURL.Path)
	assert.Equal(t, "sse", parsedURL.Query().Get("alt"))
}

func TestAdaptorResponsesToGeminiUsesResponsesBridge(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    relayconvert.ConverterOpenAIResponsesToGemini,
				Models:       []string{"gemini-test"},
			},
		},
	})
	info.RelayFormat = types.RelayFormatOpenAIResponses
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"
	info.OriginModelName = "gemini-test"
	info.UpstreamModelName = "gemini-test"
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	payload := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{Text: "hello"},
					},
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:     2,
			CandidatesTokenCount: 3,
			TotalTokenCount:      5,
		},
	}
	body, err := common.Marshal(payload)
	require.NoError(t, err)

	usage, newAPIError := adaptor.DoResponse(c, &http.Response{
		Body: io.NopCloser(bytes.NewReader(body)),
	}, info)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)

	got := recorder.Body.String()
	assert.Contains(t, got, `"object":"response"`)
	assert.Contains(t, got, `"type":"output_text"`)
	assert.Contains(t, got, `"text":"hello"`)
	assert.NotContains(t, got, `"candidates"`)
}

func TestAdaptorResponsesToGeminiAddsThoughtSignatureForFunctionCallHistory(t *testing.T) {
	geminiSettings := model_setting.GetGeminiSettings()
	originalThoughtSignatureEnabled := geminiSettings.FunctionCallThoughtSignatureEnabled
	geminiSettings.FunctionCallThoughtSignatureEnabled = true
	t.Cleanup(func() {
		geminiSettings.FunctionCallThoughtSignatureEnabled = originalThoughtSignatureEnabled
	})

	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    relayconvert.ConverterOpenAIResponsesToGemini,
				Models:       []string{"gemini-test"},
			},
		},
	})
	info.RelayFormat = types.RelayFormatOpenAIResponses
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"
	info.OriginModelName = "gemini-test"
	info.UpstreamModelName = "gemini-test"

	converted, err := adaptor.ConvertOpenAIResponsesRequest(advancedCustomGinContext("/v1/responses"), info, dto.OpenAIResponsesRequest{
		Model: "gemini-test",
		Input: mustAdvancedCustomRawMessage(t, []map[string]any{
			{
				"role":    "user",
				"content": "hi",
			},
			{
				"type":      "function_call",
				"call_id":   "call_1",
				"name":      "glob",
				"arguments": map[string]any{"query": "*"},
			},
			{
				"type":    "function_call_output",
				"call_id": "call_1",
				"output":  []map[string]any{{"path": "report.md"}},
			},
		}),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
			{"type": "function", "name": "glob", "parameters": map[string]any{"type": "object"}},
		}),
	})
	require.NoError(t, err)

	geminiReq, ok := converted.(*dto.GeminiChatRequest)
	require.True(t, ok)
	require.Len(t, geminiReq.Contents, 3)
	require.Len(t, geminiReq.Contents[1].Parts, 1)
	require.NotNil(t, geminiReq.Contents[1].Parts[0].FunctionCall)
	assert.NotEmpty(t, geminiReq.Contents[1].Parts[0].ThoughtSignature)
	require.Len(t, geminiReq.Contents[2].Parts, 1)
	require.NotNil(t, geminiReq.Contents[2].Parts[0].FunctionResponse)
	assert.Empty(t, geminiReq.Contents[2].Parts[0].ThoughtSignature)
}

func TestAdaptorConvertsOpenAIChatRequestToResponsesUpstream(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "/v1/responses",
				Converter:    relayconvert.ConverterOpenAIChatToOpenAIResponses,
			},
		},
	})
	c := advancedCustomGinContext("/v1/chat/completions")

	converted, err := adaptor.ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{
		Model: "gpt-test",
		Messages: []dto.Message{
			{Role: "user", Content: "hello"},
		},
	})
	require.NoError(t, err)

	responsesReq, ok := converted.(*dto.OpenAIResponsesRequest)
	require.True(t, ok)
	assert.Equal(t, "gpt-test", responsesReq.Model)
	assert.NotEmpty(t, responsesReq.Input)
}

func TestAdaptorConvertsOpenAIChatRequestToClaudeUpstream(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "/v1/messages",
				Converter:    relayconvert.ConverterOpenAIChatToClaudeMessages,
			},
		},
	})
	c := advancedCustomGinContext("/v1/chat/completions")

	converted, err := adaptor.ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{
		Model: "claude-test",
		Messages: []dto.Message{
			{Role: "user", Content: "hello"},
		},
	})
	require.NoError(t, err)

	claudeReq, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	assert.Equal(t, "claude-test", claudeReq.Model)
	require.Len(t, claudeReq.Messages, 1)
	assert.Equal(t, "user", claudeReq.Messages[0].Role)
}

func TestAdaptorConvertsOpenAIChatRequestToGeminiUpstream(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    relayconvert.ConverterOpenAIChatToGeminiContent,
			},
		},
	})
	info.UpstreamModelName = "gemini-2.5-flash"
	c := advancedCustomGinContext("/v1/chat/completions")

	converted, err := adaptor.ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{
		Model: "gemini-2.5-flash",
		Messages: []dto.Message{
			{Role: "user", Content: "hello"},
		},
	})
	require.NoError(t, err)

	geminiReq, ok := converted.(*dto.GeminiChatRequest)
	require.True(t, ok)
	require.Len(t, geminiReq.Contents, 1)
	assert.Equal(t, "user", geminiReq.Contents[0].Role)
}

func TestAdaptorConvertsClaudeRequestToOpenAIChatUpstream(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/messages",
				UpstreamPath: "/v1/chat/completions",
				Converter:    relayconvert.ConverterClaudeMessagesToOpenAIChat,
			},
		},
	})
	info.RelayFormat = types.RelayFormatClaude
	info.RequestURLPath = "/v1/messages"
	c := advancedCustomGinContext("/v1/messages")

	converted, err := adaptor.ConvertClaudeRequest(c, info, &dto.ClaudeRequest{
		Model: "gpt-test",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "hello"},
		},
	})
	require.NoError(t, err)

	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	assert.Equal(t, "gpt-test", chatReq.Model)
	require.Len(t, chatReq.Messages, 1)
	assert.Equal(t, "user", chatReq.Messages[0].Role)
}

func TestAdaptorConvertsGeminiRequestToOpenAIChatUpstream(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1beta/models/{model}:generateContent",
				UpstreamPath: "/v1/chat/completions",
				Converter:    relayconvert.ConverterGeminiContentToOpenAIChat,
			},
		},
	})
	info.RelayFormat = types.RelayFormatGemini
	info.RequestURLPath = "/v1beta/models/gemini-2.5-flash:generateContent"
	info.UpstreamModelName = "gpt-test"
	c := advancedCustomGinContext("/v1beta/models/gemini-2.5-flash:generateContent")

	converted, err := adaptor.ConvertGeminiRequest(c, info, &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{
				Role: "user",
				Parts: []dto.GeminiPart{
					{Text: "hello"},
				},
			},
		},
	})
	require.NoError(t, err)

	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	assert.Equal(t, "gpt-test", chatReq.Model)
	require.Len(t, chatReq.Messages, 1)
	assert.Equal(t, "user", chatReq.Messages[0].Role)
}

func TestAdaptorRestoresOriginalModelForNativeJSONResponses(t *testing.T) {
	const clientModel = "claude-4.5-sonnet-20250929"
	const upstreamModel = "claude-4.5-sonnet"

	for _, tc := range []struct {
		name, requestURL, routePath, modelPath, body string
		format                                       types.RelayFormat
		mode                                         int
	}{
		{"openai chat", "/v1/chat/completions", "/v1/chat/completions", "model", `{"id":"chat_1","object":"chat.completion","model":"claude-4.5-sonnet","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2},"extension":{"model":"nested-upstream","tool":{"model":"tool-upstream"}}}`, types.RelayFormatOpenAI, relayconstant.RelayModeChatCompletions},
		{"claude messages", "/v1/messages", "/v1/messages", "model", `{"id":"msg_1","type":"message","role":"assistant","model":"claude-4.5-sonnet","content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":1,"output_tokens":1},"extension":{"model":"nested-upstream","tool":{"model":"tool-upstream"}}}`, types.RelayFormatClaude, relayconstant.RelayModeChatCompletions},
		{"gemini generate content", "/v1beta/models/claude-4.5-sonnet-20250929:generateContent", "/v1beta/models/{model}:generateContent", "modelVersion", `{"candidates":[],"modelVersion":"","usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2},"extension":{"model":"nested-upstream","tool":{"model":"tool-upstream"}}}`, types.RelayFormatGemini, relayconstant.RelayModeGemini},
		{"openai responses", "/v1/responses", "/v1/responses", "model", `{"id":"resp_1","object":"response","status":"completed","model":"claude-4.5-sonnet","output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2},"extension":{"model":"nested-upstream","tool":{"model":"tool-upstream"}}}`, types.RelayFormatOpenAIResponses, relayconstant.RelayModeResponses},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := advancedCustomResponseInfo(tc.requestURL, tc.routePath, tc.format, tc.mode, relayconvert.ConverterNone)
			info.OriginModelName, info.UpstreamModelName = clientModel, upstreamModel
			c, recorder := advancedCustomGinRecorder(tc.requestURL)
			common.SetContextKey(c, constant.ContextKeyOriginalModel, clientModel)

			_, responseErr := (&Adaptor{}).DoResponse(c, advancedCustomHTTPResponse(tc.body), info)
			require.Nil(t, responseErr)
			got := recorder.Body.String()
			assert.Equal(t, clientModel, gjson.Get(got, tc.modelPath).String())
			assert.Equal(t, "nested-upstream", gjson.Get(got, "extension.model").String())
			assert.Equal(t, "tool-upstream", gjson.Get(got, "extension.tool.model").String())
			assert.Equal(t, strconv.Itoa(len(recorder.Body.Bytes())), recorder.Header().Get("Content-Length"))
		})
	}
}

func TestAdaptorRestoresOriginalModelForNativeStreams(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })
	const clientModel = "claude-4.5-sonnet-20250929"
	const upstreamModel = "claude-4.5-sonnet"

	for _, tc := range []struct {
		name, requestURL, routePath, modelPath, body string
		format                                       types.RelayFormat
		mode                                         int
		modelFrames, sparseFrames                    []int
	}{
		{"openai chat", "/v1/chat/completions", "/v1/chat/completions", "model", "data: {\"id\":\"chat_1\",\"model\":\"claude-4.5-sonnet\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"}}]}\n\ndata: {\"id\":\"chat_1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n", types.RelayFormatOpenAI, relayconstant.RelayModeChatCompletions, []int{0, 2}, []int{1}},
		{"claude messages", "/v1/messages", "/v1/messages", "message.model", "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-4.5-sonnet\",\"content\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":0}}}\n\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":1}}\n", types.RelayFormatClaude, relayconstant.RelayModeChatCompletions, []int{0}, []int{1}},
		{"gemini generate content", "/v1beta/models/claude-4.5-sonnet-20250929:streamGenerateContent", "/v1beta/models/{model}:streamGenerateContent", "modelVersion", "data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"hello\"}]}}],\"modelVersion\":\"claude-4.5-sonnet\"}\n\ndata: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"world\"}]}}]}\n", types.RelayFormatGemini, relayconstant.RelayModeGemini, []int{0}, nil},
		{"openai responses", "/v1/responses", "/v1/responses", "response.model", "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"status\":\"in_progress\",\"model\":\"claude-4.5-sonnet\",\"output\":[]}}\n\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"status\":\"completed\",\"model\":\"claude-4.5-sonnet\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\ndata: [DONE]\n", types.RelayFormatOpenAIResponses, relayconstant.RelayModeResponses, []int{0, 2}, []int{1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := advancedCustomResponseInfo(tc.requestURL, tc.routePath, tc.format, tc.mode, relayconvert.ConverterNone)
			info.IsStream, info.ShouldIncludeUsage = true, true
			info.OriginModelName, info.UpstreamModelName = clientModel, upstreamModel
			c, recorder := advancedCustomGinRecorder(tc.requestURL)
			common.SetContextKey(c, constant.ContextKeyOriginalModel, clientModel)

			_, responseErr := (&Adaptor{}).DoResponse(c, advancedCustomHTTPResponse(tc.body), info)
			require.Nil(t, responseErr)
			frames := advancedCustomSSEFrames(recorder.Body.String())
			for _, i := range tc.modelFrames {
				assert.Equal(t, clientModel, gjson.Get(frames[i], tc.modelPath).String())
			}
			for _, i := range tc.sparseFrames {
				assert.False(t, gjson.Get(frames[i], tc.modelPath).Exists())
			}
		})
	}
}

func TestAdaptorCrossProtocolChatUpstreamRequestsStreamUsage(t *testing.T) {
	claudeReq := &dto.ClaudeRequest{
		Model:    "gpt-test",
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "hello"}},
	}
	geminiReq := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "hello"}}}},
	}
	responsesReq := dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustAdvancedCustomRawMessage(t, "hello"),
	}

	tests := []struct {
		name         string
		route        dto.AdvancedCustomRoute
		relayFormat  types.RelayFormat
		relayMode    int
		requestPath  string
		convert      func(*Adaptor, *gin.Context, *relaycommon.RelayInfo) (any, error)
		isStream     bool
		supportUsage bool
		wantUsage    bool
	}{
		{
			name: "claude stream requests usage",
			route: dto.AdvancedCustomRoute{
				IncomingPath: "/v1/messages",
				UpstreamPath: "/v1/chat/completions",
				Converter:    relayconvert.ConverterClaudeMessagesToOpenAIChat,
			},
			relayFormat: types.RelayFormatClaude,
			relayMode:   relayconstant.RelayModeChatCompletions,
			requestPath: "/v1/messages",
			convert: func(a *Adaptor, c *gin.Context, info *relaycommon.RelayInfo) (any, error) {
				return a.ConvertClaudeRequest(c, info, claudeReq)
			},
			isStream:     true,
			supportUsage: true,
			wantUsage:    true,
		},
		{
			name: "gemini stream requests usage",
			route: dto.AdvancedCustomRoute{
				IncomingPath: "/v1beta/models/{model}:generateContent",
				UpstreamPath: "/v1/chat/completions",
				Converter:    relayconvert.ConverterGeminiContentToOpenAIChat,
			},
			relayFormat: types.RelayFormatGemini,
			relayMode:   relayconstant.RelayModeGemini,
			requestPath: "/v1beta/models/gpt-test:generateContent",
			convert: func(a *Adaptor, c *gin.Context, info *relaycommon.RelayInfo) (any, error) {
				return a.ConvertGeminiRequest(c, info, geminiReq)
			},
			isStream:     true,
			supportUsage: true,
			wantUsage:    true,
		},
		{
			name: "responses stream requests usage",
			route: dto.AdvancedCustomRoute{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    relayconvert.ConverterOpenAIResponsesToOpenAIChat,
			},
			relayFormat: types.RelayFormatOpenAIResponses,
			relayMode:   relayconstant.RelayModeResponses,
			requestPath: "/v1/responses",
			convert: func(a *Adaptor, c *gin.Context, info *relaycommon.RelayInfo) (any, error) {
				return a.ConvertOpenAIResponsesRequest(c, info, responsesReq)
			},
			isStream:     true,
			supportUsage: true,
			wantUsage:    true,
		},
		{
			name: "claude non-stream leaves stream options unset",
			route: dto.AdvancedCustomRoute{
				IncomingPath: "/v1/messages",
				UpstreamPath: "/v1/chat/completions",
				Converter:    relayconvert.ConverterClaudeMessagesToOpenAIChat,
			},
			relayFormat: types.RelayFormatClaude,
			relayMode:   relayconstant.RelayModeChatCompletions,
			requestPath: "/v1/messages",
			convert: func(a *Adaptor, c *gin.Context, info *relaycommon.RelayInfo) (any, error) {
				return a.ConvertClaudeRequest(c, info, claudeReq)
			},
			isStream:     false,
			supportUsage: true,
			wantUsage:    false,
		},
		{
			name: "claude stream without stream options support",
			route: dto.AdvancedCustomRoute{
				IncomingPath: "/v1/messages",
				UpstreamPath: "/v1/chat/completions",
				Converter:    relayconvert.ConverterClaudeMessagesToOpenAIChat,
			},
			relayFormat: types.RelayFormatClaude,
			relayMode:   relayconstant.RelayModeChatCompletions,
			requestPath: "/v1/messages",
			convert: func(a *Adaptor, c *gin.Context, info *relaycommon.RelayInfo) (any, error) {
				return a.ConvertClaudeRequest(c, info, claudeReq)
			},
			isStream:     true,
			supportUsage: false,
			wantUsage:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adaptor := &Adaptor{}
			info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{tt.route}})
			info.RelayFormat = tt.relayFormat
			info.RelayMode = tt.relayMode
			info.RequestURLPath = tt.requestPath
			info.IsStream = tt.isStream
			info.SupportStreamOptions = tt.supportUsage
			c := advancedCustomGinContext(tt.requestPath)

			converted, err := tt.convert(adaptor, c, info)
			require.NoError(t, err)
			chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
			require.True(t, ok)

			if !tt.wantUsage {
				assert.Nil(t, chatReq.StreamOptions)
				return
			}
			require.NotNil(t, chatReq.StreamOptions)
			assert.True(t, chatReq.StreamOptions.IncludeUsage)
		})
	}
}

func TestAdaptorRestoresOriginalModelForConvertedGeminiJSON(t *testing.T) {
	const clientModel = "claude-4.5-sonnet-20250929"
	info := advancedCustomResponseInfo("/v1beta/models/claude-4.5-sonnet-20250929:generateContent", "/v1beta/models/{model}:generateContent", types.RelayFormatGemini, relayconstant.RelayModeGemini, relayconvert.ConverterGeminiContentToOpenAIChat)
	info.OriginModelName, info.UpstreamModelName = clientModel, "claude-4.5-sonnet"
	c, recorder := advancedCustomGinRecorder(info.RequestURLPath)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, clientModel)
	body := `{"id":"chat_1","object":"chat.completion","model":"claude-4.5-sonnet","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`

	_, responseErr := (&Adaptor{}).DoResponse(c, advancedCustomHTTPResponse(body), info)
	require.Nil(t, responseErr)
	assert.Equal(t, clientModel, gjson.Get(recorder.Body.String(), "modelVersion").String())
}

func TestAdaptorRestoresOriginalModelForConvertedStream(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })
	const clientModel = "claude-4.5-sonnet-20250929"
	body := "data: {\"id\":\"chat_1\",\"model\":\"claude-4.5-sonnet\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"}}]}\n\ndata: {\"id\":\"chat_1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"}}]}\n\ndata: {\"id\":\"chat_1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n"
	for _, tc := range []struct {
		name, path, routePath, modelPath, converter string
		format                                      types.RelayFormat
		mode                                        int
	}{
		{"responses", "/v1/responses", "/v1/responses", "response.model", relayconvert.ConverterOpenAIResponsesToOpenAIChat, types.RelayFormatOpenAIResponses, relayconstant.RelayModeResponses},
		{"claude", "/v1/messages", "/v1/messages", "message.model", relayconvert.ConverterClaudeMessagesToOpenAIChat, types.RelayFormatClaude, relayconstant.RelayModeChatCompletions},
		{"gemini", "/v1beta/models/claude-4.5-sonnet-20250929:streamGenerateContent", "/v1beta/models/{model}:streamGenerateContent", "modelVersion", relayconvert.ConverterGeminiContentToOpenAIChat, types.RelayFormatGemini, relayconstant.RelayModeGemini},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := advancedCustomResponseInfo(tc.path, tc.routePath, tc.format, tc.mode, tc.converter)
			info.IsStream = true
			info.OriginModelName, info.UpstreamModelName = clientModel, "claude-4.5-sonnet"
			c, recorder := advancedCustomGinRecorder(tc.path)
			common.SetContextKey(c, constant.ContextKeyOriginalModel, clientModel)

			_, responseErr := (&Adaptor{}).DoResponse(c, advancedCustomHTTPResponse(body), info)
			require.Nil(t, responseErr)
			var sawModel bool
			for _, frame := range advancedCustomSSEFrames(recorder.Body.String()) {
				if tc.format == types.RelayFormatGemini || gjson.Get(frame, "type").String() == "message_start" || gjson.Get(frame, "response").IsObject() {
					sawModel = true
					assert.Equal(t, clientModel, gjson.Get(frame, tc.modelPath).String())
				} else {
					assert.False(t, gjson.Get(frame, tc.modelPath).Exists())
				}
			}
			assert.True(t, sawModel)
			assert.Contains(t, recorder.Body.String(), "hello")
		})
	}
}

func TestBufferedResponsesToChatRestoresOriginalModel(t *testing.T) {
	const clientModel = "claude-4.5-sonnet-20250929"
	body := "data: {\"type\":\"response.created\",\"response\":{\"model\":\"claude-4.5-sonnet\"}}\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":2,\"output_tokens\":3,\"total_tokens\":5}}}\n"

	for _, tc := range []struct {
		name, path string
		format     types.RelayFormat
	}{
		{"openai chat", "/v1/chat/completions", types.RelayFormatOpenAI},
		{"claude messages", "/v1/messages", types.RelayFormatClaude},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := advancedCustomResponseInfo(tc.path, tc.path, tc.format, relayconstant.RelayModeChatCompletions, relayconvert.ConverterNone)
			info.OriginModelName, info.UpstreamModelName = clientModel, "claude-4.5-sonnet"
			c, recorder := advancedCustomGinRecorder(tc.path)
			common.SetContextKey(c, constant.ContextKeyOriginalModel, clientModel)

			_, responseErr := openai.OaiResponsesToChatBufferedStreamHandler(c, info, advancedCustomHTTPResponse(body))
			require.Nil(t, responseErr)
			assert.Equal(t, clientModel, gjson.Get(recorder.Body.String(), "model").String())
		})
	}
}

func TestAdaptorLeavesResponseModelUntouchedOutsideRestoreScope(t *testing.T) {
	const upstreamModel = "claude-4.5-sonnet"
	body := `{"id":"chat_1","object":"chat.completion","model":"claude-4.5-sonnet","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`

	for _, tc := range []struct {
		name, requestURL string
		setOriginal      bool
	}{
		{"missing original model", "/v1/chat/completions", false},
		{"out of scope endpoint", "/v1/embeddings", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := advancedCustomResponseInfo(tc.requestURL, tc.requestURL, types.RelayFormatOpenAI, relayconstant.RelayModeChatCompletions, relayconvert.ConverterNone)
			info.OriginModelName, info.UpstreamModelName = "claude-4.5-sonnet-20250929", upstreamModel
			c, recorder := advancedCustomGinRecorder(tc.requestURL)
			if tc.setOriginal {
				common.SetContextKey(c, constant.ContextKeyOriginalModel, info.OriginModelName)
			}

			_, responseErr := (&Adaptor{}).DoResponse(c, advancedCustomHTTPResponse(body), info)
			require.Nil(t, responseErr)
			assert.Equal(t, upstreamModel, gjson.Get(recorder.Body.String(), "model").String())
		})
	}
}

func advancedCustomResponseInfo(requestURL string, routePath string, format types.RelayFormat, relayMode int, converter string) *relaycommon.RelayInfo {
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{{
		IncomingPath: routePath,
		UpstreamPath: "https://upstream.example/v1/chat/completions",
		Converter:    converter,
	}}})
	info.RequestURLPath = requestURL
	info.RelayFormat = format
	info.RelayMode = relayMode
	return info
}

func advancedCustomGinRecorder(path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	c.Request.Header.Set("Content-Type", "application/json")
	return c, recorder
}

func advancedCustomHTTPResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func advancedCustomSSEFrames(body string) []string {
	frames := make([]string, 0)
	for line := range strings.SplitSeq(body, "\n") {
		data, ok := strings.CutPrefix(line, "data: ")
		if ok && gjson.Valid(data) {
			frames = append(frames, data)
		}
	}
	return frames
}

func advancedCustomRelayInfo(config *dto.AdvancedCustomConfig) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RequestURLPath:  "/v1/chat/completions",
		OriginModelName: "gpt-test",
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:            "sk-test",
			ChannelBaseUrl:    "https://fallback.example",
			ChannelType:       constant.ChannelTypeAdvancedCustom,
			UpstreamModelName: "gpt-test",
			ChannelOtherSettings: dto.ChannelOtherSettings{
				AdvancedCustom: config,
			},
		},
	}
}

func advancedCustomGinContext(path string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func mustAdvancedCustomRawMessage(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := common.Marshal(value)
	require.NoError(t, err)
	return raw
}
