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
	"strings"
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

	// 日期后缀正则
	dateSuffixRe := regexp.MustCompile(`-\d{8}$`)

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

// renameModel 重命名单个模型（使用动态厂商规则和标准化名称）
func renameModel(model string, vendorRules []*VendorRule, dateSuffixRe *regexp.Regexp, includeVendor bool) string {
	model = strings.TrimSpace(model)

	// 1. 检查是否已经是标准 owner/model 格式
	//    仅在需要保留厂商前缀（includeVendor=true）时，保持现有格式直接返回
	if includeVendor {
		if slashCount := strings.Count(model, "/"); slashCount == 1 && !strings.Contains(model, ":") {
			parts := strings.Split(model, "/")
			if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
				// 排除特殊前缀情况（如 BigModel/ 开头）
				if parts[0] != "BigModel" && !strings.HasPrefix(model, "Pro/") {
					// 已经是标准格式且需要保留厂商前缀，直接返回
					return model
				}
			}
		}
	}

	// 2. 处理特殊前缀（如 BigModel/GLM-4.5 → GLM-4.5）
	model = strings.TrimPrefix(model, "BigModel/")

	// 3. 提取真实模型名（去除 owner/ 前缀）
	//    对于包含 '/' 的非标准格式模型名，提取最后一个 '/' 后面的部分
	//    例如：deepseek/deepseek-r1-0528-qwen3-8b:free → deepseek-r1-0528-qwen3-8b:free
	actualModel := model
	if strings.Contains(model, "/") {
		if idx := strings.LastIndex(model, "/"); idx >= 0 {
			actualModel = model[idx+1:] // 取最后一个 / 后面的部分作为真实模型名
		}
	}

	// 4. 移除冒号后缀（如 :free、:extended 等）
	if idx := strings.Index(actualModel, ":"); idx >= 0 {
		actualModel = actualModel[:idx]
	}

	// 5. 移除日期后缀
	actualModel = dateSuffixRe.ReplaceAllString(actualModel, "")

	// 6. 尝试在元数据中查找标准化名称
	vendorManager := GetVendorRuleManager()
	standardName, vendorName, found := vendorManager.FindStandardModelName(actualModel)

	if found {
		// 找到标准化名称，使用标准名称
		// 如果标准名称本身包含 '/'（如 deepseek-ai/DeepSeek-V3.1），需要根据 includeVendor 决定是否保留厂商部分
		if strings.Contains(standardName, "/") {
			// 标准名称本身就包含厂商前缀
			if includeVendor {
				// 需要厂商前缀，直接返回标准名称
				return standardName
			} else {
				// 不需要厂商前缀，只取模型名部分（最后一个 / 后面的部分）
				if idx := strings.LastIndex(standardName, "/"); idx >= 0 {
					return standardName[idx+1:]
				}
				return standardName
			}
		} else {
			// 标准名称不包含厂商前缀
			if includeVendor && vendorName != "" {
				return vendorName + "/" + standardName
			}
			return standardName
		}
	}

	// 7. 未找到标准化名称，使用传统逻辑识别厂商
	vendor := ""
	for _, rule := range vendorRules {
		if rule.Pattern.MatchString(actualModel) {
			vendor = rule.DisplayName
			break
		}
	}

	// 8. 组合最终名称
	if includeVendor && vendor != "" {
		return vendor + "/" + actualModel
	}

	// 不包含厂商或未识别到厂商，返回清理后的名称
	return actualModel
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
