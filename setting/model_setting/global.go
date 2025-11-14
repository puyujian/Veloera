// Copyright (c) 2025 Tethys Plex
//
// This file is part of Veloera.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.
package model_setting

import (
	"strconv"
	"strings"
	"veloera/setting/config"
)

type GlobalSettings struct {
	PassThroughRequestEnabled    bool   `json:"pass_through_request_enabled"`
	HideUpstreamErrorEnabled     bool   `json:"hide_upstream_error_enabled"`
	BlockBrowserExtensionEnabled bool   `json:"block_browser_extension_enabled"`
	RateLimitExemptEnabled       bool   `json:"rate_limit_exempt_enabled"`
	RateLimitExemptGroup         string `json:"rate_limit_exempt_group"`
	SafeCheckExemptEnabled       bool   `json:"safe_check_exempt_enabled"`
	SafeCheckExemptGroup         string `json:"safe_check_exempt_group"`
	AutoRetryEnabled             bool   `json:"auto_retry_enabled"`
	AutoRetryCount               int    `json:"auto_retry_count"`
	AutoRetryForceChannelSwitch  bool   `json:"auto_retry_force_channel_switch"`
	AutoRetryStatusCodes         string `json:"auto_retry_status_codes"`
	EmptyResponseRetryEnabled    bool   `json:"empty_response_retry_enabled"`
}

// 默认配置
var defaultOpenaiSettings = GlobalSettings{
	PassThroughRequestEnabled:    false,
	HideUpstreamErrorEnabled:     false,
	BlockBrowserExtensionEnabled: false,
	RateLimitExemptEnabled:       false,
	RateLimitExemptGroup:         "bulk-ok",
	SafeCheckExemptEnabled:       false,
	SafeCheckExemptGroup:         "nsfw-ok",
	AutoRetryEnabled:             false,
	AutoRetryCount:               3,
	AutoRetryForceChannelSwitch:  false,
	AutoRetryStatusCodes:         "5xx,4xx",
	EmptyResponseRetryEnabled:    false,
}

// 全局实例
var globalSettings = defaultOpenaiSettings

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("global", &globalSettings)
}

func GetGlobalSettings() *GlobalSettings {
	return &globalSettings
}

func ShouldBypassRateLimit(group string) bool {
	return globalSettings.RateLimitExemptEnabled && group == globalSettings.RateLimitExemptGroup
}

func ShouldBypassSafeCheck(group string) bool {
	return globalSettings.SafeCheckExemptEnabled && group == globalSettings.SafeCheckExemptGroup
}

// GetAutoRetryCount 获取自动重试次数
func GetAutoRetryCount() int {
	if !globalSettings.AutoRetryEnabled {
		return 0
	}
	return globalSettings.AutoRetryCount
}

// ShouldForceChannelSwitch 是否强制切换渠道
func ShouldForceChannelSwitch() bool {
	return globalSettings.AutoRetryEnabled && globalSettings.AutoRetryForceChannelSwitch
}

// ShouldTreatEmptyResponseAsError 是否将空回复视为错误并触发重试
func ShouldTreatEmptyResponseAsError() bool {
	return globalSettings.AutoRetryEnabled && globalSettings.EmptyResponseRetryEnabled
}

// ShouldRetryForStatusCode 检查状态码是否应该重试
func ShouldRetryForStatusCode(statusCode int) bool {
	if !globalSettings.AutoRetryEnabled {
		return false
	}

	if globalSettings.AutoRetryStatusCodes == "" {
		return true // 默认所有状态码都重试
	}

	codes := strings.Split(globalSettings.AutoRetryStatusCodes, ",")
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}

		// 处理 5xx, 4xx 这样的通配符
		if strings.HasSuffix(code, "xx") {
			prefix := strings.TrimSuffix(code, "xx")
			if prefixNum, err := strconv.Atoi(prefix); err == nil {
				if statusCode/100 == prefixNum {
					return true
				}
			}
		} else {
			// 处理具体的状态码
			if codeNum, err := strconv.Atoi(code); err == nil {
				if statusCode == codeNum {
					return true
				}
			}
		}
	}

	return false
}
