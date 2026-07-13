package cap

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

const (
	DefaultChallengeCount      = 80
	DefaultChallengeSize       = 32
	DefaultChallengeDifficulty = 4
	DefaultChallengeTTL        = 15 * time.Minute
	DefaultRedeemTokenTTL      = 2 * time.Hour
	MinSecretLen               = 16
)

var (
	ErrSecretRequired = errors.New("cap secret must be at least 16 bytes")
	ErrInvalidSiteKey = errors.New("invalid cap site key")
)

type GenerateOpts struct {
	Scope      string
	Count      int
	Size       int
	Difficulty int
	ExpiresMs  int64
}

type ChallengeResponse struct {
	Challenge struct {
		C int `json:"c"`
		S int `json:"s"`
		D int `json:"d"`
	} `json:"challenge"`
	Token   string `json:"token"`
	Expires int64  `json:"expires"`
}

type RedeemRequest struct {
	Token     string  `json:"token"`
	Solutions []int64 `json:"solutions"`
}

type RedeemSuccess struct {
	Success bool   `json:"success"`
	Token   string `json:"token"`
	Expires int64  `json:"expires"`
}

type SiteverifyRequest struct {
	Secret   string `json:"secret"`
	Response string `json:"response"`
}

type SiteverifyResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func assertSecret(secret string) error {
	if len(secret) < MinSecretLen {
		return ErrSecretRequired
	}
	return nil
}

// GenerateChallenge creates a PoW challenge compatible with cap-widget (capjs-core v1, no instrumentation).
func GenerateChallenge(secret string, opts GenerateOpts) (*ChallengeResponse, error) {
	if err := assertSecret(secret); err != nil {
		return nil, err
	}
	c := opts.Count
	if c <= 0 {
		c = DefaultChallengeCount
	}
	s := opts.Size
	if s <= 0 {
		s = DefaultChallengeSize
	}
	d := opts.Difficulty
	if d <= 0 {
		d = DefaultChallengeDifficulty
	}
	ttl := opts.ExpiresMs
	if ttl <= 0 {
		ttl = DefaultChallengeTTL.Milliseconds()
	}
	now := time.Now().UnixMilli()
	expires := now + ttl

	nonce, err := randomHex(25)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"n":   nonce,
		"c":   c,
		"s":   s,
		"d":   d,
		"exp": float64(expires),
		"iat": float64(now),
	}
	if opts.Scope != "" {
		payload["sk"] = opts.Scope
	}
	token, err := jwtSign(payload, secret)
	if err != nil {
		return nil, err
	}
	resp := &ChallengeResponse{
		Token:   token,
		Expires: expires,
	}
	resp.Challenge.C = c
	resp.Challenge.S = s
	resp.Challenge.D = d
	return resp, nil
}

type ValidateOpts struct {
	Scope       string
	SignRedeem  func(scope string, expires int64) (token string, err error)
	ConsumeNonce func(sigHex string, ttlMs int64) (claimed bool, err error)
	RedeemTTL   time.Duration
}

// ValidateChallenge redeems a solved challenge and returns a siteverify token (siteKey:id:secret).
func ValidateChallenge(secret string, req RedeemRequest, opts ValidateOpts) (*RedeemSuccess, string, error) {
	if err := assertSecret(secret); err != nil {
		return nil, "", err
	}
	if req.Token == "" || len(req.Solutions) == 0 {
		return nil, "missing_token", nil
	}
	payload, ok := jwtVerify(req.Token, secret)
	if !ok {
		return nil, "invalid_token", nil
	}
	if opts.Scope != "" {
		if sk, _ := payload["sk"].(string); sk != opts.Scope {
			return nil, "scope_mismatch", nil
		}
	}
	exp, ok := payloadInt64(payload["exp"])
	if !ok || exp < time.Now().UnixMilli() {
		return nil, "expired", nil
	}
	c, ok := payloadInt64(payload["c"])
	if !ok || c < 1 || int(c) != len(req.Solutions) {
		return nil, "invalid_solutions", nil
	}
	size, ok := payloadInt64(payload["s"])
	if !ok {
		return nil, "invalid_token", nil
	}
	difficulty, ok := payloadInt64(payload["d"])
	if !ok {
		return nil, "invalid_token", nil
	}

	tokenFnv := fnv1a(req.Token)
	for i := 0; i < int(c); i++ {
		idxStr := strconv.Itoa(i + 1)
		saltSeed := fnv1aResume(tokenFnv, idxStr)
		targetSeed := fnv1aResume(saltSeed, "d")
		salt := prngFromHash(saltSeed, int(size))
		target := prngFromHash(targetSeed, int(difficulty))
		hash := hashConcatSaltSolution(salt, req.Solutions[i])
		if !powMatchesPrefix(hash, parseHexPrefix(target)) {
			return nil, "invalid_solution", nil
		}
	}

	if opts.ConsumeNonce != nil {
		sig := jwtSigHex(req.Token)
		if sig == "" {
			return nil, "invalid_token", nil
		}
		ttlMs := exp - time.Now().UnixMilli()
		if ttlMs < 1 {
			ttlMs = 1
		}
		claimed, err := opts.ConsumeNonce(sig, ttlMs)
		if err != nil {
			return nil, "nonce_store_error", err
		}
		if !claimed {
			return nil, "already_redeemed", nil
		}
	}

	redeemTTL := opts.RedeemTTL
	if redeemTTL <= 0 {
		redeemTTL = DefaultRedeemTokenTTL
	}
	tokenExpires := time.Now().Add(redeemTTL).UnixMilli()

	var redeemToken string
	if opts.SignRedeem != nil {
		var err error
		redeemToken, err = opts.SignRedeem(opts.Scope, tokenExpires)
		if err != nil {
			return nil, "", err
		}
	} else {
		id, err := randomHex(8)
		if err != nil {
			return nil, "", err
		}
		ver, err := randomHex(15)
		if err != nil {
			return nil, "", err
		}
		redeemToken = fmt.Sprintf("%s:%s", id, ver)
	}

	return &RedeemSuccess{
		Success: true,
		Token:   redeemToken,
		Expires: tokenExpires,
	}, "", nil
}