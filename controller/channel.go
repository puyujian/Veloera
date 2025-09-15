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
	"strconv"
	"strings"
	"veloera/common"
	"veloera/middleware"
	"veloera/model"

	"github.com/gin-gonic/gin"
)

type OpenAIModel struct {
	ID         string `json:"id"`
	Object     string `json:"object"`
	Created    int64  `json:"created"`
	OwnedBy    string `json:"owned_by"`
	Permission []struct {
		ID                 string `json:"id"`
		Object             string `json:"object"`
		Created            int64  `json:"created"`
		AllowCreateEngine  bool   `json:"allow_create_engine"`
		AllowSampling      bool   `json:"allow_sampling"`
		AllowLogprobs      bool   `json:"allow_logprobs"`
		AllowSearchIndices bool   `json:"allow_search_indices"`
		AllowView          bool   `json:"allow_view"`
		AllowFineTuning    bool   `json:"allow_fine_tuning"`
		Organization       string `json:"organization"`
		Group              string `json:"group"`
		IsBlocking         bool   `json:"is_blocking"`
	} `json:"permission"`
	Root   string `json:"root"`
	Parent string `json:"parent"`
}

type OpenAIModelsResponse struct {
	Data    []OpenAIModel `json:"data"`
	Success bool          `json:"success"`
}
// GetChannelTags 返回去重后的标签列表，可用于前端筛选下拉
func GetChannelTags(c *gin.Context) {
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	tags, err := model.GetPaginatedTags(offset, limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    tags,
	})
}


func GetAllChannels(c *gin.Context) {
	p, _ := strconv.Atoi(c.Query("p"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if p < 0 {
		p = 0
	}
	if pageSize < 0 {
		pageSize = common.ItemsPerPage
	}
	channelData := make([]*model.Channel, 0)
	idSort, _ := strconv.ParseBool(c.Query("id_sort"))
	enableTagMode, _ := strconv.ParseBool(c.Query("tag_mode"))
	if enableTagMode {
		tags, err := model.GetPaginatedTags(p*pageSize, pageSize)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		for _, tag := range tags {
			if tag != nil && *tag != "" {
				tagChannel, err := model.GetChannelsByTag(*tag, idSort)
				if err == nil {
					channelData = append(channelData, tagChannel...)
				}
			}
		}
	} else {
		channels, err := model.GetAllChannels(p*pageSize, pageSize, false, idSort)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		channelData = channels
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    channelData,
	})
	return
}

func FetchUpstreamModels(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	channel, err := model.GetChannelById(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}


	baseURL := common.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() != "" {
		baseURL = channel.GetBaseURL()
	}
	url := fmt.Sprintf("%s/v1/models", baseURL)

	if strings.HasSuffix(baseURL, "/chat/completions") {
		url = strings.TrimSuffix(baseURL, "/chat/completions") + "/models"
	}

	if channel.Type == common.ChannelTypeGemini {
		url = fmt.Sprintf("%s/v1beta/openai/models", baseURL)
	}
	if channel.Type == common.ChannelTypeGitHub {
		url = strings.Replace(baseURL, "/inference", "/catalog/models", 1)
	}
	key := strings.Split(channel.Key, ",")[0]
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(key))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	var ids []string
	// GitHub 返回的是裸数组，先单独处理
	if channel.Type == common.ChannelTypeGitHub {
		var arr []struct { ID string `json:"id"` }
		if err = json.Unmarshal(body, &arr); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("解析 GitHub 响应失败: %s", err.Error()),
			})
			return
		}
		for _, m := range arr {
			ids = append(ids, m.ID)
		}
	} else {
		var result OpenAIModelsResponse
		if err = json.Unmarshal(body, &result); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("解析响应失败: %s", err.Error()),
			})
			return
		}
		for _, model := range result.Data {
			id := model.ID
			if channel.Type == common.ChannelTypeGemini {
				id = strings.TrimPrefix(id, "models/")
			}
			ids = append(ids, id)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    ids,
	})
}

func FixChannelsAbilities(c *gin.Context) {
	count, err := model.FixAbility()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
}

