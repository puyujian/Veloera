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

package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"veloera/common"
	"veloera/middleware"
	"veloera/model"
	"veloera/service"

	"github.com/gin-gonic/gin"
)

type autoUpdateChannelResult struct {
	ChannelID         int      `json:"channel_id"`
	ChannelName       string   `json:"channel_name"`
	Success           bool     `json:"success"`
	Error             string   `json:"error,omitempty"`
	BeforeCount       int      `json:"before_count"`
	UpstreamCount     int      `json:"upstream_count"`
	AfterCount        int      `json:"after_count"`
	AddedModels       []string `json:"added_models"`
	RenamedModels     int      `json:"renamed_models,omitempty"`
	AutoRenameApplied bool     `json:"auto_rename_applied"`
}

func parseScheduledAutoUpdateChannelIDs(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []int{}
	}

	parts := strings.Split(raw, ",")
	ids := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err != nil || id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func formatScheduledAutoUpdateChannelIDs(ids []int) string {
	if len(ids) == 0 {
		return ""
	}
	seen := make(map[int]struct{}, len(ids))
	valid := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		valid = append(valid, id)
	}
	if len(valid) == 0 {
		return ""
	}
	sort.Ints(valid)
	parts := make([]string, len(valid))
	for i, id := range valid {
		parts[i] = strconv.Itoa(id)
	}
	return strings.Join(parts, ",")
}

// AutoUpdateChannelModels 自动更新渠道模型（批量）
func AutoUpdateChannelModels(c *gin.Context) {
	type AutoUpdateRequest struct {
		ChannelIDs       []int  `json:"channel_ids"`        // 选中的渠道ID列表
		UpdateMode       string `json:"update_mode"`        // incremental 或 full
		EnableAutoRename bool   `json:"enable_auto_rename"` // 是否启用自动重命名
		IncludeVendor    bool   `json:"include_vendor"`     // 自动重命名时是否包含厂商前缀
	}

	type AutoUpdateResponse struct {
		Total   int                       `json:"total"`
		Success int                       `json:"success"`
		Failed  int                       `json:"failed"`
		Results []autoUpdateChannelResult `json:"results"`
	}

	var req AutoUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}

	// 验证参数
	if len(req.ChannelIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请至少选择一个渠道"})
		return
	}

	updateMode := strings.ToLower(strings.TrimSpace(req.UpdateMode))
	if updateMode != "incremental" && updateMode != "full" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "update_mode 仅支持 incremental 或 full"})
		return
	}

	// 获取选中的渠道
	channels := make([]*model.Channel, 0, len(req.ChannelIDs))
	for _, id := range req.ChannelIDs {
		ch, err := model.GetChannelById(id, true)
		if err != nil {
			common.SysError(fmt.Sprintf("[自动更新模型] 获取渠道 %d 失败: %v", id, err))
			continue
		}
		if ch == nil {
			common.SysError(fmt.Sprintf("[自动更新模型] 渠道 %d 不存在", id))
			continue
		}
		channels = append(channels, ch)
	}

	if len(channels) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "没有可处理的渠道"})
		return
	}

	// 并发处理每个渠道
	sem := make(chan struct{}, 8) // 并发度为8
	results := make([]autoUpdateChannelResult, 0, len(channels))
	done := make(chan autoUpdateChannelResult, len(channels))

	for _, ch := range channels {
		ch := ch
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
			result := processChannelAutoUpdate(ch, updateMode, req.EnableAutoRename, req.IncludeVendor)
			done <- result
		}()
	}

	// 收集结果
	successCount := 0
	failedCount := 0
	for i := 0; i < len(channels); i++ {
		result := <-done
		results = append(results, result)
		if result.Success {
			successCount++
		} else {
			failedCount++
		}
	}

	response := AutoUpdateResponse{
		Total:   len(channels),
		Success: successCount,
		Failed:  failedCount,
		Results: results,
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": response})
}

