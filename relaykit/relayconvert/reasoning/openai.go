package reasoning

import (
	"strings"

	"github.com/QuantumNous/new-api/relaykit/dto"
)

// OpenAIEffort projects a portable intent onto the target's documented effort
// range. Unknown OpenAI-compatible models keep their requested effort.
// https://developers.openai.com/api/docs/guides/reasoning#reasoning-effort
// https://developers.openai.com/api/docs/models
func OpenAIEffort(model string, intent Intent) Effort {
	effort := EffectiveEffort(intent)
	if effort == "" {
		return ""
	}
	var lowest, highest Effort
	switch dto.OpenAIBaseModel(strings.TrimPrefix(model, "openai/")) {
	case "gpt-4o", "gpt-4o-mini", "gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano", "gpt-4", "gpt-4-turbo", "gpt-3.5-turbo":
		return ""
	case "gpt-5-pro":
		return EffortHigh
	case "gpt-5.2-pro", "gpt-5.4-pro", "gpt-5.5-pro":
		lowest, highest = EffortMedium, EffortXHigh
	case "gpt-5", "gpt-5-mini", "gpt-5-nano":
		lowest, highest = EffortMinimal, EffortHigh
	case "o1", "o3", "o3-mini", "o4-mini", "gpt-5-codex", "gpt-5.1-codex", "gpt-5.1-codex-mini":
		lowest, highest = EffortLow, EffortHigh
	case "gpt-5.1":
		lowest, highest = EffortNone, EffortHigh
	case "gpt-5.1-codex-max", "gpt-5.2-codex", "gpt-5.3-codex":
		lowest, highest = EffortLow, EffortXHigh
	case "gpt-5.2", "gpt-5.4", "gpt-5.4-mini", "gpt-5.4-nano", "gpt-5.5":
		lowest, highest = EffortNone, EffortXHigh
	case "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-6-sol", "gpt-6-luna", "gpt-6.1-terra":
		lowest, highest = EffortNone, EffortMax
	case "gpt-6-astra", "gpt-6.1-sol", "gpt-6.2-astra", "gpt-6.2-sol":
		lowest, highest = EffortLow, EffortMax
	default:
		return effort
	}
	switch effort {
	case EffortNone:
		return lowest
	case EffortMinimal:
		if lowest == EffortNone {
			return EffortLow // minimal requests thinking; none turns it off.
		}
		return lowest
	case EffortLow:
		if lowest == EffortMedium {
			return lowest
		}
	case EffortMax:
		return highest
	case EffortXHigh:
		if highest == EffortHigh {
			return highest
		}
	}
	return effort
}
