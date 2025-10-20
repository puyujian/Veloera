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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"veloera/common"
	"veloera/dto"
	"veloera/middleware"
	"veloera/model"
	"veloera/service"

	"github.com/gin-gonic/gin"
)

// GenerateAutoRename 生成重命名映射（预览）
func GenerateAutoRename(c *gin.Context) {
	var req dto.GenerateRenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}

	// 验证AI模式参数
	if req.Mode == "ai" {
		if req.AIModel == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "AI模式下必须指定ai_model参数"})
			return
		}
		if req.Prompt == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "AI模式下必须提供prompt参数"})
			return
		}
	}

	// 获取渠道列表
	var channels []*model.Channel
	var err error

	if len(req.ChannelIDs) > 0 {
		channels = make([]*model.Channel, 0, len(req.ChannelIDs))
		for _, id := range req.ChannelIDs {
			ch, e := model.GetChannelById(id, true)
			if e != nil || ch == nil {
				common.SysError(fmt.Sprintf("[自动重命名] 获取渠道 %d 失败: %v", id, e))
				continue
			}
			channels = append(channels, ch)
		}
	} else {
		channels, err = model.GetAllChannels(0, 0, true, false)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取渠道列表失败: " + err.Error()})
			return
		}
	}

	// 过滤仅启用的渠道
	if req.EnabledOnly {
		filtered := make([]*model.Channel, 0, len(channels))
		for _, ch := range channels {
			if ch.Status == common.ChannelStatusEnabled {
				filtered = append(filtered, ch)
			}
		}
		channels = filtered
	}

	if len(channels) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "没有可处理的渠道"})
		return
	}

	// 收集所有模型名（按渠道）
	channelModelsMap := make(map[int][]string)
	allModels := make([]string, 0)

	for _, ch := range channels {
		models := ch.GetModels()
		channelModelsMap[ch.Id] = models
		allModels = append(allModels, models...)
	}

	// 去重
	uniqueModels := deduplicateStrings(allModels)

	if len(uniqueModels) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "没有可处理的模型"})
		return
	}

	// 调用处理器
	var globalMapping map[string]string
	if req.Mode == "system" {
		globalMapping = service.SystemRenameProcessor(uniqueModels, req.IncludeVendor)
	} else if req.Mode == "ai" {
		globalMapping, err = service.AIRenameProcessor(uniqueModels, req.AIModel, req.Prompt)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "AI处理失败: " + err.Error()})
			return
		}
	}

	// 生成SessionID
	sessionID := generateSessionID()

	// 构造各渠道预览
	channelPreviews := make([]dto.ChannelRenamePreview, 0, len(channels))
	totalModelsCount := 0
	renamedCount := 0

	for _, ch := range channels {
		originalModels := channelModelsMap[ch.Id]
		totalModelsCount += len(originalModels)

		// 解析当前渠道的旧映射
		oldMapping := make(map[string]string)
		if ch.ModelMapping != nil && *ch.ModelMapping != "" && *ch.ModelMapping != "{}" {
			json.Unmarshal([]byte(*ch.ModelMapping), &oldMapping)
		}

		// 提取该渠道相关的新映射
		newMapping := make(map[string]string)
		affectedModels := make([]string, 0)
		unchangedModels := make([]string, 0)

		for _, model := range originalModels {
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}

			// 检查是否有重命名
			found := false
			for newName, origName := range globalMapping {
				if origName == model {
					newMapping[newName] = origName
					affectedModels = append(affectedModels, model)
					found = true
					break
				}
			}

			if !found {
				unchangedModels = append(unchangedModels, model)
			}
		}

		channelPreviews = append(channelPreviews, dto.ChannelRenamePreview{
			ChannelID:       ch.Id,
			ChannelName:     ch.Name,
			Group:           ch.Group,
			OriginalModels:  originalModels,
			OldMapping:      oldMapping,
			NewMapping:      newMapping,
			AffectedModels:  affectedModels,
			UnchangedModels: unchangedModels,
		})

		renamedCount += len(affectedModels)
	}

	// 统计信息
	statistics := dto.RenameStatistics{
		TotalChannels:   len(channels),
		TotalModels:     totalModelsCount,
		UniqueModels:    len(uniqueModels),
		RenamedModels:   len(globalMapping),
		UnchangedModels: len(uniqueModels) - len(globalMapping),
	}

	response := dto.GenerateRenameResponse{
		SessionID:     sessionID,
		GlobalMapping: globalMapping,
		Channels:      channelPreviews,
		Statistics:    statistics,
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": response})
}

