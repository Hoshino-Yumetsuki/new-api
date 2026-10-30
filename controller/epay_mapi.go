package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

var epayMAPIHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// The opt-in gateway contract treats qrcode as an HTTP cashier URL, not QR content.
func requestEpayMAPI(c *gin.Context, client *epay.Client, params map[string]string) (string, error) {
	params["clientip"] = c.ClientIP()
	params, err := epay.GenerateParamsWithError(params, client.Config.Key)
	if err != nil {
		return "", err
	}
	values := make(url.Values, len(params))
	for name, value := range params {
		values.Set(name, value)
	}
	endpoint := *client.BaseUrl
	endpoint.Path = path.Join(endpoint.Path, "mapi.php")
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, endpoint.String(), strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := epayMAPIHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Epay mapi returned HTTP %d", response.StatusCode)
	}
	var result struct {
		Code   int    `json:"code"`
		Msg    string `json:"msg"`
		QRCode string `json:"qrcode"`
	}
	if err := common.DecodeJson(io.LimitReader(response.Body, 1<<20), &result); err != nil {
		return "", fmt.Errorf("invalid Epay mapi response: %w", err)
	}
	if result.Code != 1 {
		return "", fmt.Errorf("Epay mapi payment failed: %s", result.Msg)
	}
	cashier, err := url.Parse(result.QRCode)
	if err != nil || cashier.Hostname() == "" || (cashier.Scheme != "https" && cashier.Scheme != "http") || cashier.User != nil {
		return "", errors.New("Epay mapi qrcode must be an absolute HTTP(S) cashier URL")
	}
	return result.QRCode, nil
}
