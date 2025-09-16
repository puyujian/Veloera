package controller

import (
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "path"
    "strings"
    "veloera/common"
    "veloera/middleware"
    "veloera/model"

    "github.com/gin-gonic/gin"
)


var unsafeRedirectModelKeys = map[string]struct{}{
    "__proto__":   {},
    "prototype":   {},
    "constructor": {},
}

type channelRedirectMapping struct {
    aliasToActual map[string]string
    actualToAlias map[string]string
}

func buildChannelRedirectMapping(ch *model.Channel) *channelRedirectMapping {
    if ch == nil || ch.ModelMapping == nil {
        return nil
    }
    raw := strings.TrimSpace(*ch.ModelMapping)
    if raw == "" || raw == "{}" {
        return nil
    }

    tmp := make(map[string]interface{})
    if err := json.Unmarshal([]byte(raw), &tmp); err != nil {
        common.SysError(fmt.Sprintf("[模型同步] 渠道 %d(%s) 模型重定向解析失败: %v", ch.Id, ch.Name, err))
        return nil
    }

    aliasToActual := make(map[string]string, len(tmp))
    actualToAlias := make(map[string]string, len(tmp))
    for alias, value := range tmp {
        trimmedAlias := strings.TrimSpace(alias)
        if trimmedAlias == "" {
            continue
        }
        if _, blocked := unsafeRedirectModelKeys[trimmedAlias]; blocked {
            continue
        }

        actual := ""
        switch v := value.(type) {
        case string:
            actual = strings.TrimSpace(v)
        default:
            continue
        }
        if actual == "" {
            continue
        }

        aliasToActual[trimmedAlias] = actual
        if _, exists := actualToAlias[actual]; !exists {
            actualToAlias[actual] = trimmedAlias
        }
    }

    if len(aliasToActual) == 0 {
        return nil
    }

    return &channelRedirectMapping{
        aliasToActual: aliasToActual,
        actualToAlias: actualToAlias,
    }
}

func (m *channelRedirectMapping) resolveActual(name string) string {
    trimmed := strings.TrimSpace(name)
    if trimmed == "" {
        return ""
    }
    if actual, ok := m.aliasToActual[trimmed]; ok && actual != "" {
        return actual
    }
    return trimmed
}

