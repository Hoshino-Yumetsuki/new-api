package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func epayChannelDatabase(t *testing.T, kind, dsn string) *gorm.DB {
	t.Helper()
	db := modelManagementDB(t, kind, dsn)
	require.NoError(t, db.AutoMigrate(&model.TopUp{}, &model.SubscriptionPlan{}, &model.SubscriptionOrder{}, &model.UserSubscription{}, &model.Log{}))
	oldConfig := config.GlobalConfig.ExportAllConfigs()
	oldMethods, oldPrice, oldMin, oldCallback := operation_setting.PayMethods, operation_setting.Price, operation_setting.MinTopUp, operation_setting.CustomCallbackAddress
	oldAddress, oldID, oldKey := operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey
	oldQuota := common.QuotaPerUnit
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(oldConfig))
		operation_setting.PayMethods, operation_setting.Price, operation_setting.MinTopUp, operation_setting.CustomCallbackAddress = oldMethods, oldPrice, oldMin, oldCallback
		operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey = oldAddress, oldID, oldKey
		common.QuotaPerUnit = oldQuota
	})
	common.QuotaPerUnit = 500000
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	operation_setting.GetPaymentSetting().EpayChannels = nil
	operation_setting.GetPaymentSetting().AmountDiscount = map[int]float64{}
	operation_setting.PayMethods = nil
	operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey = "", "", ""
	operation_setting.Price, operation_setting.MinTopUp = 1, 1
	operation_setting.CustomCallbackAddress = "https://gateway.example.com"
	confirmPaymentComplianceForTest(t)
	return db
}

func epayChannelRequest(t *testing.T, handler gin.HandlerFunc, body any) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := common.Marshal(body)
	require.NoError(t, err)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(encoded))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 1)
	ctx.Set("role", common.RoleRootUser)
	handler(ctx)
	return response
}

func epayChannelCallback(t *testing.T, handler gin.HandlerFunc, httpMethod, tradeNo, pid, money, method, key string) *httptest.ResponseRecorder {
	t.Helper()
	params := epay.GenerateParams(map[string]string{
		"pid": pid, "out_trade_no": tradeNo, "trade_no": "provider-" + tradeNo,
		"type": method, "money": money, "trade_status": epay.StatusTradeSuccess,
	}, key)
	values := url.Values{}
	for name, value := range params {
		values.Set(name, value)
	}
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	if httpMethod == http.MethodPost {
		ctx.Request = httptest.NewRequest(httpMethod, "/notify", strings.NewReader(values.Encode()))
		ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		ctx.Request = httptest.NewRequest(httpMethod, "/notify?"+values.Encode(), nil)
	}
	handler(ctx)
	ctx.Writer.WriteHeaderNow()
	return response
}

