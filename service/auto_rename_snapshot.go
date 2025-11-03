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
	"strings"
	"time"
	"veloera/common"
	"veloera/dto"
	"veloera/model"
)

const snapshotPrefix = "auto_rename_snapshot_"

// SaveRenameSnapshot 保存重命名快照
func SaveRenameSnapshot(sessionID string, channels map[int]string, description string) error {
	snapshot := dto.RenameSnapshot{
		SessionID:   sessionID,
		CreatedAt:   time.Now(),
		Channels:    channels,
		Description: description,
	}

	jsonData, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("序列化快照失败: %w", err)
	}

	key := snapshotPrefix + sessionID
	if err := model.UpdateOption(key, string(jsonData)); err != nil {
		return fmt.Errorf("保存快照到数据库失败: %w", err)
	}

	common.SysLog(fmt.Sprintf("保存自动重命名快照: %s, 影响 %d 个渠道", sessionID, len(channels)))
	return nil
}

// LoadRenameSnapshot 加载重命名快照
func LoadRenameSnapshot(sessionID string) (*dto.RenameSnapshot, error) {
	key := snapshotPrefix + sessionID

	common.OptionMapRWMutex.RLock()
	jsonStr, exists := common.OptionMap[key]
	common.OptionMapRWMutex.RUnlock()

	if !exists || jsonStr == "" {
		return nil, fmt.Errorf("快照不存在: %s", sessionID)
	}

	var snapshot dto.RenameSnapshot
	if err := json.Unmarshal([]byte(jsonStr), &snapshot); err != nil {
		return nil, fmt.Errorf("解析快照失败: %w", err)
	}

	return &snapshot, nil
}

// ListRenameSnapshots 列出所有重命名快照
func ListRenameSnapshots() ([]*dto.RenameSnapshot, error) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	snapshots := make([]*dto.RenameSnapshot, 0)

	for key, jsonStr := range common.OptionMap {
		if !strings.HasPrefix(key, snapshotPrefix) {
			continue
		}

		var snapshot dto.RenameSnapshot
		if err := json.Unmarshal([]byte(jsonStr), &snapshot); err != nil {
			common.SysError(fmt.Sprintf("解析快照失败 (%s): %v", key, err))
			continue
		}

		snapshots = append(snapshots, &snapshot)
	}

	// 按创建时间倒序排序
	for i := 0; i < len(snapshots)-1; i++ {
		for j := i + 1; j < len(snapshots); j++ {
			if snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt) {
				snapshots[i], snapshots[j] = snapshots[j], snapshots[i]
			}
		}
	}

	return snapshots, nil
}

// DeleteRenameSnapshot 删除重命名快照
func DeleteRenameSnapshot(sessionID string) error {
	key := snapshotPrefix + sessionID

	if err := model.DeleteOption(key); err != nil {
		return fmt.Errorf("删除快照失败: %w", err)
	}

	common.SysLog(fmt.Sprintf("删除自动重命名快照: %s", sessionID))
	return nil
}

// CleanOldSnapshots 清理旧快照（保留最近30天）
func CleanOldSnapshots(daysToKeep int) error {
	if daysToKeep <= 0 {
		daysToKeep = 30
	}

	cutoffTime := time.Now().AddDate(0, 0, -daysToKeep)
	snapshots, err := ListRenameSnapshots()
	if err != nil {
		return err
	}

	deletedCount := 0
	for _, snapshot := range snapshots {
		if snapshot.CreatedAt.Before(cutoffTime) {
			if err := DeleteRenameSnapshot(snapshot.SessionID); err != nil {
				common.SysError(fmt.Sprintf("清理快照失败 (%s): %v", snapshot.SessionID, err))
			} else {
				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		common.SysLog(fmt.Sprintf("清理了 %d 个过期快照", deletedCount))
	}

	return nil
}
