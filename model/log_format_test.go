package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/require"
)

func TestFormatUserLogsModelRedirectVisibility(t *testing.T) {
	setting := operation_setting.GetGeneralSetting()
	previous := setting.HideModelRedirectForNonAdmin
	t.Cleanup(func() { setting.HideModelRedirectForNonAdmin = previous })

	tests := []struct {
		name         string
		hide         bool
		wantMapped   bool
		wantUpstream bool
	}{
		{name: "disabled preserves redirect metadata", hide: false, wantMapped: true, wantUpstream: true},
		{name: "enabled strips redirect metadata", hide: true, wantMapped: false, wantUpstream: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting.HideModelRedirectForNonAdmin = tt.hide
			logs := []*Log{{Other: common.MapToJsonStr(map[string]interface{}{
				"is_model_mapped":     true,
				"upstream_model_name": "upstream-model",
				"model_price":         0.004,
			})}}

			formatUserLogs(logs, 0)

			parsed, err := common.StrToMap(logs[0].Other)
			require.NoError(t, err)
			_, mapped := parsed["is_model_mapped"]
			_, upstream := parsed["upstream_model_name"]
			require.Equal(t, tt.wantMapped, mapped)
			require.Equal(t, tt.wantUpstream, upstream)
			require.Equal(t, float64(0.004), parsed["model_price"])
		})
	}
}

// TestFormatUserLogsStripsQuotaSaturation verifies the admin-only quota
// saturation marker (nested under other.admin_info) is removed for non-admin
// log views, since formatUserLogs strips the whole admin_info object.
func TestFormatUserLogsStripsQuotaSaturation(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"model_price": 0.004,
		"admin_info": map[string]interface{}{
			"quota_saturation": map[string]interface{}{
				"op":      "QuotaFromDecimal",
				"kind":    "overflow",
				"clamped": common.MaxQuota,
			},
		},
	})
	logs := []*Log{{Other: other}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	_, hasAdminInfo := parsed["admin_info"]
	require.False(t, hasAdminInfo, "admin_info (and nested quota_saturation) must be stripped for non-admin views")
	// Non-admin billing fields remain visible.
	require.Contains(t, parsed, "model_price")
}
