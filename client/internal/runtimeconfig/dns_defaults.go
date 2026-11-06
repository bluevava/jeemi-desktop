package runtimeconfig

import (
	_ "embed"
	"encoding/json"
)

// 与浏览器开发预览共用一份默认列表；只用于新配置或缺失字段，不能覆盖已保存的值。
//
//go:embed fake_ip_filter.json
var defaultDNSFakeIPFilterJSON []byte

var defaultDNSFakeIPFilter = func() []string {
	var values []string
	if err := json.Unmarshal(defaultDNSFakeIPFilterJSON, &values); err != nil {
		panic("invalid built-in fake-ip-filter: " + err.Error())
	}
	return values
}()
