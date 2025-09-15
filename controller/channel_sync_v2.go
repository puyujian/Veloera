package controller

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "veloera/common"
    "veloera/middleware"
    "veloera/model"

    "github.com/gin-gonic/gin"
)

// SyncChannelModelsV2：并发抓取上游，仅返回预览结果，不落库
func SyncChannelModelsV2(c *gin.Context) {
    type SyncRequest struct {
        Mode        string `json:"mode"`        // incremental | full（兼容 replace）
        Ids         []int  `json:"ids"`         // 指定渠道；为空则全部
        EnabledOnly bool   `json:"enabled_only"`// 仅同步启用渠道
        Concurrency int    `json:"concurrency"` // 并发度，默认 8
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
        NextModels      []string `json:"next_models"`
        Error           string   `json:"error,omitempty"`
        Group           string   `json:"group"`
        UpstreamPreview []string `json:"upstream_preview,omitempty"`
    }
    type SyncResponse struct {
        Total   int                 `json:"total"`
        Success int                 `json:"success"`
        Failed  int                 `json:"failed"`
        Mode    string              `json:"mode"`
        Results []ChannelSyncResult `json:"results"`
    }

    var req SyncRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误"})
        return
    }
    mode := strings.ToLower(strings.TrimSpace(req.Mode))
    if mode == "replace" { // 兼容旧命名
        mode = "full"
    }
    if mode != "incremental" && mode != "full" {
        c.JSON(http.StatusOK, gin.H{"success": false, "message": "mode 仅支持 incremental 或 full"})
        return
    }
    conc := req.Concurrency
    if conc <= 0 { conc = 8 }
    if conc > 32 { conc = 32 }

    // 读取渠道
    var channels []*model.Channel
    var err error
    if len(req.Ids) > 0 {
        channels = make([]*model.Channel, 0, len(req.Ids))
        for _, id := range req.Ids {
            ch, e := model.GetChannelById(id, true)
            if e != nil {
                common.SysError(fmt.Sprintf("[模型同步] 获取渠道 %d 时发生错误: %s", id, e.Error()))
                continue
            }
            if ch == nil {
                common.SysError(fmt.Sprintf("[模型同步] 渠道 %d 不存在，已跳过", id))
                continue
            }
            channels = append(channels, ch)
        }
    } else {
        channels, err = model.GetAllChannels(0, 0, true, false)
        if err != nil {
            c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
            return
        }
    }
    if req.EnabledOnly {
        filtered := make([]*model.Channel, 0, len(channels))
        for _, ch := range channels {
            if ch != nil && ch.Status == common.ChannelStatusEnabled {
                filtered = append(filtered, ch)
            }
        }
        channels = filtered
    }

    // 构建上游 URL
    buildModelsURL := func(ch *model.Channel) string {
        baseURL := common.ChannelBaseURLs[ch.Type]
        if ch.GetBaseURL() != "" { baseURL = ch.GetBaseURL() }
        url := fmt.Sprintf("%s/v1/models", baseURL)
        if strings.HasSuffix(baseURL, "/chat/completions") { url = strings.TrimSuffix(baseURL, "/chat/completions") + "/models" }
        if ch.Type == common.ChannelTypeGemini { url = fmt.Sprintf("%s/v1beta/openai/models", baseURL) }
        if ch.Type == common.ChannelTypeGitHub { url = strings.Replace(baseURL, "/inference", "/catalog/models", 1) }
        return url
    }
    // 拉取上游模型 ID 列表
    fetchUpstream := func(ch *model.Channel) ([]string, error) {
        key := strings.Split(ch.Key, ",")[0]
        url := buildModelsURL(ch)
        body, err := GetResponseBody("GET", url, ch, GetAuthHeader(key))
        if err != nil {
            return nil, fmt.Errorf("请求上游失败 (渠道 %d: %s, URL: %s): %w", ch.Id, ch.Name, url, err)
        }
        if ch.Type == common.ChannelTypeGitHub {
            var arr []struct{ ID string `json:"id"` }
            if e := json.Unmarshal(body, &arr); e != nil {
                return nil, fmt.Errorf("解析 GitHub 响应失败 (渠道 %d: %s, URL: %s): %w", ch.Id, ch.Name, url, e)
            }
            ids := make([]string, 0, len(arr))
            for _, m := range arr { ids = append(ids, m.ID) }
            return ids, nil
        }
        var result OpenAIModelsResponse
        if e := json.Unmarshal(body, &result); e != nil {
            return nil, fmt.Errorf("解析 OpenAI 响应失败 (渠道 %d: %s, 类型: %d, URL: %s): %w", ch.Id, ch.Name, ch.Type, url, e)
        }
        ids := make([]string, 0, len(result.Data))
        for _, m := range result.Data {
            id := m.ID
            if ch.Type == common.ChannelTypeGemini { id = strings.TrimPrefix(id, "models/") }
            ids = append(ids, id)
        }
        return ids, nil
    }

    // 并发抓取与计算
    results := make([]ChannelSyncResult, 0, len(channels))
    success := 0
    failed := 0
    sem := make(chan struct{}, conc)
    done := make(chan ChannelSyncResult, len(channels))

    tasks := 0
    for _, ch := range channels {
        ch := ch
        if ch == nil { continue }
        sem <- struct{}{}
        go func() {
            defer func() { <-sem }()
            res := ChannelSyncResult{ChannelID: ch.Id, ChannelName: ch.Name, Mode: mode, Group: ch.Group}
            current := ch.GetModels()
            res.BeforeCount = len(current)

            upstream, err := fetchUpstream(ch)
            if err != nil {
                res.Error = err.Error()
                common.SysError(fmt.Sprintf("[模型同步] 渠道 %d(%s) 获取上游模型失败: %s", ch.Id, ch.Name, err.Error()))
                done <- res
                return
            }
            res.UpstreamCount = len(upstream)
            res.UpstreamPreview = upstream
            if len(upstream) == 0 {
                res.Error = "上游返回空模型列表，已跳过"
                done <- res
                return
            }
            upstreamSet := make(map[string]struct{}, len(upstream))
            for _, m := range upstream { if mm := strings.TrimSpace(m); mm != "" { upstreamSet[mm] = struct{}{} } }

            var next []string
            removed := make([]string, 0)
            added := make([]string, 0)
            if mode == "incremental" {
                for _, m := range current {
                    m = strings.TrimSpace(m)
                    if _, ok := upstreamSet[m]; ok { next = append(next, m) } else { removed = append(removed, m) }
                }
            } else { // full
                next = make([]string, 0, len(upstream))
                currSet := make(map[string]struct{}, len(current))
                for _, m := range current { currSet[strings.TrimSpace(m)] = struct{}{} }
                for _, m := range upstream {
                    m = strings.TrimSpace(m)
                    if m != "" {
                        next = append(next, m)
                        if _, ok := currSet[m]; !ok { added = append(added, m) }
                    }
                }
                for _, m := range current {
                    m = strings.TrimSpace(m)
                    if m == "" { continue }
                    if _, ok := upstreamSet[m]; !ok { removed = append(removed, m) }
                }
            }
            res.AfterCount = len(next)
            res.RemovedModels = removed
            res.AddedModels = added
            res.NextModels = next
            done <- res
        }()
        tasks++
    }
    for i := 0; i < tasks; i++ {
        r := <-done
        if r.Error != "" { failed++ } else { success++ }
        results = append(results, r)
    }

    c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": SyncResponse{Total: len(channels), Success: success, Failed: failed, Mode: mode, Results: results}})
}

