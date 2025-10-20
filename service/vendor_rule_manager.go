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

package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"veloera/common"
)

const (
	vendorMetadataURL = "https://llm-metadata.pages.dev/api/index.json"
	refreshInterval   = 24 * time.Hour
)

// ModelMetadata 模型元数据
type ModelMetadata struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ProviderID string `json:"providerId"`
}

// VendorMetadataResponse 远程元数据响应
type VendorMetadataResponse struct {
	Models []ModelMetadata `json:"models"`
}

// VendorRule 厂商识别规则
type VendorRule struct {
	ProviderID  string
	DisplayName string
	Pattern     *regexp.Regexp
}

// VendorRuleManager 厂商规则管理器
type VendorRuleManager struct {
	rules      []*VendorRule
	mutex      sync.RWMutex
	lastUpdate time.Time
}

var (
	globalVendorManager *VendorRuleManager
	vendorManagerOnce   sync.Once
)

// providerDisplayNames providerId 到显示名称的映射
var providerDisplayNames = map[string]string{
	"anthropic":               "Anthropic",
	"openai":                  "OpenAI",
	"google":                  "Google",
	"alibaba":                 "阿里巴巴",
	"alibaba-cn":              "通义千问",
	"doubao":                  "豆包",
	"moonshot":                "Moonshot",
	"deepseek":                "DeepSeek",
	"zhipu":                   "智谱",
	"tencent":                 "腾讯",
	"baidu":                   "百度",
	"minimax":                 "MiniMax",
	"mistral":                 "MistralAI",
	"xai":                     "xAI",
	"meta":                    "Meta",
	"amazon-bedrock":          "Amazon Bedrock",
	"azure":                   "Azure",
	"cloudflare-workers-ai":   "Cloudflare",
	"cerebras":                "Cerebras",
	"deepinfra":               "DeepInfra",
	"fireworks-ai":            "Fireworks AI",
	"github-models":           "GitHub Models",
	"huggingface":             "Hugging Face",
	"together-ai":             "Together AI",
	"groq":                    "Groq",
	"perplexity":              "Perplexity",
	"cohere":                  "Cohere",
	"ai21":                    "AI21 Labs",
	"stability-ai":            "Stability AI",
	"replicate":               "Replicate",
}

// GetVendorRuleManager 获取全局厂商规则管理器（单例）
func GetVendorRuleManager() *VendorRuleManager {
	vendorManagerOnce.Do(func() {
		globalVendorManager = &VendorRuleManager{
			rules: make([]*VendorRule, 0),
		}
	})
	return globalVendorManager
}

// InitVendorRules 初始化厂商规则（启动时调用）
func InitVendorRules() error {
	manager := GetVendorRuleManager()
	if err := manager.LoadRules(); err != nil {
		common.SysError(fmt.Sprintf("加载厂商规则失败: %v，使用默认规则", err))
		manager.loadDefaultRules()
		return err
	}

	// 启动定期刷新
	go manager.startAutoRefresh()

	return nil
}

// LoadRules 从远程加载厂商规则
func (m *VendorRuleManager) LoadRules() error {
	// 获取远程JSON
	responseBody, statusCode, err := DoHTTPRequest("GET", vendorMetadataURL, nil, nil)
	if err != nil {
		return fmt.Errorf("请求远程元数据失败: %w", err)
	}

	if statusCode != 200 {
		return fmt.Errorf("远程元数据返回错误状态码: %d", statusCode)
	}

	// 解析JSON
	var response VendorMetadataResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return fmt.Errorf("解析远程元数据失败: %w", err)
	}

	// 构建规则
	rules := m.buildRulesFromMetadata(response.Models)

	// 更新规则
	m.mutex.Lock()
	m.rules = rules
	m.lastUpdate = time.Now()
	m.mutex.Unlock()

	common.SysLog(fmt.Sprintf("成功加载 %d 个厂商规则", len(rules)))
	return nil
}

