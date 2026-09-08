package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateGroupSettingsRenamesLiveReferences(t *testing.T) {
	previousOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	t.Cleanup(func() { common.OptionMap = previousOptionMap })
	db := useFrontendOptionMigrationDB(t)
	require.NoError(t, db.AutoMigrate(&User{}, &Token{}, &Channel{}, &Ability{}, &SubscriptionPlan{}, &UserSubscription{}))
	require.NoError(t, db.Create(&[]Option{
		{Key: "GroupRatio", Value: `{"old":1,"other":1}`},
		{Key: "TopupGroupRatio", Value: `{"old":1}`},
		{Key: "UserUsableGroups", Value: `{"old":"old"}`},
		{Key: "GroupGroupRatio", Value: `{"old":{"old":1}}`},
		{Key: "AutoGroups", Value: `["old"]`},
		{Key: "ModelRequestRateLimitGroup", Value: `{"old":[1,2]}`},
		{Key: "group_ratio_setting.group_special_usable_group", Value: `{"old":{"old":"old","+:old":"add","-:old":"remove"}}`},
	}).Error)
	require.NoError(t, db.Create(&User{Id: 1001, Group: "old"}).Error)
	require.NoError(t, db.Create(&Token{Id: 1001, UserId: 1001, Group: "old", Key: "sk-group-rename"}).Error)
	require.NoError(t, db.Create(&Channel{Id: 1001, Group: "old,other", Models: "gpt-4", Status: 1}).Error)
	require.NoError(t, db.Create(&Channel{Id: 1002, Group: "older", Models: "gpt-4", Status: 1}).Error)
	require.NoError(t, db.Create(&SubscriptionPlan{Id: 1001, UpgradeGroup: "old", DowngradeGroup: "old"}).Error)
	require.NoError(t, db.Create(&UserSubscription{Id: 1001, UpgradeGroup: "old", PrevUserGroup: "old", DowngradeGroup: "old"}).Error)

	result, err := UpdateGroupSettings(GroupSettingsRequest{
		Options: map[string]string{"GroupRatio": `{"new":1,"other":1}`},
		Renames: []GroupRename{{From: "old", To: "new"}},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Users)
	assert.Equal(t, 1, result.Tokens)
	assert.Equal(t, 1, result.Channels)

	var user User
	require.NoError(t, db.First(&user, 1001).Error)
	assert.Equal(t, "new", user.Group)
	var token Token
	require.NoError(t, db.First(&token, 1001).Error)
	assert.Equal(t, "new", token.Group)
	var channel Channel
	require.NoError(t, db.First(&channel, 1001).Error)
	assert.Equal(t, "new,other", channel.Group)
	var unchangedChannel Channel
	require.NoError(t, db.First(&unchangedChannel, 1002).Error)
	assert.Equal(t, "older", unchangedChannel.Group)
	var ability Ability
	require.NoError(t, db.Where("channel_id = ?", 1001).First(&ability).Error)
	assert.Equal(t, "new", ability.Group)
	for key, expected := range map[string]string{
		"TopupGroupRatio":            `{"new":1}`,
		"UserUsableGroups":           `{"new":"old"}`,
		"GroupGroupRatio":            `{"new":{"new":1}}`,
		"AutoGroups":                 `["new"]`,
		"ModelRequestRateLimitGroup": `{"new":[1,2]}`,
		"group_ratio_setting.group_special_usable_group": `{"new":{"new":"old","+:new":"add","-:new":"remove"}}`,
	} {
		assert.JSONEq(t, expected, requireOptionValue(t, db, key))
	}
	var plan SubscriptionPlan
	require.NoError(t, db.First(&plan, 1001).Error)
	assert.Equal(t, "new", plan.UpgradeGroup)
	assert.Equal(t, "new", plan.DowngradeGroup)
	var subscription UserSubscription
	require.NoError(t, db.First(&subscription, 1001).Error)
	assert.Equal(t, "new", subscription.UpgradeGroup)
	assert.Equal(t, "new", subscription.PrevUserGroup)
	assert.Equal(t, "new", subscription.DowngradeGroup)
}

func TestUpdateGroupSettingsAllowsUnpairedDeletionWithoutReferences(t *testing.T) {
	previousOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	t.Cleanup(func() { common.OptionMap = previousOptionMap })
	db := useFrontendOptionMigrationDB(t)
	require.NoError(t, db.AutoMigrate(&User{}, &Token{}, &Channel{}, &Ability{}, &SubscriptionPlan{}, &UserSubscription{}))
	require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: `{"old":1}`}).Error)
	_, err := UpdateGroupSettings(GroupSettingsRequest{Options: map[string]string{"GroupRatio": `{}`}})
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, requireOptionValue(t, db, "GroupRatio"))
}

func TestUpdateGroupSettingsRejectsDeletionWithLiveReferences(t *testing.T) {
	db := useFrontendOptionMigrationDB(t)
	require.NoError(t, db.AutoMigrate(&User{}, &Token{}, &Channel{}, &Ability{}, &SubscriptionPlan{}, &UserSubscription{}))
	require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: `{"old":1}`}).Error)
	require.NoError(t, db.Create(&Channel{Id: 1001, Group: "old", Models: "gpt-4", Status: 1}).Error)
	_, err := UpdateGroupSettings(GroupSettingsRequest{Options: map[string]string{"GroupRatio": `{}`}})
	require.EqualError(t, err, "group deletion requires explicit rename: old")
	assert.JSONEq(t, `{"old":1}`, requireOptionValue(t, db, "GroupRatio"))
}

