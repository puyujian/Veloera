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

package dto

import "time"

// GenerateRenameRequest 生成重命名映射请求
type GenerateRenameRequest struct {
	Mode          string `json:"mode" binding:"required,oneof=system ai"`
	AIModel       string `json:"ai_model"`
	Prompt        string `json:"prompt"`
	ChannelIDs    []int  `json:"channel_ids"`
	EnabledOnly   bool   `json:"enabled_only"`
	IncludeVendor bool   `json:"include_vendor"` // 系统模式下是否包含厂商前缀
}

// ChannelRenamePreview 单个渠道的重命名预览
type ChannelRenamePreview struct {
	ChannelID       int               `json:"channel_id"`
	ChannelName     string            `json:"channel_name"`
	Group           string            `json:"group"`
	OriginalModels  []string          `json:"original_models"`
	OldMapping      map[string]string `json:"old_mapping"`
	NewMapping      map[string]string `json:"new_mapping"`
	AffectedModels  []string          `json:"affected_models"`
	UnchangedModels []string          `json:"unchanged_models"`
}

// RenameStatistics 重命名统计信息
type RenameStatistics struct {
	TotalChannels   int `json:"total_channels"`
	TotalModels     int `json:"total_models"`
	UniqueModels    int `json:"unique_models"`
	RenamedModels   int `json:"renamed_models"`
	UnchangedModels int `json:"unchanged_models"`
}

// GenerateRenameResponse 生成重命名响应
type GenerateRenameResponse struct {
	SessionID     string                 `json:"session_id"`
	GlobalMapping map[string]string      `json:"global_mapping"`
	Channels      []ChannelRenamePreview `json:"channels"`
	Statistics    RenameStatistics       `json:"statistics"`
}

// ApplyRenameRequest 应用重命名请求
type ApplyRenameRequest struct {
	SessionID  string            `json:"session_id" binding:"required"`
	Mode       string            `json:"mode" binding:"required,oneof=append replace"`
	ChannelIDs []int             `json:"channel_ids" binding:"required"`
	Mapping    map[string]string `json:"mapping" binding:"required"`
}

// ApplyRenameResult 单个渠道的应用结果
type ApplyRenameResult struct {
	ChannelID    int    `json:"channel_id"`
	ChannelName  string `json:"channel_name"`
	Success      bool   `json:"success"`
	Error        string `json:"error,omitempty"`
	UpdatedCount int    `json:"updated_count"`
}

// ApplyRenameResponse 应用重命名响应
type ApplyRenameResponse struct {
	SessionID string              `json:"session_id"`
	Total     int                 `json:"total"`
	Success   int                 `json:"success"`
	Failed    int                 `json:"failed"`
	Results   []ApplyRenameResult `json:"results"`
}

// RenameSnapshot 重命名快照（用于撤销）
type RenameSnapshot struct {
	SessionID   string         `json:"session_id"`
	CreatedAt   time.Time      `json:"created_at"`
	Channels    map[int]string `json:"channels"` // channelID → old ModelMapping JSON
	Description string         `json:"description"`
}

// UndoRenameRequest 撤销重命名请求
type UndoRenameRequest struct {
	SessionID string `json:"session_id" binding:"required"`
}

// UndoRenameResponse 撤销重命名响应（复用ApplyRenameResponse）
type UndoRenameResponse = ApplyRenameResponse