func TestEpayChannelsDatabaseMatrix(t *testing.T) {
	for _, dialect := range []struct{ kind, env string }{{"sqlite", ""}, {"mysql", "TEST_MYSQL_DSN"}, {"postgres", "TEST_POSTGRES_DSN"}} {
		t.Run(dialect.kind, func(t *testing.T) {
			if dialect.env != "" && os.Getenv(dialect.env) == "" {
				t.Skip("set " + dialect.env + " to exercise this database")
			}
			t.Run("configuration and payment isolation", func(t *testing.T) {
				db := epayChannelDatabase(t, dialect.kind, os.Getenv(dialect.env))
				// Checkout formats this half-cent amount as 2.67, not decimal Round(2)'s 2.68.
				operation_setting.Price = 1.3375
				channels := []operation_setting.EpayChannel{
					{Name: "achannel", PayAddress: "https://a.example.com", EpayId: "1001", EpayKey: "key-for-a"},
					{Name: "bchannel", PayAddress: "https://b.example.com", EpayId: "1002", EpayKey: "key-for-b"},
				}
				encoded, err := common.Marshal(channels)
				require.NoError(t, err)
				var result struct{ Success bool }
				modelManagementRequest(t, UpdateOption, http.MethodPut, "/api/option/", OptionUpdateRequest{Key: "payment_setting.epay_channels", Value: string(encoded)}, &result)
				require.True(t, result.Success)
				var saved model.Option
				require.NoError(t, db.Where(&model.Option{Key: operation_setting.EpayChannelsStorageKey}).First(&saved).Error)
				originalConfig := saved.Value

				for _, invalid := range []string{"", "with1", "with_space", "with.dot", "中文", strings.Repeat("a", 49)} {
					candidate := []operation_setting.EpayChannel{channels[0]}
					candidate[0].Name = invalid
					payload, marshalErr := common.Marshal(candidate)
					require.NoError(t, marshalErr)
					response := epayChannelRequest(t, UpdateOption, OptionUpdateRequest{Key: "payment_setting.epay_channels", Value: string(payload)})
					assert.Contains(t, response.Body.String(), `"success":false`, invalid)
				}
				for _, invalid := range []string{
					`null`, `{}`, `[`,
					`[{"name":"newchannel","pay_address":"https://c.example.com","epay_id":"1003","epay_key":""}]`,
					`[{"name":"achannel","pay_address":"https://a.example.com","epay_id":"1001","epay_key":""},{"name":"achannel","pay_address":"https://b.example.com","epay_id":"1002","epay_key":"key-for-b"}]`,
				} {
					response := epayChannelRequest(t, UpdateOption, OptionUpdateRequest{Key: "payment_setting.epay_channels", Value: invalid})
					assert.Contains(t, response.Body.String(), `"success":false`, invalid)
				}
				require.NoError(t, db.Where(&model.Option{Key: operation_setting.EpayChannelsStorageKey}).First(&saved).Error)
				assert.Equal(t, originalConfig, saved.Value, "invalid configuration must not replace saved credentials")

				var options struct {
					Success bool
					Data    []model.Option
				}
				response := modelManagementRequest(t, GetOptions, http.MethodGet, "/api/option/", nil, &options)
				require.True(t, options.Success)
				assert.NotContains(t, response.Body.String(), "key-for-a")
				assert.NotContains(t, response.Body.String(), "key-for-b")
				var redacted string
				for _, option := range options.Data {
					if option.Key == "payment_setting.epay_channels" {
						redacted = option.Value
					}
				}
				require.NoError(t, common.UnmarshalJsonStr(redacted, &channels))
				require.Len(t, channels, 2)
				assert.Empty(t, channels[0].EpayKey)
				channels[0].PayAddress = "https://a.example.com/pay"
				encoded, err = common.Marshal(channels)
				require.NoError(t, err)
				modelManagementRequest(t, UpdateOption, http.MethodPut, "/api/option/", OptionUpdateRequest{Key: "payment_setting.epay_channels", Value: string(encoded)}, &result)
				require.True(t, result.Success)
				channels = append(channels, operation_setting.EpayChannel{Name: "default", PayAddress: "https://default.example.com", EpayId: "9001", EpayKey: "key-for-default"})
				encoded, err = common.Marshal(channels)
				require.NoError(t, err)
				require.NoError(t, model.UpdateOption("payment_setting.epay_channels", string(encoded)))
				longType := strings.Repeat("x", 246)
				require.NoError(t, model.UpdateOption("PayMethods", `[{"name":"A","type":"achannel.wxpay"},{"name":"B","type":"bchannel.alipay"},{"name":"Long","type":"achannel.`+longType+`"},{"name":"Long default","type":"default.`+longType+`"}]`))
				var compatibleMethods model.Option
				require.NoError(t, db.Where(&model.Option{Key: "PayMethods"}).First(&compatibleMethods).Error)
				assert.JSONEq(t, "[]", compatibleMethods.Value, "old versions must not offer unsupported channels or types")
				require.NoError(t, db.Create(&model.User{Id: 1, Username: "epay-user", Group: "default", Status: common.UserStatusEnabled}).Error)
				plan := model.SubscriptionPlan{Title: "Epay plan", PriceAmount: 2.675, Currency: "USD", DurationUnit: "month", DurationValue: 1, Enabled: true, TotalAmount: 1000}
				require.NoError(t, db.Create(&plan).Error)
				model.InvalidateSubscriptionPlanCache(plan.Id)
				t.Cleanup(func() { model.InvalidateSubscriptionPlanCache(plan.Id) })

				for _, purchase := range []struct{ method, pid, key, address, actualType, legacyType string }{
					{"achannel.wxpay", "1001", "key-for-a", "https://a.example.com/pay/submit.php", "bank", "bank"},
					{"bchannel.alipay", "1002", "key-for-b", "https://b.example.com/submit.php", "bank", "bank"},
					{"achannel." + longType, "1001", "key-for-a", "https://a.example.com/pay/submit.php", longType, "epay"},
					{"default." + longType, "9001", "key-for-default", "https://default.example.com/submit.php", longType, "epay"},
				} {
					t.Run(purchase.method, func(t *testing.T) {
						for _, subscription := range []bool{false, true} {
							handler := RequestEpay
							body := map[string]any{"amount": 2, "payment_method": purchase.method}
							if subscription {
								handler = SubscriptionRequestEpay
								body["plan_id"] = plan.Id
							}
							paymentResponse := epayChannelRequest(t, handler, body)
							var payment struct {
								Message string
								Data    map[string]string
								URL     string
							}
							require.NoError(t, common.Unmarshal(paymentResponse.Body.Bytes(), &payment), paymentResponse.Body.String())
							require.Equal(t, "success", payment.Message, paymentResponse.Body.String())
							assert.Equal(t, purchase.address, payment.URL)
							_, upstreamType, _ := strings.Cut(purchase.method, ".")
							assert.Equal(t, upstreamType, payment.Data["type"])
							assert.Equal(t, purchase.pid, payment.Data["pid"])
							client, clientErr := epay.NewClient(&epay.Config{PartnerID: purchase.pid, Key: purchase.key}, purchase.address)
							require.NoError(t, clientErr)
							verified, verifyErr := client.Verify(payment.Data)
							require.NoError(t, verifyErr)
							assert.True(t, verified.VerifyStatus, "blank resave must retain the original signing key")
							tradeNo := payment.Data["out_trade_no"]
							assert.Equal(t, "2.67", payment.Data["money"])
							if subscription {
								assert.ErrorIs(t, model.CompleteSubscriptionOrder(tradeNo, "", model.PaymentProviderEpayChannel, "foreign.bank"), model.ErrPaymentMethodMismatch)
								assert.Equal(t, common.TopUpStatusPending, model.GetSubscriptionOrderByTradeNo(tradeNo).Status)
							} else {
								_, settlementErr := model.RechargeEpay(tradeNo, "foreign.bank", "127.0.0.1")
								assert.ErrorIs(t, settlementErr, model.ErrPaymentMethodMismatch)
								assert.Equal(t, common.TopUpStatusPending, model.GetTopUpByTradeNo(tradeNo).Status)
							}
							notify := EpayNotify
							if subscription {
								notify = SubscriptionEpayNotify
							}
							otherKey := "key-for-a"
							if purchase.key == otherKey {
								otherKey = "key-for-b"
							}
							for _, forged := range []struct{ pid, money, key string }{
								{purchase.pid, payment.Data["money"], otherKey},
								{"foreign-merchant", payment.Data["money"], purchase.key},
								{purchase.pid, "0.01", purchase.key},
								{purchase.pid, "1e-999999999", otherKey},
								{purchase.pid, "1e999999999", purchase.key},
								{purchase.pid, strings.Repeat("9", 65), purchase.key},
							} {
								callback := epayChannelCallback(t, notify, http.MethodGet, tradeNo, forged.pid, forged.money, upstreamType, forged.key)
								assert.Equal(t, "fail", callback.Body.String())
								if subscription {
									returned := epayChannelCallback(t, SubscriptionEpayReturn, http.MethodGet, tradeNo, forged.pid, forged.money, upstreamType, forged.key)
									assert.Contains(t, returned.Header().Get("Location"), "pay=fail")
								}
							}
							actualType := purchase.actualType
							if subscription && purchase.pid == "1002" {
								invalidReturn := epayChannelCallback(t, SubscriptionEpayReturn, http.MethodGet, tradeNo, purchase.pid, payment.Data["money"], actualType, otherKey)
								assert.Contains(t, invalidReturn.Header().Get("Location"), "pay=fail")
								returned := epayChannelCallback(t, SubscriptionEpayReturn, http.MethodPost, tradeNo, purchase.pid, payment.Data["money"], actualType, purchase.key)
								require.Equal(t, http.StatusFound, returned.Code)
								require.Contains(t, returned.Header().Get("Location"), "pay=success")
							}
							for _, verb := range []string{http.MethodGet, http.MethodPost} {
								callback := epayChannelCallback(t, notify, verb, tradeNo, purchase.pid, payment.Data["money"], actualType, purchase.key)
								require.Equal(t, "success", callback.Body.String())
							}
							prefix, _, _ := strings.Cut(purchase.method, ".")
							stored := model.GetTopUpByTradeNo(tradeNo)
							require.NotNil(t, stored)
							assert.Equal(t, prefix+"."+actualType, stored.EffectivePaymentMethod())
							assert.Equal(t, purchase.legacyType, stored.PaymentMethod)
							assert.Equal(t, model.PaymentProviderEpayChannel, stored.PaymentProvider)
							assert.Equal(t, common.TopUpStatusSuccess, stored.Status)
							if subscription {
								order := model.GetSubscriptionOrderByTradeNo(tradeNo)
								require.NotNil(t, order)
								assert.Equal(t, prefix+"."+actualType, order.EffectivePaymentMethod())
								assert.Equal(t, purchase.legacyType, order.PaymentMethod)
								assert.Equal(t, model.PaymentProviderEpayChannel, order.PaymentProvider)
								returned := epayChannelCallback(t, SubscriptionEpayReturn, http.MethodGet, tradeNo, purchase.pid, payment.Data["money"], actualType, purchase.key)
								assert.Equal(t, http.StatusFound, returned.Code)
								assert.Contains(t, returned.Header().Get("Location"), "pay=success")
							}
						}
					})
				}
				var user model.User
				require.NoError(t, db.First(&user, 1).Error)
				assert.Equal(t, 4_000_000, user.Quota, "each topup credits once; subscriptions never credit wallet")
				var subscriptions int64
				require.NoError(t, db.Model(&model.UserSubscription{}).Where("user_id = ?", 1).Count(&subscriptions).Error)
				assert.EqualValues(t, 4, subscriptions, "notify/return retries must not duplicate subscriptions")
				for _, invalid := range []string{"wxpay", "unknown.wxpay", "achannel.", "achannel.wxpay.extra", "achannel.alipay"} {
					rejected := epayChannelRequest(t, RequestEpay, map[string]any{"amount": 2, "payment_method": invalid})
					assert.Contains(t, rejected.Body.String(), `"message":"error"`, invalid)
				}
				modelManagementRequest(t, UpdateOption, http.MethodPut, "/api/option/", OptionUpdateRequest{Key: "payment_setting.epay_channels", Value: "[]"}, &result)
				require.True(t, result.Success)
				var topupInfo struct {
					Data struct {
						EnableOnlineTopup bool                `json:"enable_online_topup"`
						PayMethods        []map[string]string `json:"pay_methods"`
					}
				}
				modelManagementRequest(t, GetTopUpInfo, http.MethodGet, "/api/user/topup/info", nil, &topupInfo)
				assert.False(t, topupInfo.Data.EnableOnlineTopup)
				for _, method := range topupInfo.Data.PayMethods {
					assert.NotContains(t, method["type"], "channel.")
				}
			})

			t.Run("legacy storage remains usable after downgrade", func(t *testing.T) {
				db := epayChannelDatabase(t, dialect.kind, os.Getenv(dialect.env))
				legacyType := strings.Repeat("x", 45) + ".bank"
				legacyOptions := map[string]string{
					"PayAddress": "https://legacy.example.com", "EpayId": "9001", "EpayKey": "legacy-key",
					"PayMethods": `[{"name":"WeChat","type":"wxpay"},{"name":"Stripe","type":"stripe"},{"name":"Custom","type":"` + legacyType + `"}]`,
				}
				for key, value := range legacyOptions {
					require.NoError(t, db.Create(&model.Option{Key: key, Value: value}).Error)
				}
				namedCredentials := `[{"name":"extra","pay_address":"https://extra.example.com","epay_id":"9002","epay_key":"extra-key"}]`
				require.NoError(t, db.Create(&model.Option{Key: operation_setting.EpayChannelsOptionKey, Value: namedCredentials}).Error)
				require.NoError(t, db.Create(&model.User{Id: 1, Username: "legacy-user", Group: "default", Status: common.UserStatusEnabled}).Error)
				for _, order := range []model.TopUp{
					{UserId: 1, Amount: 2, Money: 2, TradeNo: "legacy-pending", PaymentMethod: "wxpay", PaymentProvider: model.PaymentProviderEpay, Status: common.TopUpStatusPending},
					{UserId: 1, Amount: 2, Money: 2, TradeNo: "stripe-pending", PaymentMethod: "stripe", PaymentProvider: model.PaymentProviderStripe, Status: common.TopUpStatusPending},
					{UserId: 1, Amount: 2, Money: 2, TradeNo: "legacy-custom", PaymentMethod: legacyType, PaymentProvider: model.PaymentProviderEpay, Status: common.TopUpStatusSuccess},
				} {
					require.NoError(t, db.Create(&order).Error)
				}
				require.NoError(t, db.Create(&model.SubscriptionOrder{UserId: 1, PlanId: 1, Money: 2, TradeNo: "legacy-subscription", PaymentMethod: legacyType, PaymentProvider: model.PaymentProviderEpay, Status: common.TopUpStatusPending}).Error)
				for pass := range 2 {
					if pass == 1 {
						require.NoError(t, db.Create(&model.Option{Key: operation_setting.EpayChannelsOptionKey, Value: strings.ReplaceAll(namedCredentials, "extra-key", "stale-key")}).Error)
					}
					model.InitOptionMap()
					var credentials model.Option
					require.NoError(t, db.Where(&model.Option{Key: operation_setting.EpayChannelsStorageKey}).First(&credentials).Error)
					assert.Equal(t, namedCredentials, credentials.Value, "migration must preserve credentials, including after a stale public row reappears")
					var exposedRows int64
					require.NoError(t, db.Model(&model.Option{}).Where(&model.Option{Key: operation_setting.EpayChannelsOptionKey}).Count(&exposedRows).Error)
					assert.Zero(t, exposedRows, "old binaries must not return channel credentials as an ordinary option")
					response := epayChannelRequest(t, GetOptions, nil)
					assert.NotContains(t, response.Body.String(), "extra-key")
					assert.NotContains(t, response.Body.String(), "stale-key")
					assert.NotContains(t, response.Body.String(), "legacy-key")
					assert.Contains(t, response.Body.String(), "extra.example.com")
					legacy := model.GetTopUpByTradeNo("legacy-pending")
					require.NotNil(t, legacy)
					assert.Equal(t, "wxpay", legacy.PaymentMethod)
					assert.Equal(t, "default.wxpay", legacy.EffectivePaymentMethod())
					assert.Empty(t, legacy.EpayMethod)
					assert.Equal(t, "stripe", model.GetTopUpByTradeNo("stripe-pending").PaymentMethod)
					assert.True(t, operation_setting.ContainsPayMethod("default.wxpay"))
					assert.True(t, operation_setting.ContainsPayMethod("stripe"))
					assert.True(t, operation_setting.ContainsPayMethod("default."+legacyType))
					assert.Equal(t, legacyType, model.GetTopUpByTradeNo("legacy-custom").PaymentMethod)
					assert.Equal(t, legacyType, model.GetSubscriptionOrderByTradeNo("legacy-subscription").PaymentMethod)
					for key, value := range legacyOptions {
						var stored model.Option
						require.NoError(t, db.Where(&model.Option{Key: key}).First(&stored).Error)
						assert.Equal(t, value, stored.Value, "startup must not rewrite legacy option %s", key)
					}
				}
				rejectedStorageWrite := epayChannelRequest(t, UpdateOption, OptionUpdateRequest{Key: operation_setting.EpayChannelsStorageKey, Value: "[]"})
				assert.Contains(t, rejectedStorageWrite.Body.String(), `"success":false`)
				require.NoError(t, model.UpdateOption("PayMethods", `[{"name":"Extra","type":"extra.bank"}]`))
				namedPayment := epayChannelRequest(t, RequestEpay, map[string]any{"amount": 2, "payment_method": "extra.bank"})
				var namedCheckout struct {
					Message string
					Data    map[string]string
				}
				require.NoError(t, common.Unmarshal(namedPayment.Body.Bytes(), &namedCheckout))
				require.Equal(t, "success", namedCheckout.Message)
				assert.Equal(t, "9002", namedCheckout.Data["pid"])
				assert.Equal(t, "success", epayChannelCallback(t, EpayNotify, http.MethodPost, namedCheckout.Data["out_trade_no"], "9002", "2.00", "bank", "extra-key").Body.String())
				callback := epayChannelCallback(t, EpayNotify, http.MethodPost, "legacy-pending", "9001", "2.00", "wxpay", "legacy-key")
				require.Equal(t, "success", callback.Body.String())
				var user model.User
				require.NoError(t, db.First(&user, 1).Error)
				assert.Equal(t, 2_000_000, user.Quota)
				require.NoError(t, model.UpdateOption("payment_setting.epay_channels", `[{"name":"default","pay_address":"https://legacy.example.com","epay_id":"9001","epay_key":""},{"name":"extra","pay_address":"https://extra.example.com","epay_id":"9002","epay_key":"extra-key"}]`))
				require.NoError(t, model.UpdateOption("PayMethods", `[{"name":"WeChat","type":"default.wxpay"},{"name":"Extra","type":"extra.bank"},{"name":"Stripe","type":"stripe"}]`))
				var legacyMethods model.Option
				require.NoError(t, db.Where(&model.Option{Key: "PayMethods"}).First(&legacyMethods).Error)
				var methods []map[string]string
				require.NoError(t, common.UnmarshalJsonStr(legacyMethods.Value, &methods))
				assert.Equal(t, []map[string]string{{"name": "WeChat", "type": "wxpay"}, {"name": "Stripe", "type": "stripe"}}, methods)
				// Simulate settings edited by the old binary, which only knows the legacy keys.
				for key, value := range map[string]string{"EpayId": "9010", "EpayKey": "rotated-key", "PayMethods": `[{"name":"Alipay","type":"alipay"},{"name":"Custom","type":"` + legacyType + `"}]`} {
					require.NoError(t, db.Save(&model.Option{Key: key, Value: value}).Error)
				}
				for range 2 {
					model.InitOptionMap()
					assert.True(t, operation_setting.ContainsPayMethod("default.alipay"))
					assert.True(t, operation_setting.ContainsPayMethod("alipay"))
					assert.True(t, operation_setting.ContainsPayMethod("default."+legacyType))
					assert.True(t, operation_setting.ContainsPayMethod("extra.bank"))
					assert.False(t, operation_setting.ContainsPayMethod("default.wxpay"))
				}
				paymentResponse := epayChannelRequest(t, RequestEpay, map[string]any{"amount": 2, "payment_method": "alipay"})
				var payment struct {
					Message string
					Data    map[string]string
				}
				require.NoError(t, common.Unmarshal(paymentResponse.Body.Bytes(), &payment))
				require.Equal(t, "success", payment.Message, paymentResponse.Body.String())
				assert.Equal(t, "9010", payment.Data["pid"])
				assert.Equal(t, "alipay", payment.Data["type"])
				created := model.GetTopUpByTradeNo(payment.Data["out_trade_no"])
				require.NotNil(t, created)
				assert.Equal(t, "alipay", created.PaymentMethod)
				assert.Equal(t, model.PaymentProviderEpay, created.PaymentProvider)
				assert.Equal(t, "default.alipay", created.EffectivePaymentMethod())
				callback = epayChannelCallback(t, EpayNotify, http.MethodPost, created.TradeNo, "9010", "2.00", "alipay", "rotated-key")
				require.Equal(t, "success", callback.Body.String())
				require.NoError(t, db.First(&user, 1).Error)
				assert.Equal(t, 3_000_000, user.Quota)
				legacyWithCollision := `[{"name":"Legacy dotted","type":"extra.bank"},null]`
				require.NoError(t, db.Save(&model.Option{Key: "PayMethods", Value: legacyWithCollision}).Error)
				model.InitOptionMap()
				assert.False(t, operation_setting.ContainsPayMethod("extra.bank"), "ambiguous old identifiers must not switch merchants")
				assert.True(t, operation_setting.ContainsPayMethod("default.extra.bank"))
				legacyPlan := model.SubscriptionPlan{Title: "Legacy collision", PriceAmount: 2, Currency: "USD", DurationUnit: "month", DurationValue: 1, Enabled: true}
				require.NoError(t, db.Create(&legacyPlan).Error)
				model.InvalidateSubscriptionPlanCache(legacyPlan.Id)
				t.Cleanup(func() { model.InvalidateSubscriptionPlanCache(legacyPlan.Id) })
				for _, handler := range []gin.HandlerFunc{RequestEpay, SubscriptionRequestEpay} {
					body := map[string]any{"amount": 2, "plan_id": legacyPlan.Id, "payment_method": "extra.bank"}
					rejected := epayChannelRequest(t, handler, body)
					var result struct{ Message string }
					require.NoError(t, common.Unmarshal(rejected.Body.Bytes(), &result))
					assert.NotEqual(t, "success", result.Message)
					body["payment_method"] = "default.extra.bank"
					accepted := epayChannelRequest(t, handler, body)
					var checkout struct {
						Message string
						Data    map[string]string
					}
					require.NoError(t, common.Unmarshal(accepted.Body.Bytes(), &checkout))
					require.Equal(t, "success", checkout.Message, accepted.Body.String())
					assert.Equal(t, "9010", checkout.Data["pid"])
					assert.Equal(t, "extra.bank", checkout.Data["type"])
				}
				var untouched model.Option
				require.NoError(t, db.Where(&model.Option{Key: "PayMethods"}).First(&untouched).Error)
				assert.Equal(t, legacyWithCollision, untouched.Value)
				require.NoError(t, model.UpdateOption("PayMethods", `[{"name":"WeChat","type":"wxpay"}]`))
				model.InitOptionMap()
				assert.True(t, operation_setting.ContainsPayMethod("default.wxpay"))
				assert.True(t, operation_setting.ContainsPayMethod("extra.bank"))
				assert.False(t, operation_setting.ContainsPayMethod("default.alipay"))
				require.NoError(t, model.UpdateOption("PayMethods", "[]"))
				model.InitOptionMap()
				assert.False(t, isEpayTopUpEnabled())
				assert.Empty(t, operation_setting.PayMethods)
				t.Logf("%s: legacy storage and old-version edits survive repeated reloads", dialect.kind)
			})
			t.Run("incomplete legacy credentials remain editable", func(t *testing.T) {
				db := epayChannelDatabase(t, dialect.kind, os.Getenv(dialect.env))
				require.NoError(t, db.Create(&model.Option{Key: "EpayId", Value: "9001"}).Error)
				require.NoError(t, db.Create(&model.Option{Key: "EpayKey", Value: "retained-key"}).Error)
				for range 2 {
					model.InitOptionMap()
					assert.False(t, isEpayTopUpEnabled())
					channels := operation_setting.GetPaymentSetting().EpayChannels
					require.Len(t, channels, 1)
					assert.Equal(t, "9001", channels[0].EpayId)
					assert.Equal(t, "retained-key", channels[0].EpayKey)
				}
				require.NoError(t, model.UpdateOption("payment_setting.epay_channels", `[{"name":"default","pay_address":"https://restored.example.com","epay_id":"9001","epay_key":""}]`))
				channel, _, err := operation_setting.ResolveEpayChannel("default.wxpay")
				require.NoError(t, err)
				assert.Equal(t, "retained-key", channel.EpayKey)
			})
		})
	}
}
