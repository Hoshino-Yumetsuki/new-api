package cap

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const jwtHeaderB64 = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"

func RandomHex(byteCount int) (string, error) {
	return randomHex(byteCount)
}

func randomHex(byteCount int) (string, error) {
	b := make([]byte, byteCount)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum[:])
}

func sha256Bytes(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func b64url(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func b64urlDecode(str string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(str)
}

func jwtSign(payload any, secret string) (string, error) {
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	body := b64url(jsonBytes)
	sigInput := jwtHeaderB64 + "." + body
	sig := hmacSHA256([]byte(secret), []byte(sigInput))
	return sigInput + "." + b64url(sig), nil
}

func jwtVerify(token, secret string) (map[string]any, bool) {
	if token == "" {
		return nil, false
	}
	firstDot := strings.Index(token, ".")
	if firstDot < 1 {
		return nil, false
	}
	lastDot := strings.LastIndex(token, ".")
	if lastDot <= firstDot {
		return nil, false
	}
	if strings.Index(token[firstDot+1:], ".") != lastDot-firstDot-1 {
		return nil, false
	}
	sigInput := token[:lastDot]
	expected := hmacSHA256([]byte(secret), []byte(sigInput))
	actual, err := b64urlDecode(token[lastDot+1:])
	if err != nil || len(actual) != len(expected) {
		return nil, false
	}
	var diff byte
	for i := range expected {
		diff |= expected[i] ^ actual[i]
	}
	if diff != 0 {
		return nil, false
	}
	body, err := b64urlDecode(token[firstDot+1 : lastDot])
	if err != nil {
		return nil, false
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false
	}
	return payload, true
}

func jwtSigHex(token string) string {
	lastDot := strings.LastIndex(token, ".")
	if lastDot < 0 {
		return ""
	}
	sig, err := b64urlDecode(token[lastDot+1:])
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sig)
}

type hexPrefix struct {
	bytes         []byte
	fullBytes     int
	partialNibble int // -1 if none
}

func parseHexPrefix(target string) hexPrefix {
	lenT := len(target)
	fullBytes := lenT >> 1
	out := hexPrefix{fullBytes: fullBytes, partialNibble: -1}
	if fullBytes > 0 {
		out.bytes = make([]byte, fullBytes)
		for i := 0; i < fullBytes; i++ {
			a := hexDigit(target[i*2])
			b := hexDigit(target[i*2+1])
			if a < 0 || b < 0 {
				return hexPrefix{partialNibble: -1}
			}
			out.bytes[i] = byte((a << 4) | b)
		}
	}
	if lenT&1 != 0 {
		c := hexDigit(target[lenT-1])
		if c < 0 {
			return hexPrefix{partialNibble: -1}
		}
		out.partialNibble = c
	}
	return out
}

func hexDigit(c byte) int {
	if c >= '0' && c <= '9' {
		return int(c - '0')
	}
	if c >= 'a' && c <= 'f' {
		return int(c - 'a' + 10)
	}
	if c >= 'A' && c <= 'F' {
		return int(c - 'A' + 10)
	}
	return -1
}

func powMatchesPrefix(hashBytes []byte, parsed hexPrefix) bool {
	for i := 0; i < parsed.fullBytes; i++ {
		if hashBytes[i] != parsed.bytes[i] {
			return false
		}
	}
	if parsed.partialNibble != -1 {
		if len(hashBytes) <= parsed.fullBytes {
			return false
		}
		if hashBytes[parsed.fullBytes]>>4 != byte(parsed.partialNibble) {
			return false
		}
	}
	return true
}

func payloadInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int:
		return int64(n), true
	case int64:
		return n, true
	default:
		return 0, false
	}
}

func solutionNumber(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		if n != float64(int64(n)) {
			return 0, false
		}
		return int64(n), true
	case int:
		return int64(n), true
	case int64:
		return n, true
	default:
		return 0, false
	}
}

func hashConcatSaltSolution(salt string, solution int64) []byte {
	return sha256Bytes(salt + strconv.FormatInt(solution, 10))
}