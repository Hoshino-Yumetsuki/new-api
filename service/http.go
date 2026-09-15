package service

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// RestoreResponseModel restores the model name requested by the client in a
// serialized protocol response. It deliberately patches only the protocol's
// model field, leaving upstream response data and unknown extensions intact.
func RestoreResponseModel(c *gin.Context, data []byte) []byte {
	if c == nil || c.Request == nil || c.Request.URL == nil || len(data) == 0 {
		return data
	}
	originalModel := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	if originalModel == "" {
		return data
	}

	path := c.Request.URL.Path
	field := ""
	requireModel := false
	switch {
	case path == "/v1/chat/completions" || path == "/pg/chat/completions":
		field = "model"
	case path == "/v1/messages":
		field = "model"
		messageType := gjson.GetBytes(data, "type").String()
		requireModel = messageType == "message" || messageType == "message_start"
		if messageType == "message_start" {
			field = "message.model"
		}
	case path == "/v1/responses" || path == "/v1/responses/compact":
		field = "model"
		if responseType := gjson.GetBytes(data, "type"); responseType.Type == gjson.String && strings.HasPrefix(responseType.String(), "response.") {
			field = "response.model"
			requireModel = true
		}
	case (strings.HasPrefix(path, "/v1/models/") || strings.HasPrefix(path, "/v1beta/models/")) &&
		(strings.HasSuffix(path, ":generateContent") || strings.HasSuffix(path, ":streamGenerateContent")):
		field = "modelVersion"
	default:
		return data
	}

	current := gjson.GetBytes(data, field)
	if current.Exists() {
		if current.Type != gjson.String || current.String() == originalModel {
			return data
		}
	} else if field == "modelVersion" {
		if !gjson.GetBytes(data, "candidates").IsArray() && !gjson.GetBytes(data, "usageMetadata").IsObject() {
			return data
		}
	} else if !requireModel || (field != "model" && !gjson.GetBytes(data, strings.TrimSuffix(field, ".model")).IsObject()) {
		return data
	}
	if !gjson.ValidBytes(data) {
		return data
	}
	restored, err := sjson.SetBytes(data, field, originalModel)
	if err != nil {
		return data
	}
	return restored
}

func CloseResponseBodyGracefully(httpResponse *http.Response) {
	if httpResponse == nil || httpResponse.Body == nil {
		return
	}
	err := httpResponse.Body.Close()
	if err != nil {
		common.SysError("failed to close response body: " + err.Error())
	}
}

// ShouldCopyUpstreamHeader checks whether a given upstream response header
// should be copied to the client response. It returns false for Content-Length
// (managed separately) and X-Oneapi-Request-Id (to preserve the local instance
// ID). When the upstream header is X-Oneapi-Request-Id, the value is captured
// into the Gin context for later logging.
func ShouldCopyUpstreamHeader(c *gin.Context, k string, v []string) bool {
	if strings.EqualFold(k, "Content-Length") {
		return false
	}
	if strings.EqualFold(k, common.RequestIdKey) {
		if c != nil && len(v) > 0 {
			c.Set(common.UpstreamRequestIdKey, v[0])
		}
		return false
	}
	return true
}

func IOCopyBytesGracefully(c *gin.Context, src *http.Response, data []byte) {
	if c.Writer == nil {
		return
	}
	data = RestoreResponseModel(c, data)

	body := io.NopCloser(bytes.NewBuffer(data))

	// We shouldn't set the header before we parse the response body, because the parse part may fail.
	// And then we will have to send an error response, but in this case, the header has already been set.
	// So the httpClient will be confused by the response.
	// For example, Postman will report error, and we cannot check the response at all.
	if src != nil {
		for k, v := range src.Header {
			if !ShouldCopyUpstreamHeader(c, k, v) {
				continue
			}
			c.Writer.Header().Set(k, v[0])
		}
	}

	// set Content-Length header manually BEFORE calling WriteHeader
	c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))

	// Write header with status code (this sends the headers)
	if src != nil {
		c.Writer.WriteHeader(src.StatusCode)
	} else {
		c.Writer.WriteHeader(http.StatusOK)
	}

	_, err := io.Copy(c.Writer, body)
	if err != nil {
		logger.LogError(c, fmt.Sprintf("failed to copy response body: %s", err.Error()))
	}
	c.Writer.Flush()
}
