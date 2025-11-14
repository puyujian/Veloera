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
package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"veloera/common"
	"veloera/dto"
	relaycommon "veloera/relay/common"
	relayconstant "veloera/relay/constant"
	"veloera/service"
	"veloera/setting/model_setting"

	"github.com/gin-gonic/gin"
)

// OpenaiHandlerV2 增强版非流式响应处理：
// - 保持与原有 OpenaiHandler 相同的转发与计费逻辑
// - 增加开关：允许将“空回复”视为错误，从而触发上层的自动重试与换渠道逻辑
func OpenaiHandlerV2(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.OpenAIErrorWithStatusCode, *dto.Usage) {
	var simpleResponse dto.OpenAITextResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return service.OpenAIErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError), nil
	}
	if common.DebugEnabled {
		common.LogInfo(c, "responseBody: "+string(responseBody))
	}
	if err = resp.Body.Close(); err != nil {
		return service.OpenAIErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError), nil
	}

	if err = common.DecodeJson(responseBody, &simpleResponse); err != nil {
		return service.OpenAIErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError), nil
	}
	if simpleResponse.Error != nil && simpleResponse.Error.Type != "" {
		return &dto.OpenAIErrorWithStatusCode{
			Error:      *simpleResponse.Error,
			StatusCode: resp.StatusCode,
		}, nil
	}

	// 保存输入输出内容到 info.Other，供日志记录使用
	if info.Other == nil {
		info.Other = make(map[string]interface{})
	}

	// 提取输入内容（最后一条用户消息）和上下文（其他所有非 system 消息）
	if messages, ok := info.PromptMessages.([]interface{}); ok && len(messages) > 0 {
		var systemPrompt string
		var contextMessages []interface{}
		var userMessage interface{}
		lastUserMessageIndex := -1

		// 先找出最后一条 user 消息的索引
		for i := len(messages) - 1; i >= 0; i-- {
			if msgMap, ok := messages[i].(map[string]interface{}); ok {
				if role, exists := msgMap["role"]; exists && role == "user" {
					lastUserMessageIndex = i
					break
				}
			}
		}

		// 再遍历处理所有消息
		for i, msg := range messages {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				if role, exists := msgMap["role"]; exists {
					if role == "system" {
						// 如果是 system 消息，保存其内容
						if content, hasContent := msgMap["content"]; hasContent && content != nil {
							if systemPrompt == "" {
								systemPrompt = fmt.Sprintf("%v", content)
							} else {
								systemPrompt += "\n" + fmt.Sprintf("%v", content)
							}
						}
					} else if i == lastUserMessageIndex {
						// 如果是最后一条 user 消息
						userMessage = msgMap
					} else {
						// 其他非 system 消息作为上下文
						contextMessages = append(contextMessages, msgMap)
					}
				}
			}
		}

		// 保存处理后的数据
		if systemPrompt != "" {
			info.Other["system_prompt"] = systemPrompt
		}
		info.Other["context"] = contextMessages
		if userMessage != nil {
			info.Other["input_content"] = userMessage
		} else {
			// 如果没有找到 user 消息，保存最后一条消息作为输入
			if len(messages) > 0 {
				info.Other["input_content"] = messages[len(messages)-1]
			}
		}
	} else {
		info.Other["input_content"] = info.PromptMessages // 备用方案，保存全部输入内容
	}

	// 提取输出内容
	var outputContent string
	for _, choice := range simpleResponse.Choices {
		outputContent += choice.Message.StringContent() + choice.Message.ReasoningContent + choice.Message.Reasoning
	}
	info.Other["output_content"] = outputContent // 保存输出内容

	// 仅对文本补全类请求检查空回复
	isEmptyResponse := false
	if info.RelayMode == relayconstant.RelayModeChatCompletions ||
		info.RelayMode == relayconstant.RelayModeCompletions {
		isEmptyResponse = true
		for _, choice := range simpleResponse.Choices {
			content := choice.Message.StringContent() + choice.Message.ReasoningContent + choice.Message.Reasoning
			if !common.IsEmptyOrWhitespace(content) {
				isEmptyResponse = false
				break
			}
		}
	}

	// 如果响应是空的，根据配置决定是否视为错误（用于触发自动重试与切换渠道）
	if isEmptyResponse && model_setting.ShouldTreatEmptyResponseAsError() {
		return service.OpenAIErrorWrapperLocal(
			fmt.Errorf("empty response from upstream"),
			"empty_response",
			http.StatusInternalServerError,
		), nil
	}

	// 根据 RelayFormat 进行格式转换
	switch info.RelayFormat {
	case relaycommon.RelayFormatOpenAI:
		// 原始 OpenAI 格式，直接透传
	case relaycommon.RelayFormatClaude:
		claudeResp := service.ResponseOpenAI2Claude(&simpleResponse, info)
		claudeRespStr, err := json.Marshal(claudeResp)
		if err != nil {
			return service.OpenAIErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError), nil
		}
		responseBody = claudeRespStr
	}

	// 将响应写回给客户端
	resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))
	for k, v := range resp.Header {
		if len(v) > 0 {
			c.Writer.Header().Set(k, v[0])
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err = io.Copy(c.Writer, resp.Body); err != nil {
		// 避免将复制错误直接暴露给上游调用者，只记录日志
		common.SysError("error copying response body: " + err.Error())
	}
	_ = resp.Body.Close()

	// 如果是空回复且未被视为错误，则返回零使用量（兼容旧行为：空回复不计费）
	if isEmptyResponse {
		zeroUsage := &dto.Usage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		}
		return nil, zeroUsage
	}

	// 正常计算 usage
	if info.RelayMode == relayconstant.RelayModeEmbeddings {
		usage := fillEmbeddingUsage(info, &simpleResponse)
		simpleResponse.Usage = usage
	} else {
		// 对于非 embedding 请求，如果上游未返回 usage，则本地计算
		if simpleResponse.Usage.TotalTokens == 0 ||
			(simpleResponse.Usage.PromptTokens == 0 && simpleResponse.Usage.CompletionTokens == 0) {
			completionTokens := 0
			for _, choice := range simpleResponse.Choices {
				ctkm, _ := service.CountTextToken(
					choice.Message.StringContent()+choice.Message.ReasoningContent+choice.Message.Reasoning,
					info.UpstreamModelName,
				)
				completionTokens += ctkm
			}
			simpleResponse.Usage = dto.Usage{
				PromptTokens:     info.PromptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      info.PromptTokens + completionTokens,
			}
		}
	}

	return nil, &simpleResponse.Usage
}

