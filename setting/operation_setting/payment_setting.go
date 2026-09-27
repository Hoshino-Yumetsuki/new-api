package operation_setting

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const (
	AffCommissionTypePercentage = "percentage"
	AffCommissionTypeFixed      = "fixed"
)

type PaymentSetting struct {
	EpayChannels   []EpayChannel   `json:"epay_channels"`
	AmountOptions  []int           `json:"amount_options"`
	AmountDiscount map[int]float64 `json:"amount_discount"` // 充值金额对应的折扣，例如 100 元 0.9 表示 100 元充值享受 9 折优惠

	ComplianceConfirmed    bool   `json:"compliance_confirmed"`
	ComplianceTermsVersion string `json:"compliance_terms_version"`
	ComplianceConfirmedAt  int64  `json:"compliance_confirmed_at"`
	ComplianceConfirmedBy  int    `json:"compliance_confirmed_by"`
	ComplianceConfirmedIP  string `json:"compliance_confirmed_ip"`

	// 邀请佣金设置
	AffCommissionEnabled     bool    `json:"aff_commission_enabled"`
	AffCommissionType        string  `json:"aff_commission_type"`         // "percentage" 或 "fixed"
	AffCommissionRate        float64 `json:"aff_commission_rate"`         // 百分比佣金比例，0-100
	AffCommissionFixedAmount int     `json:"aff_commission_fixed_amount"` // 固定佣金额度（quota 单位）
}

const CurrentComplianceTermsVersion = "v1"

// 默认配置
var paymentSetting = PaymentSetting{
	EpayChannels:   []EpayChannel{},
	AmountOptions:  []int{10, 20, 50, 100, 200, 500},
	AmountDiscount: map[int]float64{},
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("payment_setting", &paymentSetting)
}

func GetPaymentSetting() *PaymentSetting {
	return &paymentSetting
}

func IsPaymentComplianceConfirmed() bool {
	return paymentSetting.ComplianceConfirmed &&
		paymentSetting.ComplianceTermsVersion == CurrentComplianceTermsVersion
}

func GetAffCommissionSetting() (enabled bool, commType string, rate float64, fixedAmount int) {
	return paymentSetting.AffCommissionEnabled,
		paymentSetting.AffCommissionType,
		paymentSetting.AffCommissionRate,
		paymentSetting.AffCommissionFixedAmount
}

const EpayChannelsOptionKey = "payment_setting.epay_channels"

// The storage name must match old versions' sensitive-key filter on downgrade.
const EpayChannelsStorageKey = "EpayChannelsKey"
const EpayPayMethodsOptionKey = "payment_setting.epay_pay_methods"
const DefaultPayMethodsJSON = `[{"name":"支付宝","icon":"SiAlipay","type":"alipay"},{"name":"微信","icon":"SiWechat","type":"wxpay"},{"name":"自定义1","icon":"LuCreditCard","type":"custom1","min_topup":"50"}]`

type EpayChannel struct {
	Name       string `json:"name"`
	PayAddress string `json:"pay_address"`
	EpayId     string `json:"epay_id"`
	EpayKey    string `json:"epay_key"`
}

// ComposeEpayOptions builds the modern view without changing legacy storage.
func ComposeEpayOptions(values map[string]string) ([]EpayChannel, []map[string]string, error) {
	channels := []EpayChannel{}
	if value := values[EpayChannelsStorageKey]; value != "" {
		if err := common.UnmarshalJsonStr(value, &channels); err != nil {
			return nil, nil, err
		}
	}
	for _, channel := range channels {
		if channel.Name == "default" {
			return nil, nil, errors.New("default Epay credentials must use PayAddress, EpayId and EpayKey")
		}
	}
	legacy := EpayChannel{Name: "default", PayAddress: values["PayAddress"], EpayId: values["EpayId"], EpayKey: values["EpayKey"]}
	if legacy.PayAddress != "" || legacy.EpayId != "" || legacy.EpayKey != "" {
		channels = append([]EpayChannel{legacy}, channels...)
	}
	value, exists := values["PayMethods"]
	if !exists {
		value = DefaultPayMethodsJSON
	}
	methods := []map[string]string{}
	if err := common.UnmarshalJsonStr(value, &methods); err != nil {
		return nil, nil, err
	}
	configuredMethods := methods[:0]
	for _, method := range methods {
		if method == nil {
			continue
		}
		if !IsNativePaymentMethod(method["type"]) {
			// Legacy upstream types may contain dots; never parse them as namespaces.
			method["type"] = "default." + method["type"]
		}
		configuredMethods = append(configuredMethods, method)
	}
	var namedMethods []map[string]string
	if value := values[EpayPayMethodsOptionKey]; value != "" {
		if err := common.UnmarshalJsonStr(value, &namedMethods); err != nil {
			return nil, nil, err
		}
	}
	return channels, append(configuredMethods, namedMethods...), nil
}

