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
	"net/http"
	"strings"
	"veloera/common"
	"veloera/dto"
	"veloera/model"
	relaycommon "veloera/relay/common"
	"veloera/relay/helper"
	"veloera/service"
	"veloera/setting"

	"github.com/gin-gonic/gin"
)

func getAndValidImageRequest(c *gin.Context, info *relaycommon.RelayInfo) (*dto.ImageRequest, error) {
	imageRequest := &dto.ImageRequest{}
	err := common.UnmarshalBodyReusable(c, imageRequest)
	if err != nil {
		return nil, err
	}
	if imageRequest.Prompt == "" {
		return nil, errors.New("prompt is required")
	}
	if strings.Contains(imageRequest.Size, "×") {
		return nil, errors.New("size an unexpected error occurred in the parameter, please use 'x' instead of the multiplication sign '×'")
	}
	if imageRequest.N == 0 {
		imageRequest.N = 1
	}
	if imageRequest.Size == "" {
		imageRequest.Size = "1024x1024"
	}
	if imageRequest.Model == "" {
		imageRequest.Model = "dall-e-2"
	}

	// Not "256x256", "512x512", or "1024x1024"
	if imageRequest.Model == "dall-e-2" || imageRequest.Model == "dall-e" {
		if imageRequest.Size != "" && imageRequest.Size != "256x256" && imageRequest.Size != "512x512" && imageRequest.Size != "1024x1024" {
			return nil, errors.New("size must be one of 256x256, 512x512, or 1024x1024, dall-e-3 1024x1792 or 1792x1024")
		}
	} else if imageRequest.Model == "dall-e-3" {
		if imageRequest.Size != "" && imageRequest.Size != "1024x1024" && imageRequest.Size != "1024x1792" && imageRequest.Size != "1792x1024" {
			return nil, errors.New("size must be one of 256x256, 512x512, or 1024x1024, dall-e-3 1024x1792 or 1792x1024")
		}
		if imageRequest.Quality == "" {
			imageRequest.Quality = "standard"
		}
		//if imageRequest.N != 1 {
		//	return nil, errors.New("n must be 1")
		//}
	}
	// N should between 1 and 10
	//if imageRequest.N != 0 && (imageRequest.N < 1 || imageRequest.N > 10) {
	//	return service.OpenAIErrorWrapper(errors.New("n must be between 1 and 10"), "invalid_field_value", http.StatusBadRequest)
	//}
	tokenGroup := c.GetString("token_group")
	if setting.ShouldCheckPromptSensitiveWithGroup(tokenGroup) {
		words, err := service.CheckSensitiveInput(imageRequest.Prompt)
		if err != nil {
			common.LogWarn(c, fmt.Sprintf("user sensitive words detected: %s", strings.Join(words, ",")))
			return nil, err
		}
	}
	return imageRequest, nil
}

func ImageHelper(c *gin.Context) *dto.OpenAIErrorWithStatusCode {
	relayInfo := relaycommon.GenRelayInfo(c)

	imageRequest, err := getAndValidImageRequest(c, relayInfo)
	if err != nil {
		common.LogError(c, fmt.Sprintf("getAndValidImageRequest failed: %s", err.Error()))
		return service.OpenAIErrorWrapper(err, "invalid_image_request", http.StatusBadRequest)
	}

	err = helper.ModelMappedHelper(c, relayInfo)
	if err != nil {
		return service.OpenAIErrorWrapperLocal(err, "model_mapped_error", http.StatusInternalServerError)
	}

	imageRequest.Model = relayInfo.UpstreamModelName

	priceData, err := helper.ModelPriceHelper(c, relayInfo, 0, 0)
	if err != nil {
		return service.OpenAIErrorWrapperLocal(err, "model_price_error", http.StatusInternalServerError)
	}
	if !priceData.UsePrice {
		// modelRatio 16 = modelPrice $0.04
		// per 1 modelRatio = $0.04 / 16
		priceData.ModelPrice = 0.0025 * priceData.ModelRatio
	}

	userBalance, err := model.GetUserQuotaBalance(relayInfo.UserId, false)
	if err != nil {
		return service.OpenAIErrorWrapperLocal(err, "get_user_quota_failed", http.StatusInternalServerError)
	}
	totalQuota := userBalance.Total()

	sizeRatio := 1.0
	// Size
	if imageRequest.Size == "256x256" {
		sizeRatio = 0.4
	} else if imageRequest.Size == "512x512" {
		sizeRatio = 0.45
	} else if imageRequest.Size == "1024x1024" {
		sizeRatio = 1
	} else if imageRequest.Size == "1024x1792" || imageRequest.Size == "1792x1024" {
		sizeRatio = 2
	}

	qualityRatio := 1.0
	if imageRequest.Model == "dall-e-3" && imageRequest.Quality == "hd" {
		qualityRatio = 2.0
		if imageRequest.Size == "1024x1792" || imageRequest.Size == "1792x1024" {
			qualityRatio = 1.5
		}
	}

	priceData.ModelPrice *= sizeRatio * qualityRatio * float64(imageRequest.N)
	quota := int(priceData.ModelPrice * priceData.GroupRatio * common.QuotaPerUnit)

	if totalQuota-quota < 0 {
		return service.OpenAIErrorWrapperLocal(fmt.Errorf("image pre-consumed quota failed, user quota: %s, need quota: %s", common.FormatQuota(totalQuota), common.FormatQuota(quota)), "insufficient_user_quota", http.StatusForbidden)
	}

	adaptor := GetAdaptor(relayInfo.ApiType)
	if adaptor == nil {
		return service.OpenAIErrorWrapperLocal(fmt.Errorf("invalid api type: %d", relayInfo.ApiType), "invalid_api_type", http.StatusBadRequest)
	}
	adaptor.Init(relayInfo)

	var requestBody io.Reader

	convertedRequest, err := adaptor.ConvertImageRequest(c, relayInfo, *imageRequest)
	if err != nil {
		return service.OpenAIErrorWrapperLocal(err, "convert_request_failed", http.StatusInternalServerError)
	}

	jsonData, err := json.Marshal(convertedRequest)
	if err != nil {
		return service.OpenAIErrorWrapperLocal(err, "json_marshal_failed", http.StatusInternalServerError)
	}
	requestBody = bytes.NewBuffer(jsonData)

	statusCodeMappingStr := c.GetString("status_code_mapping")

	resp, err := adaptor.DoRequest(c, relayInfo, requestBody)
	if err != nil {
		return service.OpenAIErrorWrapper(err, "do_request_failed", http.StatusInternalServerError)
	}
	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		relayInfo.IsStream = relayInfo.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
		if httpResp.StatusCode != http.StatusOK {
			openaiErr := service.RelayErrorHandler(httpResp, false)
			// reset status code 重置状态码
			service.ResetStatusCode(openaiErr, statusCodeMappingStr)
			return openaiErr
		}
	}

	_, openaiErr := adaptor.DoResponse(c, httpResp, relayInfo)
	if openaiErr != nil {
		// reset status code 重置状态码
		service.ResetStatusCode(openaiErr, statusCodeMappingStr)
		return openaiErr
	}

	usage := &dto.Usage{
		PromptTokens: imageRequest.N,
		TotalTokens:  imageRequest.N,
	}

	quality := "standard"
	if imageRequest.Quality == "hd" {
		quality = "hd"
	}

	logContent := fmt.Sprintf("大小 %s, 品质 %s", imageRequest.Size, quality)
	// 标记响应已写入，用于空回复检测
	c.Set("response_written", true)
	postConsumeQuota(c, relayInfo, usage, 0, totalQuota, priceData, logContent)
	return nil
}