// 批量模型同步
// 支持两种模式：
// - incremental：仅移除本地中已被上游移除/废弃的模型（与上游取交集），不新增
// - replace：完全使用上游返回的模型列表覆盖本地
// 出错的渠道会被跳过，整体继续处理，并返回详细结果
func SyncChannelModels(c *gin.Context) {
    type SyncRequest struct {
        Mode string `json:"mode"` // incremental | replace
        Ids  []int  `json:"ids"`  // 可选：指定需要同步的渠道ID；为空则同步全部
    }
    type ChannelSyncResult struct {
        ChannelID       int      `json:"channel_id"`
        ChannelName     string   `json:"channel_name"`
        Mode            string   `json:"mode"`
        BeforeCount     int      `json:"before_count"`
        UpstreamCount   int      `json:"upstream_count"`
        AfterCount      int      `json:"after_count"`
        RemovedModels   []string `json:"removed_models"`
        AddedModels     []string `json:"added_models"`
        Error           string   `json:"error,omitempty"`
        Group           string   `json:"group"`
        UpstreamPreview []string `json:"upstream_preview,omitempty"`
    }
    type SyncResponse struct {
        Total       int                   `json:"total"`
        Success     int                   `json:"success"`
        Failed      int                   `json:"failed"`
        Mode        string                `json:"mode"`
        Results     []ChannelSyncResult   `json:"results"`
    }

    var req SyncRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "success": false,
            "message": "参数错误",
        })
        return
    }
    mode := strings.ToLower(strings.TrimSpace(req.Mode))
    if mode != "incremental" && mode != "replace" {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": "mode 仅支持 incremental 或 replace",
        })
        return
    }

    // 加载渠道列表（需要包含 key）
    // 若传入 ids 则只处理指定渠道
    var channels []*model.Channel
    var err error
    if len(req.Ids) > 0 {
        channels = make([]*model.Channel, 0, len(req.Ids))
        for _, id := range req.Ids {
            ch, e := model.GetChannelById(id, true)
            if e == nil && ch != nil {
                channels = append(channels, ch)
            }
        }
    } else {
        // selectAll=true 以便不省略 key 字段
        channels, err = model.GetAllChannels(0, 0, true, false)
        if err != nil {
            c.JSON(http.StatusOK, gin.H{
                "success": false,
                "message": err.Error(),
            })
            return
        }
    }

    // 工具函数：根据渠道类型与 BaseURL 生成 /models URL
    buildModelsURL := func(ch *model.Channel) string {
        baseURL := common.ChannelBaseURLs[ch.Type]
        if ch.GetBaseURL() != "" {
            baseURL = ch.GetBaseURL()
        }
        url := fmt.Sprintf("%s/v1/models", baseURL)
        if strings.HasSuffix(baseURL, "/chat/completions") {
            url = strings.TrimSuffix(baseURL, "/chat/completions") + "/models"
        }
        if ch.Type == common.ChannelTypeGemini {
            url = fmt.Sprintf("%s/v1beta/openai/models", baseURL)
        }
        if ch.Type == common.ChannelTypeGitHub {
            url = strings.Replace(baseURL, "/inference", "/catalog/models", 1)
        }
        return url
    }

    // 工具函数：请求上游并解析模型 ID 列表
    fetchUpstream := func(ch *model.Channel) ([]string, error) {
        key := strings.Split(ch.Key, ",")[0]
        url := buildModelsURL(ch)
        // 直接使用 GetResponseBody 保持与单通道查询一致的身份头等细节
        body, err := GetResponseBody("GET", url, ch, GetAuthHeader(key))
        if err != nil {
            return nil, err
        }
        // GitHub 裸数组
        if ch.Type == common.ChannelTypeGitHub {
            var arr []struct{ ID string `json:"id"` }
            if e := json.Unmarshal(body, &arr); e != nil {
                return nil, fmt.Errorf("解析 GitHub 响应失败: %s", e.Error())
            }
            ids := make([]string, 0, len(arr))
            for _, m := range arr {
                ids = append(ids, m.ID)
            }
            return ids, nil
        }
        var result OpenAIModelsResponse
        if e := json.Unmarshal(body, &result); e != nil {
            return nil, fmt.Errorf("解析响应失败: %s", e.Error())
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

    results := make([]ChannelSyncResult, 0, len(channels))
    success := 0
    failed := 0
    changedGroups := make(map[string]struct{})

    for _, ch := range channels {
        if ch == nil {
            continue
        }
        res := ChannelSyncResult{
            ChannelID:   ch.Id,
            ChannelName: ch.Name,
            Mode:        mode,
            Group:       ch.Group,
        }
        // 当前本地模型
        current := ch.GetModels()
        res.BeforeCount = len(current)

        upstream, err := fetchUpstream(ch)
        if err != nil {
            res.Error = err.Error()
            results = append(results, res)
            failed++
            common.SysError(fmt.Sprintf("[模型同步] 渠道 %d(%s) 获取上游模型失败: %s", ch.Id, ch.Name, err.Error()))
            continue
        }
        res.UpstreamCount = len(upstream)
        if len(upstream) == 0 {
            // 上游无返回，跳过，避免误清空
            res.Error = "上游返回空模型列表，已跳过"
            results = append(results, res)
            failed++
            continue
        }

        upstreamSet := make(map[string]struct{}, len(upstream))
        for _, m := range upstream {
            m = strings.TrimSpace(m)
            if m != "" {
                upstreamSet[m] = struct{}{}
            }
        }

        var next []string
        removed := make([]string, 0)
        added := make([]string, 0)

        if mode == "incremental" {
            // 仅保留仍存在于上游的本地模型
            for _, m := range current {
                m = strings.TrimSpace(m)
                if _, ok := upstreamSet[m]; ok {
                    next = append(next, m)
                } else {
                    removed = append(removed, m)
                }
            }
        } else { // replace
            // 完全替换为上游
            next = make([]string, 0, len(upstream))
            currSet := make(map[string]struct{}, len(current))
            for _, m := range current {
                currSet[strings.TrimSpace(m)] = struct{}{}
            }
            for _, m := range upstream {
                m = strings.TrimSpace(m)
                if m != "" {
                    next = append(next, m)
                    if _, ok := currSet[m]; !ok {
                        added = append(added, m)
                    }
                }
            }
            // 统计被移除的（当前有、上游无）
            for _, m := range current {
                m = strings.TrimSpace(m)
                if m == "" {
                    continue
                }
                if _, ok := upstreamSet[m]; !ok {
                    removed = append(removed, m)
                }
            }
        }

        // 若本地与新列表一致，则不写库
        res.AfterCount = len(next)
        res.RemovedModels = removed
        res.AddedModels = added

        // 比较是否发生变化
        equal := func(a, b []string) bool {
            if len(a) != len(b) {
                return false
            }
            // 有序比较：保持与现有存储格式（逗号分隔）一致，不打乱顺序
            for i := range a {
                if strings.TrimSpace(a[i]) != strings.TrimSpace(b[i]) {
                    return false
                }
            }
            return true
        }

        if !equal(current, next) {
            // 更新 DB
            ch.Models = strings.Join(next, ",")
            if e := ch.Update(); e != nil {
                res.Error = e.Error()
                results = append(results, res)
                failed++
                common.SysError(fmt.Sprintf("[模型同步] 渠道 %d(%s) 写库失败: %s", ch.Id, ch.Name, e.Error()))
                continue
            }
            // 刷新该渠道所在分组的前缀缓存
            middleware.RefreshPrefixChannelsCache(ch.Group)
        }

        success++
        results = append(results, res)
    }

    resp := SyncResponse{
        Total:   len(channels),
        Success: success,
        Failed:  failed,
        Mode:    mode,
        Results: results,
    }
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "",
        "data":    resp,
    })
}

