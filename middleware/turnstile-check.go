package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cap"
	"github.com/gin-gonic/gin"
)

type botProtectionVerifyResponse struct {
	Success bool `json:"success"`
}

func botProtectionEnabled() bool {
	return common.BotProtectionActive()
}

func verifyCapJsToken(token string) (bool, string) {
	if common.BotProtectionUsesCapJs() {
		parts := strings.Split(token, ":")
		if len(parts) != 3 {
			return false, "Cap.js verification failed, please refresh and try again!"
		}
		if parts[0] != common.BuiltinCapSiteKey {
			return verifyCapJsTokenRemote(token)
		}
		ok, err := cap.ConsumeRedeemToken(token)
		if err != nil {
			return false, err.Error()
		}
		if !ok {
			return false, "Cap.js verification failed, please refresh and try again!"
		}
		return true, ""
	}
	return verifyCapJsTokenRemote(token)
}

func verifyCapJsTokenRemote(token string) (bool, string) {
	endpoint := strings.TrimSpace(common.CapJsApiEndpoint)
	if endpoint == "" {
		return false, "Cap.js endpoint is not configured"
	}
	endpoint = strings.TrimSuffix(endpoint, "/")
	verifyURL := endpoint + "/siteverify"

	payload := map[string]string{
		"secret":   common.CapJsSecretKey,
		"response": token,
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return false, err.Error()
	}

	req, err := http.NewRequest(http.MethodPost, verifyURL, bytes.NewReader(body))
	if err != nil {
		return false, err.Error()
	}
	req.Header.Set("Content-Type", "application/json")

	rawRes, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer rawRes.Body.Close()

	respBody, err := io.ReadAll(rawRes.Body)
	if err != nil {
		return false, err.Error()
	}

	var res botProtectionVerifyResponse
	if err := common.Unmarshal(respBody, &res); err != nil {
		return false, err.Error()
	}
	if !res.Success {
		return false, "Cap.js verification failed, please refresh and try again!"
	}
	return true, ""
}

func verifyTurnstileToken(token, clientIP string) (bool, string) {
	rawRes, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", url.Values{
		"secret":   {common.TurnstileSecretKey},
		"response": {token},
		"remoteip": {clientIP},
	})
	if err != nil {
		return false, err.Error()
	}
	defer rawRes.Body.Close()

	respBody, err := io.ReadAll(rawRes.Body)
	if err != nil {
		return false, err.Error()
	}

	var res botProtectionVerifyResponse
	if err := common.Unmarshal(respBody, &res); err != nil {
		return false, err.Error()
	}
	if !res.Success {
		return false, "Turnstile verification failed, please refresh and try again!"
	}
	return true, ""
}

func TurnstileCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !botProtectionEnabled() {
			c.Next()
			return
		}

		token := c.Query("turnstile")
		if token == "" {
			common.ApiErrorT(c, "Human verification token is empty")
			c.Abort()
			return
		}

		var ok bool
		var message string
		if common.BotProtectionUsesCapJs() {
			ok, message = verifyCapJsToken(token)
		} else {
			ok, message = verifyTurnstileToken(token, c.ClientIP())
		}
		if !ok {
			if message != "" {
				common.SysLog(message)
			}
			common.ApiErrorT(c, message)
			c.Abort()
			return
		}
		c.Next()
	}
}
