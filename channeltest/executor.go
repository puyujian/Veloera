package channeltest

import (
    "bytes"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "math"
    "net/http"
    "net/http/httptest"
    "net/url"
    "strings"
    "time"
    "veloera/common"
    "veloera/dto"
    "veloera/middleware"
    "veloera/model"
    "veloera/relay"
    relaycommon "veloera/relay/common"
    "veloera/relay/constant"
    "veloera/relay/helper"
    "veloera/service"

    "github.com/gin-gonic/gin"
)

// ExecuteChannelTest 复用现有单通道测试流程，返回耗时（秒）与错误信息
func ExecuteChannelTest(channel *model.Channel, testModel string) (consumed float64, err error, openAIError *dto.OpenAIErrorWithStatusCode) {
    if channel == nil {
        return 0, errors.New("channel is nil"), nil
    }

    start := time.Now()

    if channel.Type == common.ChannelTypeMidjourney {
        return elapsedSeconds(start), errors.New("midjourney channel test is not supported"), nil
    }
    if channel.Type == common.ChannelTypeMidjourneyPlus {
        return elapsedSeconds(start), errors.New("midjourney plus channel test is not supported!!!"), nil
    }
    if channel.Type == common.ChannelTypeSunoAPI {
        return elapsedSeconds(start), errors.New("suno channel test is not supported"), nil
    }

    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)

    requestPath := "/v1/chat/completions"
    loweredModel := strings.ToLower(testModel)
    if strings.Contains(loweredModel, "embedding") ||
        strings.HasPrefix(testModel, "m3e") ||
        strings.Contains(testModel, "bge-") ||
        strings.Contains(testModel, "embed") ||
        channel.Type == common.ChannelTypeMokaAI {
        requestPath = "/v1/embeddings"
    }

    c.Request = &http.Request{
        Method: "POST",
        URL:    &url.URL{Path: requestPath},
        Body:   nil,
        Header: make(http.Header),
    }

    if testModel == "" {
        if channel.TestModel != nil && *channel.TestModel != "" {
            testModel = *channel.TestModel
        } else {
            models := channel.GetModels()
            if len(models) > 0 {
                testModel = models[0]
            } else {
                testModel = "gpt-4o-mini"
            }
        }
    }

    cache, cacheErr := model.GetUserCache(1)
    if cacheErr != nil {
        return elapsedSeconds(start), cacheErr, nil
    }
    cache.WriteContext(c)

    c.Request.Header.Set("Authorization", "Bearer "+channel.Key)
    c.Request.Header.Set("Content-Type", "application/json")
    c.Set("channel", channel.Type)
    c.Set("base_url", channel.GetBaseURL())
    group, _ := model.GetUserGroup(1, false)
    c.Set("group", group)

    middleware.SetupContextForSelectedChannel(c, channel, testModel)

    info := relaycommon.GenRelayInfo(c)

    if err = helper.ModelMappedHelper(c, info); err != nil {
        return elapsedSeconds(start), err, nil
    }
    testModel = info.UpstreamModelName

    apiType, _ := constant.ChannelType2APIType(channel.Type)
    adaptor := relay.GetAdaptor(apiType)
    if adaptor == nil {
        return elapsedSeconds(start), fmt.Errorf("invalid api type: %d, adaptor is nil", apiType), nil
    }

    request := buildTestRequest(testModel)
    common.SysLog(fmt.Sprintf("testing channel %d with model %s , info %v ", channel.Id, testModel, info))

    priceData, priceErr := helper.ModelPriceHelper(c, info, 0, int(request.MaxTokens))
    if priceErr != nil {
        return elapsedSeconds(start), priceErr, nil
    }

    adaptor.Init(info)

    convertedRequest, convertErr := adaptor.ConvertOpenAIRequest(c, info, request)
    if convertErr != nil {
        return elapsedSeconds(start), convertErr, nil
    }

    jsonData, marshalErr := json.Marshal(convertedRequest)
    if marshalErr != nil {
        return elapsedSeconds(start), marshalErr, nil
    }
    requestBody := bytes.NewBuffer(jsonData)
    c.Request.Body = io.NopCloser(requestBody)

    resp, reqErr := adaptor.DoRequest(c, info, requestBody)
    if reqErr != nil {
        return elapsedSeconds(start), reqErr, nil
    }

    var httpResp *http.Response
    if resp != nil {
        httpResp = resp.(*http.Response)
        if httpResp.StatusCode != http.StatusOK {
            openAIError = serviceRelayError(httpResp)
            if openAIError != nil {
                return elapsedSeconds(start), fmt.Errorf("status code %d: %s", httpResp.StatusCode, openAIError.Error.Message), openAIError
            }
            return elapsedSeconds(start), fmt.Errorf("status code %d", httpResp.StatusCode), nil
        }
    }

    usageA, respErr := adaptor.DoResponse(c, httpResp, info)
    if respErr != nil {
        return elapsedSeconds(start), fmt.Errorf("%s", respErr.Error.Message), respErr
    }
    if usageA == nil {
        return elapsedSeconds(start), errors.New("usage is nil"), nil
    }
    usage := usageA.(*dto.Usage)

    result := w.Result()
    respBody, readErr := io.ReadAll(result.Body)
    if readErr != nil {
        return elapsedSeconds(start), readErr, nil
    }
    info.PromptTokens = usage.PromptTokens

    quota := calculateQuota(usage, &priceData)

    elapsed := time.Since(start)
    milliseconds := elapsed.Milliseconds()
    consumedTime := float64(milliseconds) / 1000.0

    other := service.GenerateTextOtherInfo(c, info, priceData.ModelRatio, priceData.GroupRatio, priceData.CompletionRatio,
        usage.PromptTokensDetails.CachedTokens, priceData.CacheRatio, priceData.ModelPrice)

    model.RecordConsumeLog(c, 1, channel.Id, usage.PromptTokens, usage.CompletionTokens, info.OriginModelName, "模型测试",
        quota, "模型测试", 0, quota, int(consumedTime), false, info.Group, other)

    common.SysLog(fmt.Sprintf("testing channel #%d, response: \n%s", channel.Id, string(respBody)))

    channel.UpdateResponseTime(milliseconds)

    return consumedTime, nil, nil
}