func SearchChannels(c *gin.Context) {
	keyword := c.Query("keyword")
	group := c.Query("group")
	modelKeyword := c.Query("model")
	idSort, _ := strconv.ParseBool(c.Query("id_sort"))
	enableTagMode, _ := strconv.ParseBool(c.Query("tag_mode"))

	// 新增：解析多选过滤参数 types/statuses/tags（CSV）
	parseCSVInts := func(s string) []int {
		if s == "" {
			return nil
		}
		parts := strings.Split(s, ",")
		res := make([]int, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if v, err := strconv.Atoi(p); err == nil {
				res = append(res, v)
			}
		}
		return res
	}
	parseCSVStr := func(s string) []string {
		if s == "" {
			return nil
		}
		parts := strings.Split(s, ",")
		res := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				res = append(res, p)
			}
		}
		return res
	}
	types := parseCSVInts(c.Query("types"))
	statuses := parseCSVInts(c.Query("statuses"))
	tagsFilter := parseCSVStr(c.Query("tags"))

	channelData := make([]*model.Channel, 0)
	if enableTagMode {
		// 先筛选标签集合，然后再聚合对应的渠道（保持与原有行为一致）
		tags, err := model.SearchTags(keyword, group, modelKeyword, idSort, types, statuses, tagsFilter)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		for _, tag := range tags {
			if tag != nil && *tag != "" {
				tagChannel, err := model.GetChannelsByTag(*tag, idSort)
				if err == nil {
					// 追加类型/状态/标签过滤（keyword/model 已在标签层过滤）
					for _, ch := range tagChannel {
						if len(types) > 0 {
							ok := false
							for _, t := range types {
								if ch.Type == t {
									ok = true
									break
								}
							}
							if !ok {
								continue
							}
						}
						if len(statuses) > 0 {
							ok := false
							for _, s := range statuses {
								if ch.Status == s {
									ok = true
									break
								}
							}
							if !ok {
								continue
							}
						}
						if len(tagsFilter) > 0 {
							ok := false
							for _, tg := range tagsFilter {
								if ch.GetTag() == tg {
									ok = true
									break
								}
							}
							if !ok {
								continue
							}
						}
						channelData = append(channelData, ch)
					}
				}
			}
		}
	} else {
		channels, err := model.SearchChannels(keyword, group, modelKeyword, idSort, types, statuses, tagsFilter)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		channelData = channels
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    channelData,
	})
	return
}

func GetChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	channel, err := model.GetChannelById(id, true) // Make sure to get full channel info
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    channel, // The key will be included in the response
	})
	return
}

func AddChannel(c *gin.Context) {
	channel := model.Channel{}
	err := c.ShouldBindJSON(&channel)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	channel.CreatedTime = common.GetTimestamp()

	// 特殊处理 VertexAi 类型的渠道
	if channel.Type == common.ChannelTypeVertexAi {
		if channel.Other == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "部署地区不能为空",
			})
			return
		} else {
			if common.IsJsonStr(channel.Other) {
				regionMap := common.StrToMap(channel.Other)
				if regionMap["default"] == nil {
					c.JSON(http.StatusOK, gin.H{
						"success": false,
						"message": "部署地区必须包含default字段",
					})
					return
				}
			}
		}
	}

	// 验证模型名称长度
	models := strings.Split(channel.Models, ",")
	for _, model := range models {
		if len(model) > 255 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("模型名称过长: %s", model),
			})
			return
		}
	}

	err = channel.Insert() // 使用 Insert 方法替代 InsertChannel
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// refresh prefix cache for the groups this channel belongs to
	middleware.RefreshPrefixChannelsCache(channel.Group)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func DeleteChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	channel := model.Channel{Id: id}
	err := channel.Delete()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func DeleteDisabledChannel(c *gin.Context) {
	rows, err := model.DeleteDisabledChannel()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rows,
	})
	return
}

type ChannelTag struct {
	Tag          string  `json:"tag"`
	NewTag       *string `json:"new_tag"`
	Priority     *int64  `json:"priority"`
	Weight       *uint   `json:"weight"`
	ModelMapping *string `json:"model_mapping"`
	Models       *string `json:"models"`
	Groups       *string `json:"groups"`
}

