package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type listModelsResponse struct {
	Success bool               `json:"success"`
	Data    []dto.OpenAIModels `json:"data"`
	Object  string             `json:"object"`
}

type userModelsResponse struct {
	Success bool     `json:"success"`
	Data    []string `json:"data"`
}

func setupModelListControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	initModelListColumnNames(t)

	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func initModelListColumnNames(t *testing.T) {
	t.Helper()

	originalIsMasterNode := common.IsMasterNode
	originalSQLitePath := common.SQLitePath
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")
	defer func() {
		common.IsMasterNode = originalIsMasterNode
		common.SQLitePath = originalSQLitePath
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	}()

	common.IsMasterNode = false
	common.SQLitePath = fmt.Sprintf("file:%s_init?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, os.Setenv("SQL_DSN", "local"))

	require.NoError(t, model.InitDB())
	if model.DB != nil {
		sqlDB, err := model.DB.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}
}

func withTieredBillingConfig(t *testing.T, modes map[string]string, exprs map[string]string) {
	t.Helper()

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		if strings.HasPrefix(key, "billing_setting.") {
			saved[key] = value
		}
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
		model.InvalidatePricingCache()
	})

	modeBytes, err := common.Marshal(modes)
	require.NoError(t, err)
	exprBytes, err := common.Marshal(exprs)
	require.NoError(t, err)

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": string(modeBytes),
		"billing_setting.billing_expr": string(exprBytes),
	}))
	model.InvalidatePricingCache()
}

func withSelfUseModeDisabled(t *testing.T) {
	t.Helper()

	original := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = false
	t.Cleanup(func() {
		operation_setting.SelfUseModeEnabled = original
	})
}

func withSelfUseModeEnabled(t *testing.T) {
	t.Helper()

	original := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = true
	t.Cleanup(func() {
		operation_setting.SelfUseModeEnabled = original
	})
}

func decodeListModelsPayload(t *testing.T, recorder *httptest.ResponseRecorder) listModelsResponse {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload listModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, "list", payload.Object)
	return payload
}

func decodeListModelsResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]struct{} {
	t.Helper()

	payload := decodeListModelsPayload(t, recorder)
	ids := make(map[string]struct{}, len(payload.Data))
	for _, item := range payload.Data {
		ids[item.Id] = struct{}{}
	}
	return ids
}

func pricingByModelName(pricings []model.Pricing) map[string]model.Pricing {
	byName := make(map[string]model.Pricing, len(pricings))
	for _, pricing := range pricings {
		byName[pricing.ModelName] = pricing
	}
	return byName
}

func decodeUserModelsResponse(t *testing.T, recorder *httptest.ResponseRecorder) []string {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload userModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	return payload.Data
}

func TestGetUserModelsFiltersByRequestedGroup(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1002,
		Username: "playground-model-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "zz-default-only-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-disabled-model", ChannelId: 1, Enabled: false},
	}).Error)

	defaultRecorder := httptest.NewRecorder()
	defaultContext, _ := gin.CreateTestContext(defaultRecorder)
	defaultContext.Request = httptest.NewRequest(http.MethodGet, "/api/user/models?group=default", nil)
	defaultContext.Set("id", 1002)

	GetUserModels(defaultContext)

	defaultModels := decodeUserModelsResponse(t, defaultRecorder)
	require.ElementsMatch(t, []string{"zz-default-only-model"}, defaultModels)

	vipRecorder := httptest.NewRecorder()
	vipContext, _ := gin.CreateTestContext(vipRecorder)
	vipContext.Request = httptest.NewRequest(http.MethodGet, "/api/user/models?group=vip", nil)
	vipContext.Set("id", 1002)

	GetUserModels(vipContext)

	require.Empty(t, decodeUserModelsResponse(t, vipRecorder))
}