func (m *channelRedirectMapping) resolveDisplay(actual string) string {
    trimmed := strings.TrimSpace(actual)
    if trimmed == "" {
        return ""
    }
    if alias, ok := m.actualToAlias[trimmed]; ok && alias != "" {
        return alias
    }
    return trimmed
}

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
        baseURL := strings.TrimSpace(ch.GetBaseURL())
        if baseURL == "" {
            if ch.Type < len(common.ChannelBaseURLs) && common.ChannelBaseURLs[ch.Type] != "" {
                baseURL = strings.TrimSpace(common.ChannelBaseURLs[ch.Type])
            }
        }
        if baseURL == "" {
            return ""
        }
        baseURL = strings.TrimRight(baseURL, "/")

        safeJoin := func(base string, segments ...string) string {
            parsed, err := url.Parse(base)
            if err != nil {
                trimmed := strings.TrimRight(base, "/")
                if trimmed == "" {
                    return ""
                }
                return trimmed + "/" + strings.Join(segments, "/")
            }
            joined := append([]string{parsed.Path}, segments...)
            parsed.Path = path.Join(joined...)
            return parsed.String()
        }

        switch ch.Type {
        case common.ChannelTypeGemini:
            return safeJoin(baseURL, "v1beta", "openai", "models")
        case common.ChannelTypeGitHub:
            parsed, err := url.Parse(baseURL)
            if err != nil {
                trimmed := strings.TrimRight(baseURL, "/")
                trimmed = strings.TrimSuffix(trimmed, "/inference")
                trimmed = strings.TrimRight(trimmed, "/")
                if trimmed == "" {
                    return ""
                }
                return trimmed + "/catalog/models"
            }
            cleanedPath := strings.TrimSuffix(parsed.Path, "/")
            if strings.HasSuffix(cleanedPath, "/inference") {
                cleanedPath = strings.TrimSuffix(cleanedPath, "/inference")
            }
            parsed.Path = path.Join(cleanedPath, "catalog", "models")
            return parsed.String()
        default:
            return safeJoin(baseURL, "v1", "models")
        }
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
            upstreamSet := make(map[string]struct{}, len(upstream))
            for _, m := range upstream {
                if mm := strings.TrimSpace(m); mm != "" {
                    upstreamSet[mm] = struct{}{}
                }
            }

            var next []string
            removed := make([]string, 0)
            added := make([]string, 0)
            redirectMapping := buildChannelRedirectMapping(ch)

            var seenNext map[string]struct{}
            var addedSet map[string]struct{}
            var removedSet map[string]struct{}
            if redirectMapping != nil {
                seenNext = make(map[string]struct{})
                addedSet = make(map[string]struct{})
                removedSet = make(map[string]struct{})
            }

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
            displayName := func(actual string) string {
                trimmed := strings.TrimSpace(actual)
                if trimmed == "" {
                    return ""
                }
                if redirectMapping != nil {
                    if alias := redirectMapping.resolveDisplay(trimmed); alias != "" {
                        return alias
                    }
                }
                return trimmed
            }
            appendNext := func(name string) {
                if name == "" {
                    return
                }
                if seenNext != nil {
                    if _, exists := seenNext[name]; exists {
                        return
                    }
                    seenNext[name] = struct{}{}
                }
                next = append(next, name)
            }
            appendAdded := func(name string) {
                if name == "" {
                    return
                }
                if addedSet != nil {
                    if _, exists := addedSet[name]; exists {
                        return
                    }
                    addedSet[name] = struct{}{}
                }
                added = append(added, name)
            }
            appendRemoved := func(name string) {
                if name == "" {
                    return
                }
                if removedSet != nil {
                    if _, exists := removedSet[name]; exists {
                        return
                    }
                    removedSet[name] = struct{}{}
                }
                removed = append(removed, name)
            }

            if mode == "incremental" {
                for _, m := range current {
                    trimmed := strings.TrimSpace(m)
                    if trimmed == "" {
                        continue
                    }
                    actualName := normalizeName(trimmed)
                    if actualName == "" {
                        actualName = trimmed
                    }
                    if _, ok := upstreamSet[actualName]; ok {
                        nameForNext := trimmed
                        if redirectMapping != nil {
                            if candidate := displayName(actualName); candidate != "" {
                                nameForNext = candidate
                            }
                        }
                        appendNext(strings.TrimSpace(nameForNext))
                    } else {
                        nameForRemoved := trimmed
                        if redirectMapping != nil {
                            if candidate := displayName(actualName); candidate != "" {
                                nameForRemoved = candidate
                            }
                        }
                        appendRemoved(strings.TrimSpace(nameForRemoved))
                    }
                }
            } else { // full
                currActualSet := make(map[string]struct{}, len(current))
                for _, m := range current {
                    trimmed := strings.TrimSpace(m)
                    if trimmed == "" {
                        continue
                    }
                    actualName := normalizeName(trimmed)
                    if actualName == "" {
                        actualName = trimmed
                    }
                    currActualSet[actualName] = struct{}{}
                }
                for _, m := range upstream {
                    actualName := strings.TrimSpace(m)
                    if actualName == "" {
                        continue
                    }
                    nameForNext := actualName
                    if redirectMapping != nil {
                        if candidate := displayName(actualName); candidate != "" {
                            nameForNext = candidate
                        }
                    }
                    appendNext(strings.TrimSpace(nameForNext))
                    if _, ok := currActualSet[actualName]; !ok {
                        appendAdded(strings.TrimSpace(nameForNext))
                    }
                }
                for _, m := range current {
                    trimmed := strings.TrimSpace(m)
                    if trimmed == "" {
                        continue
                    }
                    actualName := normalizeName(trimmed)
                    if actualName == "" {
                        actualName = trimmed
                    }
                    if _, ok := upstreamSet[actualName]; !ok {
                        nameForRemoved := trimmed
                        if redirectMapping != nil {
                            if candidate := displayName(actualName); candidate != "" {
                                nameForRemoved = candidate
                            }
                        }
                        appendRemoved(strings.TrimSpace(nameForRemoved))
                    }
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