func DisableTagChannels(c *gin.Context) {
	channelTag := ChannelTag{}
	err := c.ShouldBindJSON(&channelTag)
	if err != nil || channelTag.Tag == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}
	err = model.DisableChannelByTag(channelTag.Tag)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if channels, err := model.GetChannelsByTag(channelTag.Tag, false); err == nil {
		groupSet := make(map[string]struct{})
		for _, ch := range channels {
			for _, g := range strings.Split(ch.Group, ",") {
				g = strings.TrimSpace(g)
				if g != "" {
					groupSet[g] = struct{}{}
				}
			}
		}
		for g := range groupSet {
			middleware.RefreshPrefixChannelsCache(g)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func EnableTagChannels(c *gin.Context) {
	channelTag := ChannelTag{}
	err := c.ShouldBindJSON(&channelTag)
	if err != nil || channelTag.Tag == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}
	err = model.EnableChannelByTag(channelTag.Tag)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if channels, err := model.GetChannelsByTag(channelTag.Tag, false); err == nil {
		groupSet := make(map[string]struct{})
		for _, ch := range channels {
			for _, g := range strings.Split(ch.Group, ",") {
				g = strings.TrimSpace(g)
				if g != "" {
					groupSet[g] = struct{}{}
				}
			}
		}
		for g := range groupSet {
			middleware.RefreshPrefixChannelsCache(g)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func EditTagChannels(c *gin.Context) {
	channelTag := ChannelTag{}
	err := c.ShouldBindJSON(&channelTag)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}
	if channelTag.Tag == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "tag不能为空",
		})
		return
	}
	err = model.EditChannelByTag(channelTag.Tag, channelTag.NewTag, channelTag.ModelMapping, channelTag.Models, channelTag.Groups, channelTag.Priority, channelTag.Weight)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

type ChannelBatch struct {
	Ids []int   `json:"ids"`
	Tag *string `json:"tag"`
}

func DeleteChannelBatch(c *gin.Context) {
	channelBatch := ChannelBatch{}
	err := c.ShouldBindJSON(&channelBatch)
	if err != nil || len(channelBatch.Ids) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}
	err = model.BatchDeleteChannels(channelBatch.Ids)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    len(channelBatch.Ids),
	})
	return
}

func UpdateChannel(c *gin.Context) {
	channel := model.Channel{}
	err := c.ShouldBindJSON(&channel)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if channel.Type == common.ChannelTypeVertexAi {
		if channel.Other == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "部署地区不能为空",
			})
			return
		} else {
			if common.IsJsonStr(channel.Other) {
				// must have default
				regionMap := common.StrToMap(channel.Other)
				if regionMap["default"] == nil {
					c.JSON(http.StatusOK, gin.H{
						"success": false,
						"message": "部署地区必须包含default字段",
					})
					return
				}
			}
		}
	}
	err = channel.Update()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// refresh prefix cache as channel configuration may change
	middleware.RefreshPrefixChannelsCache(channel.Group)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    channel,
	})
	return
}

func FetchModels(c *gin.Context) {
	var req struct {
		BaseURL string `json:"base_url"`
		Type    int    `json:"type"`
		Key     string `json:"key"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request",
		})
		return
	}

	baseURL := req.BaseURL
	if baseURL == "" {
		baseURL = common.ChannelBaseURLs[req.Type]
	}

	client := &http.Client{}
	url := fmt.Sprintf("%s/v1/models", baseURL)

	if strings.HasSuffix(baseURL, "/chat/completions") {
		url = strings.TrimSuffix(baseURL, "/chat/completions") + "/models"
	}

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// remove line breaks and extra spaces.
	key := strings.TrimSpace(req.Key)
	// If the key contains a line break, only take the first part.
	key = strings.Split(key, "\n")[0]
	request.Header.Set("Authorization", "Bearer "+key)

	response, err := client.Do(request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	//check status code
	if response.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch models",
		})
		return
	}
	defer response.Body.Close()

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	var models []string
	for _, model := range result.Data {
		models = append(models, model.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    models,
	})
}

func BatchSetChannelTag(c *gin.Context) {
	channelBatch := ChannelBatch{}
	err := c.ShouldBindJSON(&channelBatch)
	if err != nil || len(channelBatch.Ids) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}
	err = model.BatchSetChannelTag(channelBatch.Ids, channelBatch.Tag)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    len(channelBatch.Ids),
	})
	return
}
