package common

import (
	"strconv"
	"strings"
)

const BuiltinCapSiteKey = "builtin"

const (
	BotProtectionProviderTurnstile = "turnstile"
	BotProtectionProviderCapJs     = "capjs"
)

var BotProtectionEnabled bool
var BotProtectionProvider string

func SyncBotProtectionLegacyFlags() {
	if BotProtectionEnabled {
		switch BotProtectionProvider {
		case BotProtectionProviderCapJs:
			CapJsCheckEnabled = true
			TurnstileCheckEnabled = false
		default:
			TurnstileCheckEnabled = true
			CapJsCheckEnabled = false
			if BotProtectionProvider == "" {
				BotProtectionProvider = BotProtectionProviderTurnstile
			}
		}
		return
	}
	TurnstileCheckEnabled = false
	CapJsCheckEnabled = false
}

func BotProtectionActive() bool {
	SyncBotProtectionLegacyFlags()
	return BotProtectionEnabled
}

func BotProtectionUsesCapJs() bool {
	SyncBotProtectionLegacyFlags()
	return BotProtectionEnabled && BotProtectionProvider == BotProtectionProviderCapJs
}

func BuiltinCapAPIEndpoint() string {
	return "/api/cap/" + BuiltinCapSiteKey + "/"
}

func EffectiveCapJsAPIEndpoint() string {
	if BotProtectionUsesCapJs() {
		return BuiltinCapAPIEndpoint()
	}
	return strings.TrimSpace(CapJsApiEndpoint)
}

func CapJwtSecret() string {
	secret := strings.TrimSpace(CapJsSecretKey)
	if len(secret) >= 16 {
		return secret
	}
	// ponytail: fallback so builtin cap works before admin sets a long secret
	if len(SessionSecret) >= 16 {
		return SessionSecret
	}
	return SessionSecret + "-capjs-fallback"
}

// MigrateBotProtectionFromLegacy derives unified bot-protection options from legacy Turnstile/Cap flags.
func MigrateBotProtectionFromLegacy() {
	OptionMapRWMutex.RLock()
	_, hasEnabled := OptionMap["BotProtectionEnabled"]
	_, hasProvider := OptionMap["BotProtectionProvider"]
	OptionMapRWMutex.RUnlock()
	if hasEnabled && hasProvider {
		return
	}
	if CapJsCheckEnabled {
		BotProtectionEnabled = true
		BotProtectionProvider = BotProtectionProviderCapJs
	} else if TurnstileCheckEnabled {
		BotProtectionEnabled = true
		BotProtectionProvider = BotProtectionProviderTurnstile
	} else {
		BotProtectionEnabled = false
		if BotProtectionProvider == "" {
			BotProtectionProvider = BotProtectionProviderTurnstile
		}
	}
	SyncBotProtectionLegacyFlags()
	OptionMapRWMutex.Lock()
	OptionMap["BotProtectionEnabled"] = strconv.FormatBool(BotProtectionEnabled)
	OptionMap["BotProtectionProvider"] = BotProtectionProvider
	OptionMapRWMutex.Unlock()
}