// processChannelAutoUpdate 处理单个渠道的自动更新
func processChannelAutoUpdate(ch *model.Channel, updateMode string, enableAutoRename bool, includeVendor bool) autoUpdateChannelResult {
	result := autoUpdateChannelResult{
		ChannelID:   ch.Id,
		ChannelName: ch.Name,
		Success:     false,
	}

	// 1. 获取当前模型列表
	currentModels := ch.GetModels()
	result.BeforeCount = len(currentModels)

	// 2. 从上游拉取模型列表
	upstreamModels, err := fetchUpstreamModels(ch)
	if err != nil {
		result.Error = fmt.Sprintf("获取上游模型失败: %v", err)
		common.SysError(fmt.Sprintf("[自动更新模型] 渠道 %d(%s) %s", ch.Id, ch.Name, result.Error))
		return result
	}
	result.UpstreamCount = len(upstreamModels)

	// 3. 构建上游模型集合
	upstreamSet := make(map[string]struct{}, len(upstreamModels))
	for _, m := range upstreamModels {
		if mm := strings.TrimSpace(m); mm != "" {
			upstreamSet[mm] = struct{}{}
		}
	}

	// 4. 根据更新模式计算新增模型
	var addedModels []string
	var nextModels []string
	redirectMapping := buildChannelRedirectMapping(ch)

	normalizeName := func(name string) string {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return ""
		}
		if redirectMapping != nil {
			if actual := redirectMapping.resolveActual(trimmed); actual != "" {
				return actual
			}
		}
		return trimmed
	}

	if updateMode == "incremental" {
		// 增量模式：保留现有模型，只添加新增的
		currentSet := make(map[string]struct{}, len(currentModels))
		for _, m := range currentModels {
			trimmed := strings.TrimSpace(m)
			if trimmed != "" {
				actualName := normalizeName(trimmed)
				currentSet[actualName] = struct{}{}
			}
		}

		// 保留现有模型
		nextModels = append(nextModels, currentModels...)

		// 添加新增模型
		for _, upstreamModel := range upstreamModels {
			upstreamModel = strings.TrimSpace(upstreamModel)
			if upstreamModel == "" {
				continue
			}
			if _, exists := currentSet[upstreamModel]; !exists {
				addedModels = append(addedModels, upstreamModel)
				nextModels = append(nextModels, upstreamModel)
			}
		}
	} else {
		// 完全同步模式：以上游为准
		currentActualSet := make(map[string]struct{}, len(currentModels))
		for _, m := range currentModels {
			trimmed := strings.TrimSpace(m)
			if trimmed != "" {
				actualName := normalizeName(trimmed)
				currentActualSet[actualName] = struct{}{}
			}
		}

		for _, upstreamModel := range upstreamModels {
			upstreamModel = strings.TrimSpace(upstreamModel)
			if upstreamModel == "" {
				continue
			}
			nextModels = append(nextModels, upstreamModel)
			if _, exists := currentActualSet[upstreamModel]; !exists {
				addedModels = append(addedModels, upstreamModel)
			}
		}
	}

	result.AddedModels = addedModels

	// 5. 如果启用自动重命名且有新增模型，执行重命名
	if enableAutoRename && len(addedModels) > 0 {
		// 对新增模型进行重命名处理
		globalMapping := service.MultiPassSystemRenameProcessor(addedModels, includeVendor)
		if len(globalMapping) > 0 {
			// 应用重命名到新增模型
			renamedNextModels := make([]string, 0, len(nextModels))
			reverseMapping := make(map[string]string)
			for newName, origName := range globalMapping {
				reverseMapping[strings.TrimSpace(origName)] = strings.TrimSpace(newName)
			}

			for _, model := range nextModels {
				model = strings.TrimSpace(model)
				if model == "" {
					continue
				}
				if renamedModel, exists := reverseMapping[model]; exists {
					renamedNextModels = append(renamedNextModels, renamedModel)
				} else {
					renamedNextModels = append(renamedNextModels, model)
				}
			}
			nextModels = renamedNextModels

			// 更新模型映射
			currentMapping := make(map[string]string)
			if ch.ModelMapping != nil && *ch.ModelMapping != "" && *ch.ModelMapping != "{}" {
				json.Unmarshal([]byte(*ch.ModelMapping), &currentMapping)
			}

			// 合并新的映射
			for newName, origName := range globalMapping {
				currentMapping[newName] = origName
			}

			mappingJSON, err := json.Marshal(currentMapping)
			if err == nil {
				mappingStr := string(mappingJSON)
				ch.ModelMapping = &mappingStr
				result.RenamedModels = len(globalMapping)
				result.AutoRenameApplied = true
			}
		}
	}

	// 6. 更新渠道模型列表
	ch.Models = strings.Join(nextModels, ",")
	result.AfterCount = len(nextModels)

	// 7. 保存到数据库
	if err := ch.Update(); err != nil {
		result.Error = fmt.Sprintf("更新渠道失败: %v", err)
		common.SysError(fmt.Sprintf("[自动更新模型] 渠道 %d(%s) %s", ch.Id, ch.Name, result.Error))
		return result
	}

	// 8. 刷新缓存
	middleware.RefreshPrefixChannelsCache(ch.Group)

	result.Success = true
	return result
}

