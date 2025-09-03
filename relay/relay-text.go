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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
	"veloera/common"
	"veloera/constant"
	"veloera/dto"
	"veloera/model"
	gemini "veloera/relay/channel/gemini"
	openai "veloera/relay/channel/openai"
	relaycommon "veloera/relay/common"
	relayconstant "veloera/relay/constant"
	"veloera/relay/helper"
	"veloera/service"
	"veloera/setting"
	"veloera/setting/model_setting"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/shopspring/decimal"

	"github.com/gin-gonic/gin"
)

func getAndValidateTextRequest(c *gin.Context, relayInfo *relaycommon.RelayInfo) (*dto.GeneralOpenAIRequest, error) {
	textRequest := &dto.GeneralOpenAIRequest{}
	err := common.UnmarshalBodyReusable(c, textRequest)
	if err != nil {
		return nil, err
	}
	if relayInfo.RelayMode == relayconstant.RelayModeModerations && textRequest.Model == "" {
		textRequest.Model = "text-moderation-latest"
	}
	if relayInfo.RelayMode == relayconstant.RelayModeEmbeddings && textRequest.Model == "" {
		textRequest.Model = c.Param("model")
	}

	if textRequest.MaxTokens > math.MaxInt32/2 {
		return nil, errors.New("max_tokens is invalid")
	}
	if textRequest.Model == "" {
		return nil, errors.New("model is required")
	}
	switch relayInfo.RelayMode {
	case relayconstant.RelayModeCompletions:
		if textRequest.Prompt == "" {
			return nil, errors.New("field prompt is required")
		}
	case relayconstant.RelayModeChatCompletions:
		if len(textRequest.Messages) == 0 {
			return nil, errors.New("field messages is required")
		}
	case relayconstant.RelayModeEmbeddings:
	case relayconstant.RelayModeModerations:
		if textRequest.Input == nil || textRequest.Input == "" {
			return nil, errors.New("field input is required")
		}
	case relayconstant.RelayModeEdits:
		if textRequest.Instruction == "" {
			return nil, errors.New("field instruction is required")
		}
	}
	relayInfo.IsStream = textRequest.Stream

	// 保存请求消息内容，用于记录日志
	switch relayInfo.RelayMode {
	case relayconstant.RelayModeChatCompletions:
		relayInfo.PromptMessages = textRequest.Messages
	case relayconstant.RelayModeCompletions:
		relayInfo.PromptMessages = textRequest.Prompt
	}

	return textRequest, nil
}

func prependSystemPromptIfNeeded(c *gin.Context, textRequest *dto.GeneralOpenAIRequest, relayInfo *relaycommon.RelayInfo) {
	channelSystemPrompt := c.GetString("system_prompt")
	if channelSystemPrompt == "" {
		return
	}

	// Only process chat completions
	if relayInfo.RelayMode != relayconstant.RelayModeChatCompletions {
		return
	}

	// Find existing system message (should be first if exists)
	var existingSystemMessage *dto.Message

	for i, message := range textRequest.Messages {
		if message.Role == "system" {
			existingSystemMessage = &textRequest.Messages[i]
			break
		}
	}

	// Prepend channel system prompt
	if existingSystemMessage != nil {
		// If user already has a system message, prepend channel's system prompt
		existingContent := existingSystemMessage.StringContent()
		newSystemContent := channelSystemPrompt + "\n\n" + existingContent
		newContentBytes, _ := json.Marshal(newSystemContent)
		existingSystemMessage.Content = newContentBytes
	} else {
		// If no existing system message, create a new one and prepend to messages
		newContentBytes, _ := json.Marshal(channelSystemPrompt)
		newSystemMessage := dto.Message{
			Role:    "system",
			Content: newContentBytes,
		}
		textRequest.Messages = append([]dto.Message{newSystemMessage}, textRequest.Messages...)
	}
}

