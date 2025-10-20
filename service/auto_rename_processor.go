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
	"veloera/common"
	"veloera/dto"
	"veloera/middleware"
	"veloera/model"
)

// SystemRenameProcessor 系统规则处理器（使用动态厂商规则）
func SystemRenameProcessor(models []string) map[string]string {
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

		renamed := renameModel(model, vendorRules, dateSuffixRe)
		if renamed != model {
			result[renamed] = model
		}
	}

	return result
}

// renameModel 重命名单个模型（使用动态厂商规则）
func renameModel(model string, vendorRules []*VendorRule, dateSuffixRe *regexp.Regexp) string {
	original := model

	// 1. 处理特殊前缀（如 BigModel/GLM-4.5 → GLM-4.5）
	model = strings.TrimPrefix(model, "BigModel/")
	model = strings.TrimSpace(model)

	// 2. 移除日期后缀
	model = dateSuffixRe.ReplaceAllString(model, "")

	// 3. 识别厂商（使用动态规则）
	vendor := ""
	for _, rule := range vendorRules {
		if rule.Pattern.MatchString(model) {
			vendor = rule.DisplayName
			break
		}
	}

	// 4. 组合最终名称
	if vendor != "" {
		return vendor + "/" + model
	}

	// 如果没有识别到厂商，返回清理后的名称
	return model
}

// AIRenameProcessor AI调用处理器
func AIRenameProcessor(models []string, aiModel string, prompt string) (map[string]string, error) {
	// 1. 构造提示词
	systemPrompt := "你是一个模型名称规范化助手。请严格按照用户提供的规则处理模型列表，只输出JSON格式的映射结果，不要有任何其他文字。"
	userPrompt := fmt.Sprintf("%s\n\n输入模型列表：\n%s", prompt, strings.Join(models, "\n"))

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
	messages := []dto.Message{
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	request := &dto.GeneralOpenAIRequest{
		Model:    aiModel,
		Messages: messages,
		Stream:   false,
	}

	if systemPrompt != "" {
		request.Messages = append([]dto.Message{
			{
				Role:    "system",
				Content: systemPrompt,
			},
		}, request.Messages...)
	}

	// 4. 调用AI
	adaptor := middleware.GetAdaptor(targetChannel.Type)
	if adaptor == nil {
		return nil, fmt.Errorf("不支持的渠道类型: %d", targetChannel.Type)
	}

	// 转换请求
	convertedRequest, err := adaptor.ConvertRequest(targetChannel, request)
	if err != nil {
		return nil, fmt.Errorf("转换请求失败: %w", err)
	}

	// 获取完整URL
	fullRequestURL, err := adaptor.GetRequestURL(targetChannel)
	if err != nil {
		return nil, fmt.Errorf("获取请求URL失败: %w", err)
	}

	// 发送请求
	key := strings.Split(targetChannel.Key, ",")[0]
	requestBody, err := json.Marshal(convertedRequest)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 使用HTTP客户端调用
	headers := adaptor.GetRequestHeaders(targetChannel)
	headers["Authorization"] = fmt.Sprintf("Bearer %s", key)

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

	// 6. 提取JSON（可能包含在代码块中）
	jsonContent := extractJSON(content)

	// 7. 解析映射
	var mapping map[string]string
	if err := json.Unmarshal([]byte(jsonContent), &mapping); err != nil {
		return nil, fmt.Errorf("AI返回格式错误，无法解析为JSON映射: %w\n内容: %s", err, jsonContent)
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
	client := GetDefaultHTTPClient()
	req, err := NewHTTPRequest(method, url, body)
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

	responseBody, err := ReadResponseBody(resp)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return responseBody, resp.StatusCode, nil
}