// NormalizeEpayMethod accepts configured legacy aliases but rejects identifiers
// that could select either a named channel or a dotted default-channel type.
func NormalizeEpayMethod(method string) (string, error) {
	if IsNativePaymentMethod(method) {
		return method, nil
	}
	legacyMethod := "default." + method
	var qualified, legacyAlias bool
	for _, configured := range PayMethods {
		qualified = qualified || configured["type"] == method
		legacyAlias = legacyAlias || configured["type"] == legacyMethod
	}
	if qualified && legacyAlias {
		return "", fmt.Errorf("ambiguous Epay payment method: %s", method)
	}
	if legacyAlias {
		return legacyMethod, nil
	}
	return method, nil
}

func validEpayChannelName(name string) bool {
	if len(name) == 0 || len(name) > 48 {
		return false
	}
	for _, c := range name {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return false
		}
	}
	return true
}

func ParseEpayMethod(method string) (string, string, error) {
	name, upstreamType, ok := strings.Cut(method, ".")
	if !ok || !validEpayChannelName(name) || upstreamType == "" || len(method) > 255 {
		return "", "", errors.New("invalid Epay payment method: expected channel.type (maximum 255 UTF-8 bytes)")
	}
	return name, upstreamType, nil
}

func (channel EpayChannel) Validate() error {
	if !validEpayChannelName(channel.Name) {
		return errors.New("Epay channel name must contain 1-48 ASCII letters")
	}
	address, err := url.Parse(channel.PayAddress)
	if err != nil || address.Hostname() == "" || (address.Scheme != "https" && address.Scheme != "http") || address.User != nil || address.Fragment != "" {
		return fmt.Errorf("invalid payment URL for Epay channel %s", channel.Name)
	}
	if strings.TrimSpace(channel.EpayId) == "" || strings.TrimSpace(channel.EpayKey) == "" {
		return fmt.Errorf("merchant ID and key are required for Epay channel %s", channel.Name)
	}
	return nil
}

// ParseEpayChannels preserves an omitted secret only for an identically named channel.
func ParseEpayChannels(value string, previous []EpayChannel) ([]EpayChannel, error) {
	var payload []map[string]*string
	if err := common.UnmarshalJsonStr(value, &payload); err != nil || payload == nil {
		return nil, errors.New("Epay channels must be a JSON array")
	}
	channels := make([]EpayChannel, 0, len(payload))
	seen := make(map[string]bool, len(payload))
	for _, fields := range payload {
		if len(fields) != 4 || fields["name"] == nil || fields["pay_address"] == nil || fields["epay_id"] == nil || fields["epay_key"] == nil {
			return nil, errors.New("each Epay channel requires name, pay_address, epay_id and epay_key strings")
		}
		channel := EpayChannel{Name: *fields["name"], PayAddress: *fields["pay_address"], EpayId: *fields["epay_id"], EpayKey: *fields["epay_key"]}
		if seen[channel.Name] {
			return nil, fmt.Errorf("duplicate Epay channel name: %s", channel.Name)
		}
		seen[channel.Name] = true
		if channel.EpayKey == "" {
			for _, old := range previous {
				if old.Name == channel.Name {
					channel.EpayKey = old.EpayKey
					break
				}
			}
		}
		if err := channel.Validate(); err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	return channels, nil
}

func ResolveEpayChannel(method string) (EpayChannel, string, error) {
	name, upstreamType, err := ParseEpayMethod(method)
	if err != nil {
		return EpayChannel{}, "", err
	}
	for _, channel := range paymentSetting.EpayChannels {
		if channel.Name == name {
			return channel, upstreamType, channel.Validate()
		}
	}
	return EpayChannel{}, "", fmt.Errorf("Epay channel not found: %s", name)
}

func IsNativePaymentMethod(method string) bool {
	switch method {
	case "stripe", "creem", "waffo", "waffo_pancake", "balance":
		return true
	}
	return false
}