func TestGetUserModelsExpandsAutoGroupsInConfiguredOrder(t *testing.T) {
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalRatios := ratio_setting.GroupRatio2JSONString()
	originalEnabled := setting.AutoGroupEnabled
	originalSpecialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
		setting.AutoGroupEnabled = originalEnabled
		specialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup
		specialGroups.Clear()
		specialGroups.AddAll(originalSpecialGroups)
	})

	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["vip","default","unavailable"]`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2,"unavailable":1,"manual":1}`))
	specialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup
	specialGroups.Clear()
	specialGroups.Set("default", map[string]string{
		"+:vip":         "VIP 分组",
		"-:unavailable": "",
	})

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1003,
		Username: "playground-auto-model-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "vip", Model: "zz-vip-model", ChannelId: 1, Enabled: true},
		{Group: "vip", Model: "zz-shared-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-default-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-shared-model", ChannelId: 2, Enabled: true},
		{Group: "unavailable", Model: "zz-unavailable-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-disabled-model", ChannelId: 1, Enabled: false},
		{Group: "manual", Model: "zz-manual-model", ChannelId: 1, Enabled: true},
	}).Error)

	for _, test := range []struct {
		name         string
		enabled      bool
		usableGroups string
	}{
		{
			name:         "enabled without a literal auto group",
			enabled:      true,
			usableGroups: `{"default":"Default","unavailable":"Unavailable","manual":"Manual"}`,
		},
		{
			name:         "enabled with a legacy auto group",
			enabled:      true,
			usableGroups: `{"auto":"Auto","default":"Default","unavailable":"Unavailable","manual":"Manual"}`,
		},
		{
			name:         "disabled with a legacy auto group",
			enabled:      false,
			usableGroups: `{"auto":"Auto","default":"Default","unavailable":"Unavailable","manual":"Manual"}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			setting.AutoGroupEnabled = test.enabled
			require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(test.usableGroups))
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, "/api/user/models?group=auto", nil)
			context.Set("id", 1003)

			GetUserModels(context)

			models := decodeUserModelsResponse(t, recorder)
			if !test.enabled {
				assert.Empty(t, models)
				return
			}
			require.Len(t, models, 3)
			assert.ElementsMatch(t, []string{"zz-vip-model", "zz-shared-model"}, models[:2])
			assert.Equal(t, "zz-default-model", models[2])
		})
	}
}

func TestListModelsIncludesTieredBillingModel(t *testing.T) {
	withSelfUseModeDisabled(t)
	withTieredBillingConfig(t, map[string]string{
		"zz-tiered-visible-model":      "tiered_expr",
		"zz-tiered-empty-expr-model":   "tiered_expr",
		"zz-tiered-missing-expr-model": "tiered_expr",
	}, map[string]string{
		"zz-tiered-visible-model":    `tier("base", p * 1 + c * 2)`,
		"zz-tiered-empty-expr-model": "   ",
	})

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1001,
		Username: "model-list-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "zz-tiered-visible-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-tiered-empty-expr-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-tiered-missing-expr-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-unpriced-model", ChannelId: 1, Enabled: true},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	ctx.Set("id", 1001)

	ListModels(ctx, constant.ChannelTypeOpenAI)

	ids := decodeListModelsResponse(t, recorder)
	require.Contains(t, ids, "zz-tiered-visible-model")
	require.NotContains(t, ids, "zz-tiered-empty-expr-model")
	require.NotContains(t, ids, "zz-tiered-missing-expr-model")
	require.NotContains(t, ids, "zz-unpriced-model")

	pricingByName := pricingByModelName(model.GetPricing())
	visiblePricing, ok := pricingByName["zz-tiered-visible-model"]
	require.True(t, ok)
	require.Equal(t, "tiered_expr", visiblePricing.BillingMode)
	require.NotEmpty(t, visiblePricing.BillingExpr)

	emptyExprPricing, ok := pricingByName["zz-tiered-empty-expr-model"]
	require.True(t, ok)
	require.Empty(t, emptyExprPricing.BillingMode)
	require.Empty(t, emptyExprPricing.BillingExpr)

	missingExprPricing, ok := pricingByName["zz-tiered-missing-expr-model"]
	require.True(t, ok)
	require.Empty(t, missingExprPricing.BillingMode)
	require.Empty(t, missingExprPricing.BillingExpr)
}

func TestListModelsUsesAdvancedCustomEndpointTypesFromPricingCache(t *testing.T) {
	withSelfUseModeEnabled(t)
	db := setupModelListControllerTestDB(t)

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		model.InvalidatePricingCache()
	})

	require.NoError(t, db.Create(&model.User{
		Id:       1003,
		Username: "advanced-custom-model-list-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)

	channel := &model.Channel{
		Id:     701,
		Type:   constant.ChannelTypeAdvancedCustom,
		Key:    "advanced-custom-key",
		Status: common.ChannelStatusEnabled,
		Name:   "advanced-custom-channel",
		Group:  "default",
		Models: "gemini-3.5-flash",
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		AdvancedCustom: &dto.AdvancedCustomConfig{
			Routes: []dto.AdvancedCustomRoute{
				{
					IncomingPath: "/v1/chat/completions",
					UpstreamPath: "/v1/chat/completions",
				},
				{
					IncomingPath: "/v1/responses",
					UpstreamPath: "/v1beta/models/{model}:generateContent",
					Converter:    "openai_responses_to_gemini_generate_content",
					Models:       []string{"re:^gemini-"},
				},
			},
		},
	})
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "gemini-3.5-flash",
		ChannelId: 701,
		Enabled:   true,
	}).Error)

	model.InitChannelCache()
	model.GetPricing()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	ctx.Set("id", 1003)

	ListModels(ctx, constant.ChannelTypeOpenAI)

	payload := decodeListModelsPayload(t, recorder)
	require.Len(t, payload.Data, 1)
	require.Equal(t, "gemini-3.5-flash", payload.Data[0].Id)
	require.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
	}, payload.Data[0].SupportedEndpointTypes)
}

func TestListModelsTokenLimitIncludesTieredBillingModel(t *testing.T) {
	withSelfUseModeDisabled(t)
	withTieredBillingConfig(t, map[string]string{
		"zz-token-tiered-visible-model":      "tiered_expr",
		"zz-token-tiered-empty-expr-model":   "tiered_expr",
		"zz-token-tiered-missing-expr-model": "tiered_expr",
	}, map[string]string{
		"zz-token-tiered-visible-model":    `tier("base", p * 1 + c * 2)`,
		"zz-token-tiered-empty-expr-model": "",
	})
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "zz-token-tiered-visible-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-token-tiered-empty-expr-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-token-tiered-missing-expr-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-token-unpriced-model", ChannelId: 1, Enabled: true},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, map[string]bool{
		"zz-token-tiered-visible-model":      true,
		"zz-token-tiered-empty-expr-model":   true,
		"zz-token-tiered-missing-expr-model": true,
		"zz-token-unpriced-model":            true,
	})

	ListModels(ctx, constant.ChannelTypeOpenAI)

	ids := decodeListModelsResponse(t, recorder)
	require.Contains(t, ids, "zz-token-tiered-visible-model")
	require.NotContains(t, ids, "zz-token-tiered-empty-expr-model")
	require.NotContains(t, ids, "zz-token-tiered-missing-expr-model")
	require.NotContains(t, ids, "zz-token-unpriced-model")
}

func TestListModelsTokenLimitUsesResolvedCustomAutoGroups(t *testing.T) {
	withSelfUseModeEnabled(t)
	originalMax := setting.GetMaxTokenAutoGroups()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalRatios := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, setting.UpdateMaxTokenAutoGroups("5"))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1}`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateMaxTokenAutoGroups(fmt.Sprintf("%d", originalMax)))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
	})

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "vip", Model: "zz-vip-allowed", ChannelId: 1, Enabled: true},
		{Group: "vip", Model: "zz-vip-denied", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-default-outside-snapshot", ChannelId: 1, Enabled: true},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "auto")
	common.SetContextKey(ctx, constant.ContextKeyTokenAutoGroups, []string{"vip"})
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, map[string]bool{
		"zz-vip-allowed":              true,
		"zz-default-outside-snapshot": true,
		"zz-not-enabled":              true,
	})

	ListModels(ctx, constant.ChannelTypeOpenAI)
	ids := decodeListModelsResponse(t, recorder)
	require.Equal(t, map[string]struct{}{"zz-vip-allowed": {}}, ids)

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	emptyRecorder := httptest.NewRecorder()
	emptyCtx, _ := gin.CreateTestContext(emptyRecorder)
	emptyCtx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	common.SetContextKey(emptyCtx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(emptyCtx, constant.ContextKeyTokenGroup, "auto")
	common.SetContextKey(emptyCtx, constant.ContextKeyTokenAutoGroups, []string{"vip"})
	common.SetContextKey(emptyCtx, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(emptyCtx, constant.ContextKeyTokenModelLimit, map[string]bool{"zz-vip-allowed": true})

	require.NotPanics(t, func() {
		ListModels(emptyCtx, constant.ChannelTypeAnthropic)
	})
	var anthropicResponse struct {
		Data    []dto.AnthropicModel `json:"data"`
		FirstID string               `json:"first_id"`
		LastID  string               `json:"last_id"`
	}
	require.NoError(t, common.Unmarshal(emptyRecorder.Body.Bytes(), &anthropicResponse))
	require.Empty(t, anthropicResponse.Data)
	require.Empty(t, anthropicResponse.FirstID)
	require.Empty(t, anthropicResponse.LastID)
}

func TestSetupLoginDoesNotTouchPasswordWhenPasswordFieldOmitted(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.AuditLog{}, &model.UserSession{}, &model.TwoFA{}, &model.PasskeyCredential{}))

	hashedPassword, err := common.Password2Hash("CurrentPassword123")
	require.NoError(t, err)
	user := &model.User{
		Username: "twofa-user",
		Password: hashedPassword,
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, db.Create(user).Error)

	router := gin.New()
	router.GET("/", func(c *gin.Context) {
		setupLogin(&model.User{
			Id:          user.Id,
			AuthVersion: user.AuthVersion,
			Username:    user.Username,
			Role:        user.Role,
			Status:      user.Status,
			Group:       user.Group,
		}, nil, c)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var stored model.User
	require.NoError(t, db.First(&stored, user.Id).Error)
	assert.Equal(t, hashedPassword, stored.Password)
}

type modelCapabilitiesTransport func(*http.Request) (*http.Response, error)

func (transport modelCapabilitiesTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestListModelsCodexPreservesModelAvailability(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	require.NoError(t, i18n.Init())
	withSelfUseModeEnabled(t)
	originalCatalog := modelsDevCatalog
	modelsDevCatalog = &modelsDevCapabilityCache{}
	t.Cleanup(func() { modelsDevCatalog = originalCatalog })

	var fetches, sourceMode atomic.Int32
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		if r.URL.Path != "/api.json" {
			http.NotFound(w, r)
			return
		}
		switch sourceMode.Load() {
		case 0:
			w.WriteHeader(http.StatusServiceUnavailable)
		case 1:
			_, _ = w.Write([]byte(`{"broken":`))
		default:
			_, _ = w.Write([]byte(`{
				"a-reseller":{"models":{
					"gpt-5.2-codex":{"id":"gpt-5.2-codex","canonical_model_id":"openai/gpt-5.2-codex","name":"Reseller Codex","limit":{"context":1000}},
					"deployment-id":{"id":"deployment-id","canonical_model_id":"openai/canonical-only","name":"Canonical only","reasoning":true,"reasoning_options":[{"type":"budget_tokens","min":1024}],"tool_call":true,"modalities":{"input":["text","audio","video"]},"limit":{"input":2048}}
				}},
				"openai":{"models":{
					"gpt-5.2-codex":{"id":"gpt-5.2-codex","canonical_model_id":"openai/gpt-5.2-codex","name":"GPT-5.2 Codex","description":"Coding model","reasoning":true,"reasoning_options":[{"type":"effort","values":["low","medium","high","xhigh"]}],"tool_call":true,"modalities":{"input":["text","image","pdf"]},"limit":{"context":400000,"input":272000}},
					"zz-plain-chat":{"id":"zz-plain-chat","name":"Plain Chat","reasoning":false,"tool_call":false,"modalities":{"input":["text"]},"limit":{"context":8192}}
				}}
			}`))
		}
	}))
	t.Cleanup(source.Close)
	client := getHTTPClient()
	originalTransport := client.Transport
	client.Transport = modelCapabilitiesTransport(func(request *http.Request) (*http.Response, error) {
		if request.URL.Scheme != "https" || request.URL.Host != "models.dev" {
			return nil, fmt.Errorf("unexpected model capability URL: %s", request.URL)
		}
		clone := request.Clone(request.Context())
		clone.URL.Scheme = "http"
		clone.URL.Host = strings.TrimPrefix(source.URL, "http://")
		return http.DefaultTransport.RoundTrip(clone)
	})
	t.Cleanup(func() { client.Transport = originalTransport })

	user := model.User{Username: "codex-models-user", Status: common.UserStatusEnabled, Group: "default", Quota: 100}
	require.NoError(t, model.DB.Create(&user).Error)
	require.NoError(t, model.DB.Create(&[]model.Ability{
		{Group: "default", Model: "gpt-5.2-codex", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "openai/gpt-5.2-codex", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "canonical-only", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-plain-chat", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-custom-coding", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-custom-coding", ChannelId: 2, Enabled: true},
		{Group: "default", Model: "zz-token-denied", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-disabled", ChannelId: 1, Enabled: false},
		{Group: "vip", Model: "zz-other-group", ChannelId: 1, Enabled: true},
	}).Error)
	require.NoError(t, model.DB.Create(&[]model.Token{
		{
			UserId: user.Id, Key: "codexmodelskey", Status: common.TokenStatusEnabled,
			ExpiredTime: -1, UnlimitedQuota: true, ModelLimitsEnabled: true,
			ModelLimits: "gpt-5.2-codex,openai/gpt-5.2-codex,canonical-only,zz-plain-chat,zz-custom-coding,zz-disabled,zz-other-group,zz-missing",
		},
		{
			UserId: user.Id, Key: "codexemptymodelskey", Status: common.TokenStatusEnabled,
			ExpiredTime: -1, UnlimitedQuota: true, ModelLimitsEnabled: true, ModelLimits: "zz-missing",
		},
	}).Error)

	engine := gin.New()
	engine.GET("/v1/models", middleware.TokenAuth(), func(c *gin.Context) {
		ListModels(c, constant.ChannelTypeCodex)
	})
	// Transport and parse failures must not poison the permanent cache.
	for mode := range 2 {
		sourceMode.Store(int32(mode))
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/models?client_version=0.165.0", nil)
		request.Header.Set("Authorization", "Bearer codexmodelskey")
		request.Header.Set("Accept-Language", "en")
		engine.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusBadGateway, recorder.Code)
		var payload struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
		assert.Equal(t, "model_capabilities_unavailable", payload.Error.Code)
		assert.Contains(t, payload.Error.Message, "Failed to load model capabilities")
	}
	sourceMode.Store(2)
	var callers sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		callers.Go(func() {
			_, err := getModelsDevCapabilities(context.Background())
			results <- err
		})
	}
	callers.Wait()
	for range 2 {
		require.NoError(t, <-results)
	}
	require.Equal(t, int32(3), fetches.Load(), "concurrent callers share one successful catalog fetch")
	source.Close()

	for _, test := range []struct {
		name string
		key  string
		want []string
	}{
		{name: "cached models with upstream offline", key: "codexmodelskey", want: []string{"gpt-5.2-codex", "openai/gpt-5.2-codex", "canonical-only", "zz-plain-chat", "zz-custom-coding"}},
		{name: "empty catalog", key: "codexemptymodelskey", want: []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/v1/models?client_version=0.165.0", nil)
			request.Header.Set("Authorization", "Bearer "+test.key)
			engine.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code)
			var payload struct {
				Models []map[string]any `json:"models"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
			require.NotNil(t, payload.Models, "Codex requires an array, including for an empty catalog")
			slugs := make([]string, 0, len(payload.Models))
			for _, item := range payload.Models {
				slug, ok := item["slug"].(string)
				require.True(t, ok)
				slugs = append(slugs, slug)
				assert.Equal(t, "list", item["visibility"])
				assert.Equal(t, true, item["supported_in_api"])
				assert.NotEmpty(t, item["base_instructions"])
				switch slug {
				case "gpt-5.2-codex", "openai/gpt-5.2-codex":
					assert.Equal(t, "GPT-5.2 Codex", item["display_name"])
					assert.Equal(t, "Coding model", item["description"])
					assert.Equal(t, "medium", item["default_reasoning_level"])
					assert.Equal(t, float64(272000), item["context_window"])
					assert.Equal(t, []any{"text", "image"}, item["input_modalities"])
					assert.Equal(t, "unified_exec", item["shell_type"])
					levels, ok := item["supported_reasoning_levels"].([]any)
					require.True(t, ok)
					efforts := make([]any, 0, len(levels))
					for _, level := range levels {
						preset, ok := level.(map[string]any)
						require.True(t, ok)
						efforts = append(efforts, preset["effort"])
					}
					assert.Equal(t, []any{"low", "medium", "high", "xhigh"}, efforts)
				case "canonical-only":
					assert.Equal(t, "Canonical only", item["display_name"])
					assert.Equal(t, float64(2048), item["context_window"])
					assert.Equal(t, []any{"text", "audio"}, item["input_modalities"])
					assert.Equal(t, []any{}, item["supported_reasoning_levels"], "token budgets are not effort levels")
				case "zz-plain-chat":
					assert.Equal(t, "disabled", item["shell_type"])
					assert.Equal(t, float64(8192), item["context_window"])
					assert.Equal(t, []any{}, item["supported_reasoning_levels"])
				case "zz-custom-coding":
					assert.Equal(t, slug, item["display_name"])
					assert.Equal(t, []any{}, item["supported_reasoning_levels"])
					assert.Equal(t, []any{"text"}, item["input_modalities"])
					assert.NotContains(t, item, "context_window")
					assert.NotContains(t, item, "default_reasoning_level")
				}
				for _, capability := range []string{
					"supports_reasoning_summaries", "supports_reasoning_summary_parameter",
					"support_verbosity", "supports_parallel_tool_calls",
				} {
					assert.Equal(t, false, item[capability], "models.dev does not advertise %s", capability)
				}
			}
			assert.ElementsMatch(t, test.want, slugs)
		})
	}
	assert.Equal(t, int32(3), fetches.Load(), "a successful catalog has no refresh or expiry")
}