// fetchUpstreamModels 从上游获取模型列表
func fetchUpstreamModels(ch *model.Channel) ([]string, error) {
	// 构建上游 URL
	baseURL := strings.TrimSpace(ch.GetBaseURL())
	if baseURL == "" {
		if ch.Type < len(common.ChannelBaseURLs) && common.ChannelBaseURLs[ch.Type] != "" {
			baseURL = strings.TrimSpace(common.ChannelBaseURLs[ch.Type])
		}
	}
	if baseURL == "" {
		return nil, fmt.Errorf("无法确定上游地址")
	}

	baseURL = strings.TrimRight(baseURL, "/")

	// 根据渠道类型构建模型列表 URL
	var modelsURL string
	switch ch.Type {
	case common.ChannelTypeGemini:
		modelsURL = fmt.Sprintf("%s/v1beta/openai/models", baseURL)
	case common.ChannelTypeGitHub:
		// GitHub 特殊处理：移除 /inference 后缀
		cleanedURL := strings.TrimSuffix(baseURL, "/inference")
		cleanedURL = strings.TrimRight(cleanedURL, "/")
		modelsURL = fmt.Sprintf("%s/catalog/models", cleanedURL)
	default:
		modelsURL = fmt.Sprintf("%s/v1/models", baseURL)
	}

	// 获取 API Key
	key := strings.Split(ch.Key, ",")[0]

	// 请求上游
	body, err := GetResponseBody("GET", modelsURL, ch, GetAuthHeader(key))
	if err != nil {
		return nil, fmt.Errorf("请求上游失败: %w", err)
	}

	// 解析响应
	if ch.Type == common.ChannelTypeGitHub {
		var arr []struct {
			ID string `json:"id"`
		}
		if e := json.Unmarshal(body, &arr); e != nil {
			return nil, fmt.Errorf("解析 GitHub 响应失败: %w", e)
		}
		ids := make([]string, 0, len(arr))
		for _, m := range arr {
			ids = append(ids, m.ID)
		}
		return ids, nil
	}

	var result OpenAIModelsResponse
	if e := json.Unmarshal(body, &result); e != nil {
		return nil, fmt.Errorf("解析 OpenAI 响应失败: %w", e)
	}

	ids := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		id := m.ID
		if ch.Type == common.ChannelTypeGemini {
			id = strings.TrimPrefix(id, "models/")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// ScheduledAutoUpdateChannelModels 定时自动更新渠道模型
func ScheduledAutoUpdateChannelModels(defaultFrequency int) {
	if defaultFrequency <= 0 {
		defaultFrequency = 60
	}

	for {
		common.OptionMapRWMutex.RLock()
		enabled := common.OptionMap["ScheduledAutoUpdateEnabled"] == "true"
		mode := strings.ToLower(strings.TrimSpace(common.OptionMap["ScheduledAutoUpdateMode"]))
		if mode != "full" {
			mode = "incremental"
		}
		enableRename := common.OptionMap["ScheduledAutoUpdateEnableRename"] == "true"
		includeVendor := common.OptionMap["ScheduledAutoUpdateIncludeVendor"] == "true"
		channelIdsStr := common.OptionMap["ScheduledAutoUpdateChannelIds"]
		frequencyStr := common.OptionMap["ScheduledAutoUpdateFrequency"]
		common.OptionMapRWMutex.RUnlock()

		frequency := defaultFrequency
		if frequencyStr != "" {
			if freq, err := strconv.Atoi(strings.TrimSpace(frequencyStr)); err == nil && freq > 0 {
				frequency = freq
			}
		}
		if frequency < 5 {
			frequency = 5
		}

		if !enabled {
			time.Sleep(time.Duration(frequency) * time.Minute)
			continue
		}

		common.SysLog("[定时更新模型] 开始执行")

		channelIDs := parseScheduledAutoUpdateChannelIDs(channelIdsStr)

		var channels []*model.Channel
		if len(channelIDs) > 0 {
			for _, id := range channelIDs {
				ch, err := model.GetChannelById(id, true)
				if err != nil {
					common.SysError(fmt.Sprintf("[定时更新模型] 获取渠道 %d 失败: %v", id, err))
					continue
				}
				if ch != nil && ch.Status == common.ChannelStatusEnabled {
					channels = append(channels, ch)
				}
			}
		} else {
			allChannels, err := model.GetAllChannels(0, 0, true, false)
			if err != nil {
				common.SysError(fmt.Sprintf("[定时更新模型] 获取渠道列表失败: %v", err))
				time.Sleep(time.Duration(frequency) * time.Minute)
				continue
			}
			for _, ch := range allChannels {
				if ch.Status == common.ChannelStatusEnabled {
					channels = append(channels, ch)
				}
			}
		}

		if len(channels) == 0 {
			common.SysLog("[定时更新模型] 没有需要更新的渠道")
			time.Sleep(time.Duration(frequency) * time.Minute)
			continue
		}

		sem := make(chan struct{}, 5)
		done := make(chan autoUpdateChannelResult, len(channels))

		for _, ch := range channels {
			ch := ch
			sem <- struct{}{}
			go func() {
				defer func() { <-sem }()
				result := processChannelAutoUpdate(ch, mode, enableRename, includeVendor)
				done <- result
			}()
		}

		successCount := 0
		failedCount := 0
		for i := 0; i < len(channels); i++ {
			result := <-done
			if result.Success {
				successCount++
				common.SysLog(fmt.Sprintf("[定时更新模型] 渠道 %d(%s) 成功，新增 %d 个模型", result.ChannelID, result.ChannelName, len(result.AddedModels)))
			} else {
				failedCount++
				common.SysError(fmt.Sprintf("[定时更新模型] 渠道 %d(%s) 失败: %s", result.ChannelID, result.ChannelName, result.Error))
			}
		}

		common.SysLog(fmt.Sprintf("[定时更新模型] 完成，成功: %d, 失败: %d", successCount, failedCount))

		time.Sleep(time.Duration(frequency) * time.Minute)
	}
}

// GetScheduledAutoUpdateSettings 获取定时自动更新配置
func GetScheduledAutoUpdateSettings(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	enabled := common.OptionMap["ScheduledAutoUpdateEnabled"] == "true"
	mode := strings.ToLower(strings.TrimSpace(common.OptionMap["ScheduledAutoUpdateMode"]))
	if mode != "full" {
		mode = "incremental"
	}
	enableRename := common.OptionMap["ScheduledAutoUpdateEnableRename"] == "true"
	includeVendor := common.OptionMap["ScheduledAutoUpdateIncludeVendor"] == "true"
	channelIdsStr := common.OptionMap["ScheduledAutoUpdateChannelIds"]
	frequencyStr := common.OptionMap["ScheduledAutoUpdateFrequency"]
	common.OptionMapRWMutex.RUnlock()

	frequency := 60
	if frequencyStr != "" {
		if freq, err := strconv.Atoi(strings.TrimSpace(frequencyStr)); err == nil && freq > 0 {
			frequency = freq
		}
	}
	if frequency < 5 {
		frequency = 5
	}

	response := gin.H{
		"enabled":            enabled,
		"frequency":          frequency,
		"mode":               mode,
		"enable_auto_rename": enableRename,
		"include_vendor":     includeVendor,
		"channel_ids":        parseScheduledAutoUpdateChannelIDs(channelIdsStr),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// UpdateScheduledAutoUpdateSettings 更新定时自动更新配置
func UpdateScheduledAutoUpdateSettings(c *gin.Context) {
	type scheduledSettingsRequest struct {
		Enabled          bool   `json:"enabled"`
		Frequency        int    `json:"frequency"`
		Mode             string `json:"mode"`
		EnableAutoRename bool   `json:"enable_auto_rename"`
		IncludeVendor    bool   `json:"include_vendor"`
		ChannelIDs       []int  `json:"channel_ids"`
	}

	var req scheduledSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}

	frequency := req.Frequency
	if frequency <= 0 {
		frequency = 60
	}
	if frequency < 5 {
		frequency = 5
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode != "full" {
		mode = "incremental"
	}

	channelIDsStr := formatScheduledAutoUpdateChannelIDs(req.ChannelIDs)

	updates := []struct {
		key   string
		value string
	}{
		{"ScheduledAutoUpdateEnabled", strconv.FormatBool(req.Enabled)},
		{"ScheduledAutoUpdateFrequency", strconv.Itoa(frequency)},
		{"ScheduledAutoUpdateMode", mode},
		{"ScheduledAutoUpdateEnableRename", strconv.FormatBool(req.EnableAutoRename)},
		{"ScheduledAutoUpdateIncludeVendor", strconv.FormatBool(req.IncludeVendor)},
		{"ScheduledAutoUpdateChannelIds", channelIDsStr},
	}

	for _, item := range updates {
		if err := model.UpdateOption(item.key, item.value); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
	}

	common.SysLog(fmt.Sprintf("[定时更新模型] 更新配置，enabled=%v, frequency=%d, mode=%s, channel_count=%d", req.Enabled, frequency, mode, len(parseScheduledAutoUpdateChannelIDs(channelIDsStr))))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "更新成功",
		"data": gin.H{
			"enabled":            req.Enabled,
			"frequency":          frequency,
			"mode":               mode,
			"enable_auto_rename": req.EnableAutoRename,
			"include_vendor":     req.IncludeVendor,
			"channel_ids":        parseScheduledAutoUpdateChannelIDs(channelIDsStr),
		},
	})
}