func TestUpdateGroupSettingsRejectsInvalidRename(t *testing.T) {
	db := useFrontendOptionMigrationDB(t)
	require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: `{"old":1}`}).Error)
	for _, rename := range []GroupRename{{From: "auto", To: "new"}, {From: "old,new", To: "new"}, {From: " old", To: "new"}, {From: strings.Repeat("x", 65), To: "new"}} {
		_, err := UpdateGroupSettings(GroupSettingsRequest{Options: map[string]string{"GroupRatio": `{"new":1}`}, Renames: []GroupRename{rename}})
		require.Error(t, err)
	}
}
func TestUpdateGroupSettingsRejectsMalformedOptionBeforeWrite(t *testing.T) {
	db := useFrontendOptionMigrationDB(t)
	require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: `{"old":1}`}).Error)
	_, err := UpdateGroupSettings(GroupSettingsRequest{Options: map[string]string{"GroupRatio": `[]`}})
	require.Error(t, err)
	assert.JSONEq(t, `{"old":1}`, requireOptionValue(t, db, "GroupRatio"))
}
func TestUpdateGroupSettingsRejectsConflictingRename(t *testing.T) {
	db := useFrontendOptionMigrationDB(t)
	require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: `{"old":1,"new":1}`}).Error)
	_, err := UpdateGroupSettings(GroupSettingsRequest{
		Options: map[string]string{"GroupRatio": `{"new":1}`},
		Renames: []GroupRename{{From: "old", To: "new"}},
	})
	require.Error(t, err)
}

func useInitialGroupSettingsDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousOptions := common.OptionMap
	previousRatios := ratio_setting.GroupRatio2JSONString()
	previousRedis, previousMemory := common.RedisEnabled, common.MemoryCacheEnabled
	t.Cleanup(func() {
		common.OptionMap = previousOptions
		common.RedisEnabled, common.MemoryCacheEnabled = previousRedis, previousMemory
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousRatios))
		initCol()
	})
	common.OptionMap = make(map[string]string)
	common.RedisEnabled, common.MemoryCacheEnabled = false, false
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1}`))
	db := useFrontendOptionMigrationDB(t)
	initCol()
	connection, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })
	require.NoError(t, db.AutoMigrate(&User{}, &Token{}, &Channel{}, &Ability{}, &SubscriptionPlan{}, &UserSubscription{}))
	return db
}

func TestUpdateGroupSettingsFirstSavePersistsAndCanBeUpdated(t *testing.T) {
	db := useInitialGroupSettingsDB(t)
	require.NoError(t, db.Create(&User{Id: 1001, Username: "initial-user", Group: "default"}).Error)
	for _, value := range []string{`{"default":1.25,"vip":0.8}`, `{"default":2,"vip":1}`} {
		_, err := UpdateGroupSettings(GroupSettingsRequest{Options: map[string]string{"GroupRatio": value}})
		require.NoError(t, err)
		assert.JSONEq(t, value, requireOptionValue(t, db, "GroupRatio"))
		assert.JSONEq(t, value, ratio_setting.GroupRatio2JSONString())
	}
	var count int64
	require.NoError(t, db.Model(&Option{}).Where(&Option{Key: "GroupRatio"}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
	var user User
	require.NoError(t, db.First(&user, 1001).Error)
	assert.Equal(t, "default", user.Group)
}

func TestUpdateGroupSettingsInitialStatePreservesReferencesAndStoredValues(t *testing.T) {
	for _, test := range []struct {
		name      string
		stored    *string
		userGroup string
		submitted string
		renames   []GroupRename
		wantError bool
		wantGroup string
	}{
		{name: "missing row cannot delete a referenced default group", userGroup: "default", submitted: `{"vip":1}`, wantError: true, wantGroup: "default"},
		{name: "missing row can rename a default group", userGroup: "default", submitted: `{"primary":1,"vip":1}`, renames: []GroupRename{{From: "default", To: "primary"}}, wantGroup: "primary"},
		{name: "stored empty string is not a missing row", stored: common.GetPointer(""), userGroup: "default", submitted: `{"default":2,"vip":1}`, wantError: true, wantGroup: "default"},
		{name: "stored malformed JSON is not replaced", stored: common.GetPointer(`{invalid}`), userGroup: "default", submitted: `{"default":2,"vip":1}`, wantError: true, wantGroup: "default"},
		{name: "stored empty map does not regain runtime groups", stored: common.GetPointer(`{}`), userGroup: "default", submitted: `{"primary":1,"vip":1}`, renames: []GroupRename{{From: "default", To: "primary"}}, wantError: true, wantGroup: "default"},
		{name: "stored groups override runtime defaults", stored: common.GetPointer(`{"saved":1}`), userGroup: "saved", submitted: `{"primary":1}`, renames: []GroupRename{{From: "saved", To: "primary"}}, wantGroup: "primary"},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := useInitialGroupSettingsDB(t)
			if test.stored != nil {
				require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: *test.stored}).Error)
			}
			require.NoError(t, db.Create(&User{Id: 1001, Username: "initial-user", Group: test.userGroup}).Error)
			result, err := UpdateGroupSettings(GroupSettingsRequest{
				Options: map[string]string{"GroupRatio": test.submitted},
				Renames: test.renames,
			})
			if test.wantError {
				require.Error(t, err)
				if test.stored == nil {
					requireOptionMissing(t, db, "GroupRatio")
				} else {
					assert.Equal(t, *test.stored, requireOptionValue(t, db, "GroupRatio"))
				}
				assert.JSONEq(t, `{"default":1,"vip":1}`, ratio_setting.GroupRatio2JSONString())
			} else {
				require.NoError(t, err)
				assert.Equal(t, 1, result.Users)
				assert.JSONEq(t, test.submitted, requireOptionValue(t, db, "GroupRatio"))
			}
			var user User
			require.NoError(t, db.First(&user, 1001).Error)
			assert.Equal(t, test.wantGroup, user.Group)
		})
	}
}