// ApplyAutoRename 应用重命名
func ApplyAutoRename(c *gin.Context) {
	var req dto.ApplyRenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}

	if len(req.ChannelIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "channel_ids不能为空"})
		return
	}

	if len(req.Mapping) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "mapping不能为空"})
		return
	}

	// 保存快照
	snapshotData := make(map[int]string)
	for _, channelID := range req.ChannelIDs {
		ch, err := model.GetChannelById(channelID, true)
		if err != nil || ch == nil {
			continue
		}
		if ch.ModelMapping != nil {
			snapshotData[channelID] = *ch.ModelMapping
		} else {
			snapshotData[channelID] = "{}"
		}
	}

	description := fmt.Sprintf("批量重命名 %d 个渠道", len(req.ChannelIDs))
	if err := service.SaveRenameSnapshot(req.SessionID, snapshotData, description); err != nil {
		common.SysError(fmt.Sprintf("[自动重命名] 保存快照失败: %v", err))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "保存快照失败: " + err.Error()})
		return
	}

	// 并发应用
	sem := make(chan struct{}, 8) // 并发度8
	results := make(chan dto.ApplyRenameResult, len(req.ChannelIDs))

	for _, channelID := range req.ChannelIDs {
		channelID := channelID
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()

			result := dto.ApplyRenameResult{
				ChannelID: channelID,
				Success:   false,
			}

			ch, err := model.GetChannelById(channelID, true)
			if err != nil || ch == nil {
				result.Error = "获取渠道失败"
				if err != nil {
					result.Error = err.Error()
				}
				results <- result
				return
			}

			result.ChannelName = ch.Name

			// 解析当前映射
			currentMapping := make(map[string]string)
			if ch.ModelMapping != nil && *ch.ModelMapping != "" && *ch.ModelMapping != "{}" {
				if err := json.Unmarshal([]byte(*ch.ModelMapping), &currentMapping); err != nil {
					result.Error = "解析当前映射失败: " + err.Error()
					results <- result
					return
				}
			}

			// 合并映射
			finalMapping := make(map[string]string)
			if req.Mode == "append" {
				// 追加模式：保留旧映射
				for k, v := range currentMapping {
					finalMapping[k] = v
				}
			}
			// 添加/覆盖新映射
			updatedCount := 0
			for newName, origName := range req.Mapping {
				// 只添加该渠道相关的映射
				channelModels := ch.GetModels()
				isRelevant := false
				for _, m := range channelModels {
					if strings.TrimSpace(m) == origName {
						isRelevant = true
						break
					}
				}
				if isRelevant {
					finalMapping[newName] = origName
					updatedCount++
				}
			}

			// 序列化
			finalMappingJSON, err := json.Marshal(finalMapping)
			if err != nil {
				result.Error = "序列化映射失败: " + err.Error()
				results <- result
				return
			}

			// 更新渠道
			finalMappingStr := string(finalMappingJSON)
			ch.ModelMapping = &finalMappingStr
			if err := ch.Update(); err != nil {
				result.Error = "更新渠道失败: " + err.Error()
				results <- result
				return
			}

			// 刷新缓存
			middleware.RefreshPrefixChannelsCache(ch.Group)

			result.Success = true
			result.UpdatedCount = updatedCount
			results <- result
		}()
	}

	// 等待所有完成
	allResults := make([]dto.ApplyRenameResult, 0, len(req.ChannelIDs))
	successCount := 0
	failedCount := 0

	for i := 0; i < len(req.ChannelIDs); i++ {
		result := <-results
		allResults = append(allResults, result)
		if result.Success {
			successCount++
		} else {
			failedCount++
		}
	}

	response := dto.ApplyRenameResponse{
		SessionID: req.SessionID,
		Total:     len(req.ChannelIDs),
		Success:   successCount,
		Failed:    failedCount,
		Results:   allResults,
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": response})
}

// UndoAutoRename 撤销重命名
func UndoAutoRename(c *gin.Context) {
	var req dto.UndoRenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}

	// 加载快照
	snapshot, err := service.LoadRenameSnapshot(req.SessionID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "加载快照失败: " + err.Error()})
		return
	}

	// 并发恢复
	sem := make(chan struct{}, 8)
	results := make(chan dto.ApplyRenameResult, len(snapshot.Channels))

	for channelID, oldMapping := range snapshot.Channels {
		channelID := channelID
		oldMapping := oldMapping
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()

			result := dto.ApplyRenameResult{
				ChannelID: channelID,
				Success:   false,
			}

			ch, err := model.GetChannelById(channelID, true)
			if err != nil || ch == nil {
				result.Error = "获取渠道失败"
				if err != nil {
					result.Error = err.Error()
				}
				results <- result
				return
			}

			result.ChannelName = ch.Name

			// 恢复旧映射
			ch.ModelMapping = &oldMapping
			if err := ch.Update(); err != nil {
				result.Error = "更新渠道失败: " + err.Error()
				results <- result
				return
			}

			// 刷新缓存
			middleware.RefreshPrefixChannelsCache(ch.Group)

			result.Success = true
			results <- result
		}()
	}

	// 等待所有完成
	allResults := make([]dto.ApplyRenameResult, 0, len(snapshot.Channels))
	successCount := 0
	failedCount := 0

	for i := 0; i < len(snapshot.Channels); i++ {
		result := <-results
		allResults = append(allResults, result)
		if result.Success {
			successCount++
		} else {
			failedCount++
		}
	}

	response := dto.UndoRenameResponse{
		SessionID: req.SessionID,
		Total:     len(snapshot.Channels),
		Success:   successCount,
		Failed:    failedCount,
		Results:   allResults,
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "撤销成功", "data": response})
}

// ListAutoRenameSnapshots 列出所有快照
func ListAutoRenameSnapshots(c *gin.Context) {
	snapshots, err := service.ListRenameSnapshots()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取快照列表失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": snapshots})
}

// deduplicateStrings 字符串数组去重
func deduplicateStrings(arr []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(arr))

	for _, item := range arr {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; !exists {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}

	return result
}

// generateSessionID 生成会话ID
func generateSessionID() string {
	timestamp := time.Now().Format("20060102_150405")
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	randomStr := hex.EncodeToString(randomBytes)
	return fmt.Sprintf("rename_%s_%s", timestamp, randomStr)
}
