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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"veloera/common"
	"veloera/dto"
	"veloera/model"
)

// SystemRenameProcessor 系统规则处理器（使用动态厂商规则）
func SystemRenameProcessor(models []string, includeVendor bool) map[string]string {
	result := make(map[string]string)

	// 获取动态厂商规则
	vendorManager := GetVendorRuleManager()
	vendorRules := vendorManager.GetRules()

	// 日期后缀正则（支持4-8位数字：-0528, -202405, -20240528）
	dateSuffixRe := regexp.MustCompile(`-\d{4,8}$`)

	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}

		renamed := renameModel(model, vendorRules, dateSuffixRe, includeVendor)
		if renamed != model {
			result[renamed] = model
		}
	}

	return result
}

// MultiPassSystemRenameProcessor 多轮系统规则处理器
// 对每个模型进行迭代重命名直到稳定，确保一次处理到位
func MultiPassSystemRenameProcessor(models []string, includeVendor bool) map[string]string {
	const maxPasses = 10

	vendorManager := GetVendorRuleManager()
	vendorRules := vendorManager.GetRules()
	dateSuffixRe := regexp.MustCompile(`-\d{8}$`)

	originToFinal := make(map[string]string)
	seenOriginals := make(map[string]struct{})

	for _, raw := range models {
		original := strings.TrimSpace(raw)
		if original == "" {
			continue
		}
		if _, exists := seenOriginals[original]; exists {
			continue
		}
		seenOriginals[original] = struct{}{}

		current := original
		visited := map[string]struct{}{current: {}}

		for pass := 0; pass < maxPasses; pass++ {
			renamed := renameModel(current, vendorRules, dateSuffixRe, includeVendor)
			renamed = strings.TrimSpace(renamed)

			if renamed == "" || renamed == current {
				break
			}

			if _, loop := visited[renamed]; loop {
				current = renamed
				break
			}

			visited[renamed] = struct{}{}
			current = renamed
		}

		if current != original {
			originToFinal[original] = current
		}
	}

	if len(originToFinal) == 0 {
		return map[string]string{}
	}

	origins := make([]string, 0, len(originToFinal))
	for orig := range originToFinal {
		origins = append(origins, orig)
	}
	sort.Strings(origins)

	finalMapping := make(map[string]string, len(originToFinal))
	for _, orig := range origins {
		finalName := originToFinal[orig]
		if finalName == "" {
			continue
		}
		if _, exists := finalMapping[finalName]; !exists {
			finalMapping[finalName] = orig
		}
	}

	return finalMapping
}

var (
	vendorNameAliases = map[string]string{
		// DeepSeek 系列
		"deepseek":    "DeepSeek",
		"deepseek-ai": "DeepSeek",
		"deepseek ai": "DeepSeek",
		"iflowcn":     "DeepSeek",
		"wandb":       "DeepSeek",
		// MistralAI 系列
		"mistral":    "MistralAI",
		"mistralai":  "MistralAI",
		"mistral ai": "MistralAI",
		"cloudflare": "MistralAI",
		"scaleway":   "MistralAI",
		// Moonshot 系列
		"moonshot":    "Moonshot",
		"moonshotai":  "Moonshot",
		"moonshot ai": "Moonshot",
		"cortecs":     "Moonshot",
		"kimi":        "Moonshot",
		// Google 系列
		"aihubmix":    "Google",
		"hyb-optimal": "Google",
		"hyb optimal": "Google",
		"google":      "Google",
		// 阿里巴巴系列
		"alibaba":    "阿里巴巴",
		"alibaba-cn": "通义千问",
		"qwen":       "阿里巴巴",
		"iic":        "阿里巴巴",
		"musepublic": "阿里巴巴",
		// 百度系列
		"baidu":       "百度",
		"paddlepaddle": "百度",
		// 智谱系列
		"zhipu":   "智谱",
		"zhipuai": "智谱",
		"bigmodel": "智谱",
		// MiniMax 系列
		"minimax": "MiniMax",
		// 商汤/上海AI实验室系列
		"opengvlab":               "上海AI实验室",
		"shanghai_ai_laboratory":  "上海AI实验室",
		"shanghai ai laboratory":  "上海AI实验室",
		"opencompass":             "上海AI实验室",
		// 阶跃星辰系列
		"stepfun":    "阶跃星辰",
		"stepfun-ai": "阶跃星辰",
		"stepfun ai": "阶跃星辰",
		// 其他
		"github copilot": "GitHub",
		"anthropic":      "Anthropic",
		"openai":         "OpenAI",
		"xai":            "xAI",
		"meta":           "Meta",
	}
	vendorNameReplacer = strings.NewReplacer("-", " ", "_", " ", "/", " ")
)

