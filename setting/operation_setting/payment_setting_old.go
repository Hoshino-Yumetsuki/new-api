/**
此文件为旧版支付设置文件，如需增加新的参数、变量等，请在 payment_setting.go 中添加
This file is the old version of the payment settings file. If you need to add new parameters, variables, etc., please add them in payment_setting.go
*/

package operation_setting

import (
	"github.com/QuantumNous/new-api/common"
)

var PayAddress = ""
var EpayId = ""
var EpayKey = ""
var CustomCallbackAddress = ""
var Price = 7.3
var MinTopUp = 1
var USDExchangeRate = 7.3

var PayMethods = []map[string]string{}

func PayMethods2JsonString() string {
	jsonBytes, err := common.Marshal(PayMethods)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}

func ContainsPayMethod(method string) bool {
	method, err := NormalizeEpayMethod(method)
	if err != nil {
		return false
	}
	for _, payMethod := range PayMethods {
		if payMethod["type"] == method {
			return true
		}
	}
	return false
}
