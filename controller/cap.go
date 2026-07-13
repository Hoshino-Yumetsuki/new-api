package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cap"

	"github.com/gin-gonic/gin"
)

func capSiteKeyAllowed(siteKey string) bool {
	return siteKey == common.BuiltinCapSiteKey && common.BotProtectionUsesCapJs()
}

func CapChallenge(c *gin.Context) {
	siteKey := c.Param("siteKey")
	if !capSiteKeyAllowed(siteKey) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid site key or secret"})
		return
	}
	secret := common.CapJwtSecret()
	resp, err := cap.GenerateChallenge(secret, cap.GenerateOpts{Scope: siteKey})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate challenge"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

type capRedeemBody struct {
	Token     string    `json:"token"`
	Solutions []float64 `json:"solutions"`
}

func CapRedeem(c *gin.Context) {
	siteKey := c.Param("siteKey")
	if !capSiteKeyAllowed(siteKey) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid site key"})
		return
	}
	var body capRedeemBody
	if err := c.ShouldBindJSON(&body); err != nil || body.Token == "" || len(body.Solutions) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields"})
		return
	}
	solutions := make([]int64, len(body.Solutions))
	for i, v := range body.Solutions {
		solutions[i] = int64(v)
	}
	secret := common.CapJwtSecret()
	result, reason, err := cap.ValidateChallenge(secret, cap.RedeemRequest{
		Token:     body.Token,
		Solutions: solutions,
	}, cap.ValidateOpts{
		Scope: siteKey,
		SignRedeem: func(scope string, expires int64) (string, error) {
			id, err := cap.RandomHex(8)
			if err != nil {
				return "", err
			}
			ver, err := cap.RandomHex(15)
			if err != nil {
				return "", err
			}
			return scope + ":" + id + ":" + ver, nil
		},
		ConsumeNonce: cap.ClaimChallengeNonce,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Validation failed"})
		return
	}
	if result == nil {
		status := http.StatusForbidden
		if reason == "instr_timeout" {
			status = http.StatusTooManyRequests
		}
		if reason == "missing_token" || reason == "missing_solutions" || reason == "invalid_solutions" {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": reason})
		return
	}
	if err := cap.StoreRedeemToken(result.Token, result.Expires); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store token"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func CapSiteverify(c *gin.Context) {
	var req cap.SiteverifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, cap.SiteverifyResponse{Success: false, Error: "Missing required parameters"})
		return
	}
	req.Secret = strings.TrimSpace(req.Secret)
	req.Response = strings.TrimSpace(req.Response)
	if req.Secret == "" || req.Response == "" {
		c.JSON(http.StatusBadRequest, cap.SiteverifyResponse{Success: false, Error: "Missing required parameters"})
		return
	}
	parts := strings.Split(req.Response, ":")
	if len(parts) != 3 {
		c.JSON(http.StatusBadRequest, cap.SiteverifyResponse{Success: false, Error: "Missing required parameters"})
		return
	}
	siteKey := parts[0]
	if c.Param("siteKey") != "" && c.Param("siteKey") != siteKey {
		c.JSON(http.StatusNotFound, cap.SiteverifyResponse{Success: false, Error: "Invalid site key or secret"})
		return
	}
	if !capSiteKeyAllowed(siteKey) {
		c.JSON(http.StatusNotFound, cap.SiteverifyResponse{Success: false, Error: "Invalid site key or secret"})
		return
	}
	if req.Secret != strings.TrimSpace(common.CapJsSecretKey) && req.Secret != common.CapJwtSecret() {
		c.JSON(http.StatusForbidden, cap.SiteverifyResponse{Success: false, Error: "Invalid site key or secret"})
		return
	}
	ok, err := cap.ConsumeRedeemToken(req.Response)
	if err != nil {
		c.JSON(http.StatusInternalServerError, cap.SiteverifyResponse{Success: false, Error: "Internal server error"})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, cap.SiteverifyResponse{Success: false, Error: "Token not found"})
		return
	}
	c.JSON(http.StatusOK, cap.SiteverifyResponse{Success: true})
}