func TextHelper(c *gin.Context) (openaiErr *dto.OpenAIErrorWithStatusCode) {

	relayInfo := relaycommon.GenRelayInfo(c)

	// get & validate textRequest 获取并验证文本请求
	textRequest, err := getAndValidateTextRequest(c, relayInfo)
	if err != nil {
		common.LogError(c, fmt.Sprintf("getAndValidateTextRequest failed: %s", err.Error()))
		return service.OpenAIErrorWrapperLocal(err, "invalid_text_request", http.StatusBadRequest)
	}

	// Prepend channel system prompt if configured
	prependSystemPromptIfNeeded(c, textRequest, relayInfo)

	tokenGroup := c.GetString("token_group")
	if setting.ShouldCheckPromptSensitiveWithGroup(tokenGroup) {
		words, err := checkRequestSensitive(textRequest, relayInfo)
		if err != nil {
			common.LogWarn(c, fmt.Sprintf("user sensitive words detected: %s", strings.Join(words, ", ")))
			return service.OpenAIErrorWrapperLocal(err, "sensitive_words_detected", http.StatusBadRequest)
		}
	}

	err = helper.ModelMappedHelper(c, relayInfo)
	if err != nil {
		return service.OpenAIErrorWrapperLocal(err, "model_mapped_error", http.StatusInternalServerError)
	}

	textRequest.Model = relayInfo.UpstreamModelName

	// 获取 promptTokens，如果上下文中已经存在，则直接使用
	var promptTokens int
	if value, exists := c.Get("prompt_tokens"); exists {
		promptTokens = value.(int)
		relayInfo.PromptTokens = promptTokens
	} else {
		promptTokens, err = getPromptTokens(textRequest, relayInfo)
		// count messages token error 计算promptTokens错误
		if err != nil {
			return service.OpenAIErrorWrapper(err, "count_token_messages_failed", http.StatusInternalServerError)
		}
		c.Set("prompt_tokens", promptTokens)
	}

	priceData, err := helper.ModelPriceHelper(c, relayInfo, promptTokens, int(math.Max(float64(textRequest.MaxTokens), float64(textRequest.MaxCompletionTokens))))
	if err != nil {
		return service.OpenAIErrorWrapperLocal(err, "model_price_error", http.StatusInternalServerError)
	}

	// pre-consume quota 预消耗配额
	preConsumedQuota, userQuota, openaiErr := preConsumeQuota(c, priceData.ShouldPreConsumedQuota, relayInfo)
	if openaiErr != nil {
		return openaiErr
	}
	defer func() {
		if openaiErr != nil {
			returnPreConsumedQuota(c, relayInfo, userQuota, preConsumedQuota)
		}
	}()
	includeUsage := false
	// 判断用户是否需要返回使用情况
	if textRequest.StreamOptions != nil && textRequest.StreamOptions.IncludeUsage {
		includeUsage = true
	}

	// 如果不支持StreamOptions，将StreamOptions设置为nil
	if !relayInfo.SupportStreamOptions || !textRequest.Stream {
		textRequest.StreamOptions = nil
	} else {
		// 如果支持StreamOptions，且请求中没有设置StreamOptions，根据配置文件设置StreamOptions
		if constant.ForceStreamOption {
			textRequest.StreamOptions = &dto.StreamOptions{
				IncludeUsage: true,
			}
		}
	}

	if includeUsage {
		relayInfo.ShouldIncludeUsage = true
	}

	streamSupport := ""
	if v, ok := relayInfo.ChannelSetting[constant.ChannelSettingStreamSupport]; ok {
		if str, ok2 := v.(string); ok2 {
			streamSupport = strings.ToUpper(str)
		}
	}
	pseudoStream := textRequest.Stream && streamSupport == constant.StreamSupportNonStreamOnly
	var stopHeartbeat func()
	if pseudoStream {
		textRequest.Stream = false
		relayInfo.IsStream = false
		helper.SetEventStreamHeaders(c)
		relayInfo.SetFirstResponseTime()
		_ = helper.WaitData(c)
		stopHeartbeat = helper.StartWaitingHeartbeat(c, 5*time.Second)
	}

	adaptor := GetAdaptor(relayInfo.ApiType)
	if adaptor == nil {
		return service.OpenAIErrorWrapperLocal(fmt.Errorf("invalid api type: %d", relayInfo.ApiType), "invalid_api_type", http.StatusBadRequest)
	}
	adaptor.Init(relayInfo)
	var requestBody io.Reader

	if model_setting.GetGlobalSettings().PassThroughRequestEnabled {
		body, err := common.GetRequestBody(c)
		if err != nil {
			return service.OpenAIErrorWrapperLocal(err, "get_request_body_failed", http.StatusInternalServerError)
		}
		requestBody = bytes.NewBuffer(body)
	} else {
		convertedRequest, err := adaptor.ConvertOpenAIRequest(c, relayInfo, textRequest)
		if err != nil {
			return service.OpenAIErrorWrapperLocal(err, "convert_request_failed", http.StatusInternalServerError)
		}
		jsonData, err := json.Marshal(convertedRequest)
		if err != nil {
			return service.OpenAIErrorWrapperLocal(err, "json_marshal_failed", http.StatusInternalServerError)
		}

		// apply param override
		if len(relayInfo.ParamOverride) > 0 {
			reqMap := make(map[string]interface{})
			err = json.Unmarshal(jsonData, &reqMap)
			if err != nil {
				return service.OpenAIErrorWrapperLocal(err, "param_override_unmarshal_failed", http.StatusInternalServerError)
			}
			for key, value := range relayInfo.ParamOverride {
				reqMap[key] = value
			}
			jsonData, err = json.Marshal(reqMap)
			if err != nil {
				return service.OpenAIErrorWrapperLocal(err, "param_override_marshal_failed", http.StatusInternalServerError)
			}
		}

		if common.DebugEnabled {
			println("requestBody: ", string(jsonData))
		}
		requestBody = bytes.NewBuffer(jsonData)
	}

	var httpResp *http.Response
	resp, err := adaptor.DoRequest(c, relayInfo, requestBody)
	if err != nil {
		if pseudoStream && stopHeartbeat != nil {
			stopHeartbeat()
		}
		return service.OpenAIErrorWrapper(err, "do_request_failed", http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	if resp != nil {
		httpResp = resp.(*http.Response)
		relayInfo.IsStream = relayInfo.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
		if httpResp.StatusCode != http.StatusOK {
			if pseudoStream && stopHeartbeat != nil {
				stopHeartbeat()
			}
			openaiErr = service.RelayErrorHandler(httpResp, false)
			// reset status code 重置状态码
			service.ResetStatusCode(openaiErr, statusCodeMappingStr)
			return openaiErr
		}
	}

	var usage any
	if pseudoStream {
		switch relayInfo.ChannelType {
		case common.ChannelTypeOpenAI:
			openaiErr, usage = openai.OpenaiPseudoStreamHandler(c, httpResp, relayInfo)
		case common.ChannelTypeGemini:
			openaiErr, usage = gemini.GeminiChatPseudoStreamHandler(c, httpResp, relayInfo)
		default:
			usage, openaiErr = adaptor.DoResponse(c, httpResp, relayInfo)
		}
		// 对于伪流式响应，如果没有错误则标记响应已写入
		if openaiErr == nil {
			c.Set("response_written", true)
		}
	} else {
		usage, openaiErr = adaptor.DoResponse(c, httpResp, relayInfo)
		// 对于非流式响应，如果没有错误则标记响应已写入
		if openaiErr == nil {
			c.Set("response_written", true)
		}
	}
	if openaiErr != nil {
		if pseudoStream && stopHeartbeat != nil {
			stopHeartbeat()
		}
		// reset status code 重置状态码
		service.ResetStatusCode(openaiErr, statusCodeMappingStr)
		return openaiErr
	}

	if strings.HasPrefix(relayInfo.OriginModelName, "gpt-4o-audio") {
		service.PostAudioConsumeQuota(c, relayInfo, usage.(*dto.Usage), preConsumedQuota, userQuota, priceData, "")
	} else {
		postConsumeQuota(c, relayInfo, usage.(*dto.Usage), preConsumedQuota, userQuota, priceData, "")
	}
	if pseudoStream && stopHeartbeat != nil {
		stopHeartbeat()
	}
	return nil
}

func getPromptTokens(textRequest *dto.GeneralOpenAIRequest, info *relaycommon.RelayInfo) (int, error) {
	var promptTokens int
	var err error
	switch info.RelayMode {
	case relayconstant.RelayModeChatCompletions:
		promptTokens, err = service.CountTokenChatRequest(info, *textRequest)
	case relayconstant.RelayModeCompletions:
		promptTokens, err = service.CountTokenInput(textRequest.Prompt, textRequest.Model)
	case relayconstant.RelayModeModerations:
		promptTokens, err = service.CountTokenInput(textRequest.Input, textRequest.Model)
	case relayconstant.RelayModeEmbeddings:
		promptTokens, err = service.CountTokenInput(textRequest.Input, textRequest.Model)
	default:
		err = errors.New("unknown relay mode")
		promptTokens = 0
	}
	info.PromptTokens = promptTokens
	return promptTokens, err
}

func checkRequestSensitive(textRequest *dto.GeneralOpenAIRequest, info *relaycommon.RelayInfo) ([]string, error) {
	var err error
	var words []string
	switch info.RelayMode {
	case relayconstant.RelayModeChatCompletions:
		words, err = service.CheckSensitiveMessages(textRequest.Messages)
	case relayconstant.RelayModeCompletions:
		words, err = service.CheckSensitiveInput(textRequest.Prompt)
	case relayconstant.RelayModeModerations:
		words, err = service.CheckSensitiveInput(textRequest.Input)
	case relayconstant.RelayModeEmbeddings:
		words, err = service.CheckSensitiveInput(textRequest.Input)
	}
	return words, err
}

// 预扣费并返回用户剩余配额
func preConsumeQuota(c *gin.Context, preConsumedQuota int, relayInfo *relaycommon.RelayInfo) (int, int, *dto.OpenAIErrorWithStatusCode) {
	userQuota, err := model.GetUserQuota(relayInfo.UserId, false)
	if err != nil {
		return 0, 0, service.OpenAIErrorWrapperLocal(err, "get_user_quota_failed", http.StatusInternalServerError)
	}
	if userQuota <= 0 {
		return 0, 0, service.OpenAIErrorWrapperLocal(errors.New("user quota is not enough"), "insufficient_user_quota", http.StatusForbidden)
	}
	if userQuota-preConsumedQuota < 0 {
		return 0, 0, service.OpenAIErrorWrapperLocal(fmt.Errorf("chat pre-consumed quota failed, user quota: %s, need quota: %s", common.FormatQuota(userQuota), common.FormatQuota(preConsumedQuota)), "insufficient_user_quota", http.StatusForbidden)
	}
	relayInfo.UserQuota = userQuota
	if userQuota > 100*preConsumedQuota {
		// 用户额度充足，判断令牌额度是否充足
		if !relayInfo.TokenUnlimited {
			// 非无限令牌，判断令牌额度是否充足
			tokenQuota := c.GetInt("token_quota")
			if tokenQuota > 100*preConsumedQuota {
				// 令牌额度充足，信任令牌
				preConsumedQuota = 0
				common.LogInfo(c, fmt.Sprintf("user %d quota %s and token %d quota %d are enough, trusted and no need to pre-consume", relayInfo.UserId, common.FormatQuota(userQuota), relayInfo.TokenId, tokenQuota))
			}
		} else {
			// in this case, we do not pre-consume quota
			// because the user has enough quota
			preConsumedQuota = 0
			common.LogInfo(c, fmt.Sprintf("user %d with unlimited token has enough quota %s, trusted and no need to pre-consume", relayInfo.UserId, common.FormatQuota(userQuota)))
		}
	}

	if preConsumedQuota > 0 {
		err := service.PreConsumeTokenQuota(relayInfo, preConsumedQuota)
		if err != nil {
			return 0, 0, service.OpenAIErrorWrapperLocal(err, "pre_consume_token_quota_failed", http.StatusForbidden)
		}
		err = model.DecreaseUserQuota(relayInfo.UserId, preConsumedQuota)
		if err != nil {
			return 0, 0, service.OpenAIErrorWrapperLocal(err, "decrease_user_quota_failed", http.StatusInternalServerError)
		}
	}
	return preConsumedQuota, userQuota, nil
}

func returnPreConsumedQuota(c *gin.Context, relayInfo *relaycommon.RelayInfo, userQuota int, preConsumedQuota int) {
	if preConsumedQuota != 0 {
		gopool.Go(func() {
			relayInfoCopy := *relayInfo

			err := service.PostConsumeQuota(&relayInfoCopy, -preConsumedQuota, 0, false)
			if err != nil {
				common.SysError("error return pre-consumed quota: " + err.Error())
			}
		})
	}
}

func TokenCountHelper(c *gin.Context) (openaiErr *dto.OpenAIErrorWithStatusCode) {
	relayInfo := relaycommon.GenRelayInfo(c)

	// get & validate textRequest for token counting
	textRequest, err := getAndValidateTokenCountRequest(c, relayInfo)
	if err != nil {
		common.LogError(c, fmt.Sprintf("getAndValidateTokenCountRequest failed: %s", err.Error()))
		// Return Claude API-formatted error for validation failures (Requirement 6.2)
		return createTokenCountValidationError(err.Error())
	}

	// Apply model mapping
	err = helper.ModelMappedHelper(c, relayInfo)
	if err != nil {
		return createTokenCountValidationError(fmt.Sprintf("Model mapping failed: %s", err.Error()))
	}

	textRequest.Model = relayInfo.UpstreamModelName

	// Get adaptor for the channel
	adaptor := GetAdaptor(relayInfo.ApiType)
	if adaptor == nil {
		return createTokenCountValidationError(fmt.Sprintf("Unsupported channel type for model '%s'", relayInfo.OriginModelName))
	}
	adaptor.Init(relayInfo)

	// Convert request to channel-specific format
	convertedRequest, err := adaptor.ConvertOpenAIRequest(c, relayInfo, textRequest)
	if err != nil {
		return createTokenCountValidationError(fmt.Sprintf("Request conversion failed: %s", err.Error()))
	}

	jsonData, err := json.Marshal(convertedRequest)
	if err != nil {
		return service.OpenAIErrorWrapperLocal(err, "json_marshal_failed", http.StatusInternalServerError)
	}

	if common.DebugEnabled {
		println("token count requestBody: ", string(jsonData))
	}

	requestBody := bytes.NewBuffer(jsonData)

	// Make request to upstream
	resp, err := adaptor.DoRequest(c, relayInfo, requestBody)
	if err != nil {
		// Enhanced error handling for upstream request failures
		if strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "timeout") {
			return createTokenCountServiceError("Service temporarily unavailable. Please try again later.")
		}
		return service.OpenAIErrorWrapper(err, "do_request_failed", http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	if resp != nil {
		httpResp := resp.(*http.Response)
		if httpResp.StatusCode != http.StatusOK {
			openaiErr = service.RelayErrorHandler(httpResp, false)
			// reset status code
			service.ResetStatusCode(openaiErr, statusCodeMappingStr)
			return openaiErr
		}

		// Handle response - this will write directly to the client
		_, openaiErr = adaptor.DoResponse(c, httpResp, relayInfo)
		if openaiErr != nil {
			// reset status code
			service.ResetStatusCode(openaiErr, statusCodeMappingStr)
			return openaiErr
		}

		// Mark response as written
		c.Set("response_written", true)
	}

	// No usage tracking for token counting - this is the key requirement
	// Token counting requests should not consume user quotas or be logged as billable usage
	return nil
}

// createTokenCountValidationError creates a Claude API-formatted validation error
func createTokenCountValidationError(message string) *dto.OpenAIErrorWithStatusCode {
	return &dto.OpenAIErrorWithStatusCode{
		Error: dto.OpenAIError{
			Type:    "invalid_request_error",
			Code:    "invalid_request_error",
			Message: message,
		},
		StatusCode: http.StatusBadRequest,
	}
}

// createTokenCountServiceError creates a Claude API-formatted service error
func createTokenCountServiceError(message string) *dto.OpenAIErrorWithStatusCode {
	return &dto.OpenAIErrorWithStatusCode{
		Error: dto.OpenAIError{
			Type:    "api_error",
			Code:    "api_error", 
			Message: message,
		},
		StatusCode: http.StatusServiceUnavailable,
	}
}

func getAndValidateTokenCountRequest(c *gin.Context, relayInfo *relaycommon.RelayInfo) (*dto.GeneralOpenAIRequest, error) {
	textRequest := &dto.GeneralOpenAIRequest{}
	err := common.UnmarshalBodyReusable(c, textRequest)
	if err != nil {
		return nil, err
	}

	if textRequest.Model == "" {
		return nil, errors.New("model is required")
	}

	// Validate that the model supports token counting
	if !dto.IsTokenCountSupportedModel(textRequest.Model) {
		return nil, fmt.Errorf("model '%s' does not support token counting. Supported models: %s", 
			textRequest.Model, strings.Join(dto.TokenCountSupportedModels, ", "))
	}

	// For token counting, we need messages
	if len(textRequest.Messages) == 0 {
		return nil, errors.New("field messages is required")
	}

	// Validate message content for unsupported media types (Requirement 3.4)
	err = validateTokenCountMessageContent(textRequest.Messages)
	if err != nil {
		return nil, err
	}

	// Token counting is never streaming
	relayInfo.IsStream = false
	textRequest.Stream = false

	// Save request messages for logging (but not for billing)
	relayInfo.PromptMessages = textRequest.Messages

	return textRequest, nil
}

// validateTokenCountMessageContent validates that message content types are supported for token counting
func validateTokenCountMessageContent(messages []dto.Message) error {
	for i, message := range messages {
		// Use the existing ParseContent method to get media content
		contentArray := message.ParseContent()
		if len(contentArray) > 0 {
			// This is a content array, validate each content block
			for j, content := range contentArray {
				if content.Type != "text" && content.Type != "image" {
					// Claude supports text and image content for token counting
					// Other types like audio, video, etc. are not supported
					return fmt.Errorf("unsupported media type '%s' in message %d, content block %d. Token counting supports text and image content only", 
						content.Type, i+1, j+1)
				}
				
				// Additional validation for image content
				if content.Type == "image" {
					imageMedia := content.GetImageMedia()
					if imageMedia == nil {
						return fmt.Errorf("image content in message %d, content block %d is missing image information", i+1, j+1)
					}
					// Note: Claude API handles various image formats, so we don't need to restrict to base64 only
				}
				
				// Additional validation for unsupported audio content
				if content.Type == "input_audio" {
					return fmt.Errorf("audio content is not supported for token counting in message %d, content block %d", i+1, j+1)
				}
			}
		}
	}
	return nil
}

func postConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo,
	usage *dto.Usage, preConsumedQuota int, userQuota int, priceData helper.PriceData, extraContent string) {
	if usage == nil {
		usage = &dto.Usage{
			PromptTokens:     relayInfo.PromptTokens,
			CompletionTokens: 0,
			TotalTokens:      relayInfo.PromptTokens,
		}
		extraContent += "（可能是请求出错）"
	}
	useTimeSeconds := time.Now().Unix() - relayInfo.StartTime.Unix()
	promptTokens := usage.PromptTokens
	cacheTokens := usage.PromptTokensDetails.CachedTokens
	completionTokens := usage.CompletionTokens
	modelName := relayInfo.OriginModelName

	tokenName := ctx.GetString("token_name")
	completionRatio := priceData.CompletionRatio
	cacheRatio := priceData.CacheRatio
	modelRatio := priceData.ModelRatio
	groupRatio := priceData.GroupRatio
	modelPrice := priceData.ModelPrice

	// Convert values to decimal for precise calculation
	dPromptTokens := decimal.NewFromInt(int64(promptTokens))
	dCacheTokens := decimal.NewFromInt(int64(cacheTokens))
	dCompletionTokens := decimal.NewFromInt(int64(completionTokens))
	dCompletionRatio := decimal.NewFromFloat(completionRatio)
	dCacheRatio := decimal.NewFromFloat(cacheRatio)
	dModelRatio := decimal.NewFromFloat(modelRatio)
	dGroupRatio := decimal.NewFromFloat(groupRatio)
	dModelPrice := decimal.NewFromFloat(modelPrice)
	dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)

	ratio := dModelRatio.Mul(dGroupRatio)

	var quotaCalculateDecimal decimal.Decimal
	if !priceData.UsePrice {
		nonCachedTokens := dPromptTokens.Sub(dCacheTokens)
		cachedTokensWithRatio := dCacheTokens.Mul(dCacheRatio)
		promptQuota := nonCachedTokens.Add(cachedTokensWithRatio)
		completionQuota := dCompletionTokens.Mul(dCompletionRatio)

		quotaCalculateDecimal = promptQuota.Add(completionQuota).Mul(ratio)

		if !ratio.IsZero() && quotaCalculateDecimal.LessThanOrEqual(decimal.Zero) {
			quotaCalculateDecimal = decimal.NewFromInt(1)
		}
	} else {
		quotaCalculateDecimal = dModelPrice.Mul(dQuotaPerUnit).Mul(dGroupRatio)
	}

	quota := int(quotaCalculateDecimal.Round(0).IntPart())
	totalTokens := promptTokens + completionTokens

	var logContent string
	if !priceData.UsePrice {
		logContent = fmt.Sprintf("模型倍率 %.2f，补全倍率 %.2f，分组倍率 %.2f", modelRatio, completionRatio, groupRatio)
	} else {
		logContent = fmt.Sprintf("模型价格 %.2f，分组倍率 %.2f", modelPrice, groupRatio)
	}

	// record all the consume log even if quota is 0
	if totalTokens == 0 {
		// in this case, must be some error happened
		// we cannot just return, because we may have to return the pre-consumed quota
		quota = 0
		logContent += fmt.Sprintf("（可能是上游超时）")
		common.LogError(ctx, fmt.Sprintf("total tokens is 0, cannot consume quota, userId %d, channelId %d, "+
			"tokenId %d, model %s， pre-consumed quota %d", relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, modelName, preConsumedQuota))
	} else {
		model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, quota)
		model.UpdateChannelUsedQuota(relayInfo.ChannelId, quota)
	}

	quotaDelta := quota - preConsumedQuota
	if quotaDelta != 0 {
		err := service.PostConsumeQuota(relayInfo, quotaDelta, preConsumedQuota, true)
		if err != nil {
			common.LogError(ctx, "error consuming token remain quota: "+err.Error())
		}
	}

	logModel := modelName
	if strings.HasPrefix(logModel, "gpt-4-gizmo") {
		logModel = "gpt-4-gizmo-*"
		logContent += fmt.Sprintf("，模型 %s", modelName)
	}
	if strings.HasPrefix(logModel, "gpt-4o-gizmo") {
		logModel = "gpt-4o-gizmo-*"
		logContent += fmt.Sprintf("，模型 %s", modelName)
	}
	if extraContent != "" {
		logContent += ", " + extraContent
	}
	other := service.GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, cacheTokens, cacheRatio, modelPrice)
	model.RecordConsumeLog(ctx, relayInfo.UserId, relayInfo.ChannelId, promptTokens, completionTokens, logModel,
		tokenName, quota, logContent, relayInfo.TokenId, userQuota, int(useTimeSeconds), relayInfo.IsStream, relayInfo.Group, other)
}