// ApplyChannelModelSync：将预览结果保存入库
func ApplyChannelModelSync(c *gin.Context) {
    type ApplyItem struct { ChannelID int `json:"channel_id"`; NextModels []string `json:"next_models"` }
    type ApplyRequest struct { Items []ApplyItem `json:"items"` }
    type ApplyResult struct { ChannelID int `json:"channel_id"`; ChannelName string `json:"channel_name"`; Applied bool `json:"applied"`; Error string `json:"error,omitempty"` }
    type ApplyResponse struct { Total int `json:"total"`; Success int `json:"success"`; Failed int `json:"failed"`; Results []ApplyResult `json:"results"` }

    var req ApplyRequest
    if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误或空列表"})
        return
    }
    results := make([]ApplyResult, 0, len(req.Items))
    success := 0
    failed := 0
    for _, item := range req.Items {
        ch, err := model.GetChannelById(item.ChannelID, true)
        if err != nil {
            common.SysError(fmt.Sprintf("[模型同步保存] 获取渠道 %d 时发生错误: %s", item.ChannelID, err.Error()))
            results = append(results, ApplyResult{ChannelID: item.ChannelID, Applied: false, Error: fmt.Sprintf("获取渠道失败: %s", err.Error())})
            failed++
            continue
        }
        if ch == nil {
            common.SysError(fmt.Sprintf("[模型同步保存] 渠道 %d 不存在", item.ChannelID))
            results = append(results, ApplyResult{ChannelID: item.ChannelID, Applied: false, Error: "渠道不存在"})
            failed++
            continue
        }
        ch.Models = strings.Join(item.NextModels, ",")
        if e := ch.Update(); e != nil {
            results = append(results, ApplyResult{ChannelID: item.ChannelID, ChannelName: ch.Name, Applied: false, Error: e.Error()})
            failed++
            common.SysError(fmt.Sprintf("[模型同步保存] 渠道 %d(%s) 写库失败: %s", ch.Id, ch.Name, e.Error()))
            continue
        }
        middleware.RefreshPrefixChannelsCache(ch.Group)
        results = append(results, ApplyResult{ChannelID: item.ChannelID, ChannelName: ch.Name, Applied: true})
        success++
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": ApplyResponse{Total: len(req.Items), Success: success, Failed: failed, Results: results}})
}
