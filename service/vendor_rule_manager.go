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
	rules         []*VendorRule
	modelMetadata []ModelMetadata // 缓存的模型元数据
	mutex         sync.RWMutex
	lastUpdate    time.Time
}

var (
	globalVendorManager *VendorRuleManager
	vendorManagerOnce   sync.Once
)

// providerDisplayNames providerId 到显示名称的映射
var providerDisplayNames = map[string]string{
	"anthropic":             "Anthropic",
	"openai":                "OpenAI",
	"google":                "Google",
	"alibaba":               "阿里巴巴",
	"alibaba-cn":            "通义千问",
	"doubao":                "豆包",
	"moonshot":              "Moonshot",
	"deepseek":              "DeepSeek",
	"zhipu":                 "智谱",
	"tencent":               "腾讯",
	"baidu":                 "百度",
	"minimax":               "MiniMax",
	"mistral":               "MistralAI",
	"xai":                   "xAI",
	"meta":                  "Meta",
	"amazon-bedrock":        "Amazon Bedrock",
	"azure":                 "Azure",
	"cloudflare-workers-ai": "Cloudflare",
	"cerebras":              "Cerebras",
	"deepinfra":             "DeepInfra",
	"fireworks-ai":          "Fireworks AI",
	"github-models":         "GitHub Models",
	"huggingface":           "Hugging Face",
	"together-ai":           "Together AI",
	"groq":                  "Groq",
	"perplexity":            "Perplexity",
	"cohere":                "Cohere",
	"ai21":                  "AI21 Labs",
	"stability-ai":          "Stability AI",
	"replicate":             "Replicate",
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

	// 更新规则和元数据
	m.mutex.Lock()
	m.rules = rules
	m.modelMetadata = response.Models
	m.lastUpdate = time.Now()
	m.mutex.Unlock()

	common.SysLog(fmt.Sprintf("成功加载 %d 个厂商规则和 %d 个模型元数据", len(rules), len(response.Models)))
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

// FindStandardModelName 查找标准化的模型名称
// 返回: (标准模型ID, 厂商显示名, 是否找到)
func (m *VendorRuleManager) FindStandardModelName(cleanedModelName string) (string, string, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	cleanedModelName = strings.ToLower(strings.TrimSpace(cleanedModelName))

	// 日期后缀正则
	dateSuffixRe := regexp.MustCompile(`-\d{8}$`)

	// 策略1: 精确匹配
	for _, metadata := range m.modelMetadata {
		if strings.ToLower(metadata.ID) == cleanedModelName {
			vendorName := providerDisplayNames[metadata.ProviderID]
			if vendorName == "" {
				vendorName = capitalizeFirst(metadata.ProviderID)
			}
			return metadata.ID, vendorName, true
		}
	}

	// 策略2: 移除 - 不再去除日期后缀进行匹配
	// 原因: claude-3-5-sonnet-20240620 和 claude-3-5-sonnet-20241022 是不同的模型版本
	// 不应该将一个版本重命名为另一个版本

	// 策略3: 模糊匹配（处理别名情况，如 claude-haiku-4-5 → claude-3.5-haiku）
	// 但如果输入已包含日期后缀（如 claude-3-haiku-20240307），则不进行模糊匹配
	// 因为这表示用户指定了特定的模型版本，不应该被重命名为其他版本
	if !dateSuffixRe.MatchString(cleanedModelName) {
		// 提取关键词进行匹配
		for _, metadata := range m.modelMetadata {
			if m.isFuzzyMatch(cleanedModelName, strings.ToLower(metadata.ID)) {
				vendorName := providerDisplayNames[metadata.ProviderID]
				if vendorName == "" {
					vendorName = capitalizeFirst(metadata.ProviderID)
				}
				return metadata.ID, vendorName, true
			}
		}
	}

	return "", "", false
}

// isFuzzyMatch 模糊匹配逻辑
func (m *VendorRuleManager) isFuzzyMatch(input, standard string) bool {
	// 去除日期后缀
	dateSuffixRe := regexp.MustCompile(`-\d{8}$`)
	input = dateSuffixRe.ReplaceAllString(input, "")
	standard = dateSuffixRe.ReplaceAllString(standard, "")

	// 提取关键组件
	inputParts := strings.Split(input, "-")
	standardParts := strings.Split(standard, "-")

	// 如果都包含相同的前缀（如 claude）和主要关键词
	if len(inputParts) > 0 && len(standardParts) > 0 {
		// 检查厂商名/模型系列名
		if inputParts[0] != standardParts[0] {
			return false
		}

		// 提取输入和标准的关键词（非版本号）
		inputKeywords := make(map[string]bool)
		for _, part := range inputParts {
			if !isVersionNumber(part) {
				inputKeywords[part] = true
			}
		}

		standardKeywords := make(map[string]bool)
		for _, part := range standardParts {
			if !isVersionNumber(part) {
				standardKeywords[part] = true
			}
		}

		// 双向检查：
		// 1. 标准名的关键词应该在输入中 (原有逻辑)
		// 2. 输入的关键词也应该在标准名中 (新增，防止 thinkg 等重要后缀被忽略)
		matchCount := 0
		for keyword := range standardKeywords {
			if inputKeywords[keyword] {
				matchCount++
			}
		}

		// 检查输入是否有标准名没有的额外关键词
		// 如 DeepSeek-V3.1-thinkg 的 "thinkg" 不在 DeepSeek-V3.1 中
		for keyword := range inputKeywords {
			if !standardKeywords[keyword] {
				// 输入有额外的关键词，说明是不同的模型变体
				return false
			}
		}

		// 如果关键词完全匹配且数量>=2，认为是同一个模型
		// 例如 claude-haiku 和 claude-3.5-haiku 都包含 claude 和 haiku
		return matchCount >= 2
	}

	return false
}

// isVersionNumber 判断是否为版本号（纯数字或x.y格式）
func isVersionNumber(s string) bool {
	// 匹配纯数字或 x.y 格式
	matched, _ := regexp.MatchString(`^\d+(\.\d+)?$`, s)
	return matched
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

	// 添加一些常见的默认模型元数据
	defaultMetadata := []ModelMetadata{
		// Anthropic Claude 系列
		{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", ProviderID: "anthropic"},
		{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", ProviderID: "anthropic"},
		{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", ProviderID: "anthropic"},
		{ID: "claude-sonnet-4-5-20250929", Name: "Claude 4.5 Sonnet", ProviderID: "anthropic"},
		{ID: "claude-haiku-4-5-20251001", Name: "Claude 4.5 Haiku", ProviderID: "anthropic"},
		// OpenAI GPT 系列
		{ID: "gpt-4o", Name: "GPT-4o", ProviderID: "openai"},
		{ID: "gpt-4o-mini", Name: "GPT-4o mini", ProviderID: "openai"},
		{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", ProviderID: "openai"},
		{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", ProviderID: "openai"},
		// Google Gemini 系列
		{ID: "gemini-2.0-flash-exp", Name: "Gemini 2.0 Flash", ProviderID: "google"},
		{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", ProviderID: "google"},
		{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", ProviderID: "google"},
		{ID: "gemini-1.5-pro", Name: "Gemini 1.5 Pro", ProviderID: "google"},
		{ID: "gemini-1.5-flash", Name: "Gemini 1.5 Flash", ProviderID: "google"},
		// DeepSeek 系列
		{ID: "deepseek-chat", Name: "DeepSeek Chat", ProviderID: "deepseek"},
		{ID: "deepseek-reasoner", Name: "DeepSeek Reasoner", ProviderID: "deepseek"},
		{ID: "deepseek-r1", Name: "DeepSeek R1", ProviderID: "deepseek"},
		{ID: "deepseek-v3", Name: "DeepSeek V3", ProviderID: "deepseek"},
		{ID: "deepseek-v3.1", Name: "DeepSeek V3.1", ProviderID: "deepseek"},
		// Moonshot 系列
		{ID: "moonshot-v1-8k", Name: "Moonshot v1 8K", ProviderID: "moonshot"},
		{ID: "moonshot-v1-32k", Name: "Moonshot v1 32K", ProviderID: "moonshot"},
		{ID: "moonshot-v1-128k", Name: "Moonshot v1 128K", ProviderID: "moonshot"},
		{ID: "kimi-k1-instruct", Name: "Kimi K1 Instruct", ProviderID: "moonshot"},
		{ID: "kimi-k2-instruct", Name: "Kimi K2 Instruct", ProviderID: "moonshot"},
		// MistralAI 系列
		{ID: "mistral-large", Name: "Mistral Large", ProviderID: "mistral"},
		{ID: "mistral-medium", Name: "Mistral Medium", ProviderID: "mistral"},
		{ID: "mistral-small", Name: "Mistral Small", ProviderID: "mistral"},
		{ID: "mistral-small-3.1-24b-instruct", Name: "Mistral Small 3.1 24B Instruct", ProviderID: "mistral"},
	}

	m.mutex.Lock()
	m.rules = defaultRules
	m.modelMetadata = defaultMetadata
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