func buildTestRequest(modelName string) *dto.GeneralOpenAIRequest {
    request := &dto.GeneralOpenAIRequest{
        Model:  "",
        Stream: false,
    }

    lowered := strings.ToLower(modelName)
    if strings.Contains(lowered, "embedding") ||
        strings.HasPrefix(modelName, "m3e") ||
        strings.Contains(modelName, "bge-") {
        request.Model = modelName
        request.Input = []string{"hello world"}
        return request
    }

    if strings.HasPrefix(modelName, "o1") || strings.HasPrefix(modelName, "o3") {
        request.MaxCompletionTokens = 10
    } else if strings.Contains(modelName, "thinking") {
        if !strings.Contains(modelName, "claude") {
            request.MaxTokens = 50
        }
    } else if strings.Contains(modelName, "gemini") {
        request.MaxTokens = 300
    } else {
        request.MaxTokens = 10
    }

    content, _ := json.Marshal("hi")
    request.Model = modelName
    request.Messages = append(request.Messages, dto.Message{
        Role:    "user",
        Content: content,
    })
    return request
}

func calculateQuota(usage *dto.Usage, priceData *helper.PriceData) int {
    if priceData == nil || usage == nil {
        return 0
    }
    if !priceData.UsePrice {
        quota := usage.PromptTokens + int(math.Round(float64(usage.CompletionTokens)*priceData.CompletionRatio))
        quota = int(math.Round(float64(quota) * priceData.ModelRatio))
        if priceData.ModelRatio != 0 && quota <= 0 {
            quota = 1
        }
        return quota
    }
    return int(priceData.ModelPrice * common.QuotaPerUnit)
}

func elapsedSeconds(start time.Time) float64 {
    return float64(time.Since(start).Milliseconds()) / 1000.0
}

func serviceRelayError(httpResp *http.Response) *dto.OpenAIErrorWithStatusCode {
    if httpResp == nil {
        return nil
    }
    err := service.RelayErrorHandler(httpResp, true)
    return err
}
