package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserGroupsIncludesAutoOnlyWhenEnabled(t *testing.T) {
	setupTokenAutoGroupsControllerTest(t)
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalRatios := ratio_setting.GroupRatio2JSONString()
	originalEnabled := setting.AutoGroupEnabled
	originalDefault := setting.DefaultUseAutoGroup
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
		setting.AutoGroupEnabled = originalEnabled
		setting.DefaultUseAutoGroup = originalDefault
	})

	getGroups := func() map[string]map[string]interface{} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self/groups", nil)
		ctx.Set("id", 101)

		GetUserGroups(ctx)

		require.Equal(t, http.StatusOK, recorder.Code)
		var body struct {
			Success bool                              `json:"success"`
			Data    map[string]map[string]interface{} `json:"data"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
		assert.True(t, body.Success)
		return body.Data
	}

	setting.DefaultUseAutoGroup = true
	setting.AutoGroupEnabled = false
	assert.NotContains(t, getGroups(), "auto")

	setting.DefaultUseAutoGroup = false
	setting.AutoGroupEnabled = true
	groups := getGroups()
	require.Contains(t, groups, "auto")
	assert.Equal(t, "自动", groups["auto"]["ratio"])
	assert.NotEmpty(t, groups["auto"]["desc"])
}
