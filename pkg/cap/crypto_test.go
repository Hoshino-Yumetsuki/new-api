package cap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJwtSignVerifyRoundtrip(t *testing.T) {
	secret := "0123456789abcdef"
	payload := map[string]any{"scope": "builtin", "exp": int64(9_999_999_999_999)}
	token, err := jwtSign(payload, secret)
	require.NoError(t, err)
	got, ok := jwtVerify(token, secret)
	require.True(t, ok)
	assert.Equal(t, "builtin", got["scope"])
	assert.EqualValues(t, 9_999_999_999_999, got["exp"])
}

func TestJwtVerifyRejectsWrongSecret(t *testing.T) {
	token, err := jwtSign(map[string]any{"k": "v"}, "0123456789abcdef")
	require.NoError(t, err)
	_, ok := jwtVerify(token, "fedcba9876543210")
	assert.False(t, ok)
}

func TestConsumeRedeemTokenMemory(t *testing.T) {
	token := "builtin:abc:def"
	exp := int64(9_999_999_999_999)
	require.NoError(t, StoreRedeemToken(token, exp))
	ok, err := ConsumeRedeemToken(token)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = ConsumeRedeemToken(token)
	require.NoError(t, err)
	assert.False(t, ok)
}