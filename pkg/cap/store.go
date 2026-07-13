package cap

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
)

const tokenKeyPrefix = "cap:token:"
const nonceKeyPrefix = "cap:nonce:"

var memTokens sync.Map
var memNonces sync.Map

type memEntry struct {
	expires int64
}

func StoreRedeemToken(token string, expiresMs int64) error {
	ttl := time.Until(time.UnixMilli(expiresMs))
	if ttl < time.Second {
		ttl = time.Second
	}
	val := strconv.FormatInt(expiresMs, 10)
	key := tokenKeyPrefix + token
	if common.RedisEnabled && common.RDB != nil {
		return common.RedisSet(key, val, ttl)
	}
	memTokens.Store(key, memEntry{expires: expiresMs})
	return nil
}

// ConsumeRedeemToken returns true if token existed and was not expired.
func ConsumeRedeemToken(token string) (bool, error) {
	key := tokenKeyPrefix + token
	if common.RedisEnabled && common.RDB != nil {
		ctx := context.Background()
		val, err := common.RDB.GetDel(ctx, key).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return false, nil
			}
			return false, err
		}
		expires, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return false, nil
		}
		if expires < time.Now().UnixMilli() {
			return false, nil
		}
		return true, nil
	}
	raw, ok := memTokens.LoadAndDelete(key)
	if !ok {
		return false, nil
	}
	ent := raw.(memEntry)
	if ent.expires < time.Now().UnixMilli() {
		return false, nil
	}
	return true, nil
}

func ClaimChallengeNonce(sigHex string, ttlMs int64) (bool, error) {
	if sigHex == "" {
		return false, nil
	}
	ttl := time.Duration(ttlMs) * time.Millisecond
	if ttl < time.Second {
		ttl = time.Second
	}
	key := nonceKeyPrefix + sigHex
	if common.RedisEnabled && common.RDB != nil {
		ctx := context.Background()
		ok, err := common.RDB.SetNX(ctx, key, "1", ttl).Result()
		return ok, err
	}
	now := time.Now().UnixMilli()
	if v, loaded := memNonces.LoadOrStore(key, now+ttlMs); loaded {
		ent := v.(int64)
		if ent < now {
			memNonces.Store(key, now+ttlMs)
			return true, nil
		}
		return false, nil
	}
	return true, nil
}