// buildRulesFromMetadata 从模型元数据构建厂商规则
func (m *VendorRuleManager) buildRulesFromMetadata(models []ModelMetadata) []*VendorRule {
	// 按 providerId 分组，收集模型前缀
	providerPrefixes := make(map[string]map[string]struct{})

	for _, model := range models {
		if model.ProviderID == "" || model.ID == "" {
			continue
		}

		if providerPrefixes[model.ProviderID] == nil {
			providerPrefixes[model.ProviderID] = make(map[string]struct{})
		}

		// 提取前缀（取第一个 '-' 前的部分）
		parts := strings.Split(model.ID, "-")
		if len(parts) > 0 {
			prefix := strings.ToLower(strings.TrimSpace(parts[0]))
			if prefix != "" {
				providerPrefixes[model.ProviderID][prefix] = struct{}{}
			}
		}

		// 如果没有 '-'，使用完整ID作为前缀（某些简短模型名）
		if !strings.Contains(model.ID, "-") {
			prefix := strings.ToLower(strings.TrimSpace(model.ID))
			if prefix != "" && len(prefix) <= 15 { // 限制长度避免整个ID作为前缀
				providerPrefixes[model.ProviderID][prefix] = struct{}{}
			}
		}
	}

	// 生成规则
	rules := make([]*VendorRule, 0, len(providerPrefixes))

	for providerID, prefixes := range providerPrefixes {
		if len(prefixes) == 0 {
			continue
		}

		// 获取显示名称
		displayName := providerDisplayNames[providerID]
		if displayName == "" {
			// 如果没有映射，使用 providerId 转换为首字母大写
			displayName = capitalizeFirst(providerID)
		}

		// 构建正则表达式（匹配任意前缀）
		prefixList := make([]string, 0, len(prefixes))
		for prefix := range prefixes {
			prefixList = append(prefixList, regexp.QuoteMeta(prefix))
		}

		pattern := fmt.Sprintf("(?i)^(%s)", strings.Join(prefixList, "|"))
		regex, err := regexp.Compile(pattern)
		if err != nil {
			common.SysError(fmt.Sprintf("编译厂商规则失败 (%s): %v", providerID, err))
			continue
		}

		rules = append(rules, &VendorRule{
			ProviderID:  providerID,
			DisplayName: displayName,
			Pattern:     regex,
		})
	}

	return rules
}

// GetRules 获取当前厂商规则
func (m *VendorRuleManager) GetRules() []*VendorRule {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 返回副本
	rules := make([]*VendorRule, len(m.rules))
	copy(rules, m.rules)
	return rules
}

// MatchVendor 匹配模型名的厂商
func (m *VendorRuleManager) MatchVendor(modelName string) string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	modelName = strings.ToLower(strings.TrimSpace(modelName))

	for _, rule := range m.rules {
		if rule.Pattern.MatchString(modelName) {
			return rule.DisplayName
		}
	}

	return ""
}

// startAutoRefresh 启动自动刷新
func (m *VendorRuleManager) startAutoRefresh() {
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := m.LoadRules(); err != nil {
			common.SysError(fmt.Sprintf("自动刷新厂商规则失败: %v", err))
		}
	}
}

// loadDefaultRules 加载默认规则（降级方案）
func (m *VendorRuleManager) loadDefaultRules() {
	defaultRules := []*VendorRule{
		{ProviderID: "anthropic", DisplayName: "Anthropic", Pattern: regexp.MustCompile(`(?i)^claude`)},
		{ProviderID: "openai", DisplayName: "OpenAI", Pattern: regexp.MustCompile(`(?i)^(gpt|chatgpt|o1|o3)`)},
		{ProviderID: "google", DisplayName: "Google", Pattern: regexp.MustCompile(`(?i)^gemini`)},
		{ProviderID: "alibaba", DisplayName: "阿里巴巴", Pattern: regexp.MustCompile(`(?i)^qwen`)},
		{ProviderID: "alibaba-cn", DisplayName: "通义千问", Pattern: regexp.MustCompile(`(?i)^(qwen|tongyi)`)},
		{ProviderID: "deepseek", DisplayName: "DeepSeek", Pattern: regexp.MustCompile(`(?i)^deepseek`)},
		{ProviderID: "moonshot", DisplayName: "Moonshot", Pattern: regexp.MustCompile(`(?i)^moonshot`)},
		{ProviderID: "zhipu", DisplayName: "智谱", Pattern: regexp.MustCompile(`(?i)^(glm|bigmodel)`)},
		{ProviderID: "mistral", DisplayName: "MistralAI", Pattern: regexp.MustCompile(`(?i)^mistral`)},
		{ProviderID: "xai", DisplayName: "xAI", Pattern: regexp.MustCompile(`(?i)^grok`)},
		{ProviderID: "meta", DisplayName: "Meta", Pattern: regexp.MustCompile(`(?i)^llama`)},
	}

	m.mutex.Lock()
	m.rules = defaultRules
	m.lastUpdate = time.Now()
	m.mutex.Unlock()

	common.SysLog("使用默认厂商规则")
}

// capitalizeFirst 首字母大写
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	// 处理 kebab-case: provider-id -> Provider Id
	s = strings.ReplaceAll(s, "-", " ")
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}
