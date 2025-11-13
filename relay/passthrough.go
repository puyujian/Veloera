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
package relay

import (
	"strings"

	"veloera/constant"
	relaycommon "veloera/relay/common"
	"veloera/setting/model_setting"
)

type openAIPassThroughSupport interface {
	SupportsOpenAIPassThrough(*relaycommon.RelayInfo) bool
}

// shouldUsePassThrough checks the channel-level toggle first, then falls back to the global toggle.
// It also lets adaptors opt out when the upstream API is not OpenAI-compatible.
func shouldUsePassThrough(adaptor interface{}, info *relaycommon.RelayInfo) bool {
	if adaptor == nil {
		return false
	}

	// Check channel-level pass-through setting first
	if info != nil {
		if enable, ok := extractPassThroughSetting(info.ChannelSetting); ok {
			// Channel-level setting exists, use it
			if !enable {
				return false
			}
			// Channel-level pass-through is enabled, check if adaptor supports it
			if support, ok := adaptor.(openAIPassThroughSupport); ok {
				return support.SupportsOpenAIPassThrough(info)
			}
			return true
		}
	}

	// Fall back to global setting
	if !model_setting.GetGlobalSettings().PassThroughRequestEnabled {
		return false
	}

	if support, ok := adaptor.(openAIPassThroughSupport); ok {
		return support.SupportsOpenAIPassThrough(info)
	}

	return true
}

// extractPassThroughSetting extracts the pass_through setting from channel setting map
func extractPassThroughSetting(setting map[string]interface{}) (bool, bool) {
	if setting == nil {
		return false, false
	}

	value, exists := setting[constant.ChannelSettingPassThrough]
	if !exists {
		return false, false
	}

	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		lower := strings.ToLower(strings.TrimSpace(v))
		if lower == "true" || lower == "1" || lower == "yes" || lower == "on" {
			return true, true
		}
		if lower == "false" || lower == "0" || lower == "no" || lower == "off" {
			return false, true
		}
	}

	return false, false
}