// renameModel 重命名单个模型（使用动态厂商规则和标准化名称）
func renameModel(model string, vendorRules []*VendorRule, dateSuffixRe *regexp.Regexp, includeVendor bool) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return model
	}

	// 移除中文括号和方括号前缀（如 [满血1m]gemini-2.5-pro, gemini-2.5-pro(稳定版)）
	model = removeSpecialPrefixSuffix(model)

	if !includeVendor && isStandardStandaloneName(model) {
		return model
	}

	originalModel := model
	originalVendor := ""
	if idx := strings.Index(model, "/"); idx >= 0 {
		originalVendor = strings.TrimSpace(model[:idx])
	}

	model = strings.TrimPrefix(model, "BigModel/")

	actualModel := model
	if strings.Contains(model, "/") {
		if idx := strings.LastIndex(model, "/"); idx >= 0 {
			actualModel = model[idx+1:]
		}
	}
	actualModel = strings.TrimSpace(actualModel)
	if actualModel == "" {
		return strings.TrimSpace(originalModel)
	}

	if idx := strings.Index(actualModel, ":"); idx >= 0 {
		actualModel = strings.TrimSpace(actualModel[:idx])
	}
	actualModel = strings.TrimSpace(dateSuffixRe.ReplaceAllString(actualModel, ""))
	if actualModel == "" {
		return strings.TrimSpace(originalModel)
	}

	normalizedOriginalVendor := normalizeVendorName(originalVendor, vendorRules)

	vendorManager := GetVendorRuleManager()
	standardName, vendorName, found := vendorManager.FindStandardModelName(actualModel)

	if found {
		cleanedStandard := strings.TrimSpace(standardName)
		normalizedStandardVendor := normalizeVendorName(vendorName, vendorRules)

		if strings.Contains(cleanedStandard, "/") {
			if idx := strings.LastIndex(cleanedStandard, "/"); idx >= 0 {
				modelPart := strings.TrimSpace(cleanedStandard[idx+1:])
				if includeVendor {
					preferredVendor := normalizedOriginalVendor
					if preferredVendor == "" {
						preferredVendor = normalizedStandardVendor
					}
					if preferredVendor == "" {
						vendorPart := strings.TrimSpace(cleanedStandard[:idx])
						preferredVendor = normalizeVendorName(vendorPart, vendorRules)
					}
					if preferredVendor != "" && modelPart != "" {
						return preferredVendor + "/" + modelPart
					}
					return cleanedStandard
				}
				if modelPart != "" {
					return modelPart
				}
			}
			return actualModel
		}

		if includeVendor {
			preferredVendor := normalizedOriginalVendor
			if preferredVendor == "" {
				preferredVendor = normalizedStandardVendor
			}
			if preferredVendor != "" {
				return preferredVendor + "/" + cleanedStandard
			}
		}

		if cleanedStandard != "" {
			return cleanedStandard
		}
		return actualModel
	}

	if includeVendor {
		vendor := normalizedOriginalVendor
		if vendor == "" {
			for _, rule := range vendorRules {
				if rule.Pattern.MatchString(actualModel) {
					vendor = normalizeVendorName(rule.DisplayName, vendorRules)
					if vendor == "" {
						vendor = rule.DisplayName
					}
					break
				}
			}
		}
		if vendor != "" {
			return vendor + "/" + actualModel
		}
	}

	return actualModel
}

// removeSpecialPrefixSuffix 移除特殊前缀和后缀
// 处理格式如: [满血1m]gemini-2.5-pro, gemini-2.5-pro(稳定版), 【prefix】model
func removeSpecialPrefixSuffix(model string) string {
	// 移除方括号前缀 [xxx], 【xxx】
	if idx := strings.Index(model, "]"); idx >= 0 {
		model = strings.TrimSpace(model[idx+1:])
	}
	if idx := strings.Index(model, "】"); idx >= 0 {
		model = strings.TrimSpace(model[idx+len("】"):])
	}

	// 移除圆括号后缀 (xxx), （xxx）
	if idx := strings.Index(model, "("); idx >= 0 {
		model = strings.TrimSpace(model[:idx])
	}
	if idx := strings.Index(model, "（"); idx >= 0 {
		model = strings.TrimSpace(model[:idx])
	}

	return model
}

// AIRenameProcessor AI调用处理器
func AIRenameProcessor(models []string, aiModel string, prompt string) (map[string]string, error) {
	// 1. 构造提示词
	systemPrompt := "你是一个模型名称规范化助手。请严格按照用户提供的规则处理模型列表,只输出JSON格式的映射结果,不要有任何其他文字。"
	userPrompt := fmt.Sprintf("%s\n\n输入模型列表:\n%s", prompt, strings.Join(models, "\n"))

	// 2. 查找支持该模型的渠道
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		return nil, fmt.Errorf("获取渠道列表失败: %w", err)
	}

	var targetChannel *model.Channel
	for _, ch := range channels {
		if ch.Status != common.ChannelStatusEnabled {
			continue
		}
		channelModels := ch.GetModels()
		for _, m := range channelModels {
			if strings.TrimSpace(m) == aiModel {
				targetChannel = ch
				break
			}
		}
		if targetChannel != nil {
			break
		}
	}

	if targetChannel == nil {
		return nil, fmt.Errorf("未找到支持模型 %s 的启用渠道", aiModel)
	}

	// 3. 构造请求
	userContent, _ := json.Marshal(userPrompt)
	messages := []dto.Message{
		{
			Role:    "user",
			Content: userContent,
		},
	}

	request := &dto.GeneralOpenAIRequest{
		Model:    aiModel,
		Messages: messages,
		Stream:   false,
	}

	if systemPrompt != "" {
		systemContent, _ := json.Marshal(systemPrompt)
		request.Messages = append([]dto.Message{
			{
				Role:    "system",
				Content: systemContent,
			},
		}, request.Messages...)
	}

	// 4. 调用AI - 使用简单的HTTP请求,避免依赖relay包的复杂逻辑
	baseURL := targetChannel.GetBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	fullRequestURL := baseURL + "/v1/chat/completions"

	// 发送请求
	key := strings.Split(targetChannel.Key, ",")[0]
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 使用HTTP客户端调用
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": fmt.Sprintf("Bearer %s", key),
	}

	responseBody, statusCode, err := DoHTTPRequest("POST", fullRequestURL, headers, requestBody)
	if err != nil {
		return nil, fmt.Errorf("调用AI失败: %w", err)
	}

	if statusCode != 200 {
		return nil, fmt.Errorf("AI返回错误状态码: %d, 响应: %s", statusCode, string(responseBody))
	}

	// 5. 解析响应
	var aiResponse dto.OpenAITextResponse
	if err := json.Unmarshal(responseBody, &aiResponse); err != nil {
		return nil, fmt.Errorf("解析AI响应失败: %w", err)
	}

	if len(aiResponse.Choices) == 0 {
		return nil, fmt.Errorf("AI响应为空")
	}

	content := aiResponse.Choices[0].Message.StringContent()

	// 6. 提取JSON(可能包含在代码块中)
	jsonContent := extractJSON(content)

	// 7. 解析映射
	var mapping map[string]string
	if err := json.Unmarshal([]byte(jsonContent), &mapping); err != nil {
		return nil, fmt.Errorf("AI返回格式错误,无法解析为JSON映射: %w\n内容: %s", err, jsonContent)
	}

	return mapping, nil
}

// extractJSON 从AI响应中提取JSON内容（可能包含在```json```代码块中）
func extractJSON(content string) string {
	content = strings.TrimSpace(content)

	// 尝试提取代码块中的JSON
	codeBlockRe := regexp.MustCompile("(?s)```(?:json)?\\s*({.*?})\\s*```")
	matches := codeBlockRe.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}

	// 尝试提取纯JSON（查找第一个{到最后一个}）
	firstBrace := strings.Index(content, "{")
	lastBrace := strings.LastIndex(content, "}")
	if firstBrace >= 0 && lastBrace > firstBrace {
		return content[firstBrace : lastBrace+1]
	}

	// 直接返回原内容
	return content
}

// isStandardStandaloneName 判断模型名是否已经是无需处理的标准形式
func isStandardStandaloneName(name string) bool {
	if name == "" {
		return false
	}
	if strings.Contains(name, "/") {
		return false
	}
	if strings.Contains(name, ":") {
		return false
	}
	return true
}

// normalizeVendorName 标准化厂商名称
// 例如: deepseek → DeepSeek, mistralai → MistralAI, moonshotai → Moonshot
func normalizeVendorName(vendor string, vendorRules []*VendorRule) string {
	vendor = strings.TrimSpace(vendor)
	if vendor == "" {
		return ""
	}

	vendorLower := strings.ToLower(vendor)
	vendorNorm := strings.ToLower(vendorNameReplacer.Replace(vendor))

	if normalized, exists := vendorNameAliases[vendorLower]; exists {
		return normalized
	}

	if normalized, exists := vendorNameAliases[vendorNorm]; exists {
		return normalized
	}

	for _, rule := range vendorRules {
		if strings.EqualFold(vendor, strings.ToLower(rule.DisplayName)) {
			return rule.DisplayName
		}
		if strings.EqualFold(vendor, strings.ToLower(rule.ProviderID)) {
			return rule.DisplayName
		}
	}

	if len(vendor) > 0 && unicode.IsLower(rune(vendor[0])) {
		return strings.ToUpper(vendor[:1]) + vendor[1:]
	}

	return vendor
}

// DoHTTPRequest 发送HTTP请求（辅助函数）
func DoHTTPRequest(method, url string, headers map[string]string, body []byte) ([]byte, int, error) {
	client := GetHttpClient()
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return responseBody, resp.StatusCode, nil
}
