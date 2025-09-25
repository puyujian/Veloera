package controller

import (
    "encoding/csv"
    "fmt"
    "math"
    "net/http"
    "strconv"
    "strings"
    "time"
    "veloera/channeltest"
    "veloera/model"

    "github.com/gin-gonic/gin"
)

type batchModelTestRequest struct {
    ChannelIDs        []int    `json:"channel_ids"`
    IncludeAll        bool     `json:"include_all"`
    IncludeDisabled   bool     `json:"include_disabled"`
    ModelScope        string   `json:"model_scope"`
    ModelWhitelist    []string `json:"model_whitelist"`
    ModelBlacklist    []string `json:"model_blacklist"`
    TestMode          string   `json:"test_mode"`
    TargetModels      []string `json:"target_models"`
    UseChannelDefault bool     `json:"use_channel_default"`
    Concurrency       int      `json:"concurrency"`
    IntervalMs        int      `json:"interval_ms"`
    RetryLimit        int      `json:"retry_limit"`
}

type batchDeleteFailedRequest struct {
    ChannelIDs []int `json:"channel_ids"`
    DryRun     bool  `json:"dry_run"`
}

func sanitizeStringList(list []string) []string {
    if len(list) == 0 {
        return []string{}
    }
    cleaned := make([]string, 0, len(list))
    seen := make(map[string]struct{})
    for _, item := range list {
        trimmed := strings.TrimSpace(item)
        if trimmed == "" {
            continue
        }
        if _, ok := seen[trimmed]; ok {
            continue
        }
        seen[trimmed] = struct{}{}
        cleaned = append(cleaned, trimmed)
    }
    return cleaned
}

// StartChannelBatchTest 启动批量模型测试任务
func StartChannelBatchTest(c *gin.Context) {
    var req batchModelTestRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": fmt.Sprintf("参数解析失败: %v", err),
        })
        return
    }

    if !req.IncludeAll && len(req.ChannelIDs) == 0 {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": "请至少选择一个渠道或开启全量测试",
        })
        return
    }

    testMode := model.ChannelTestJobMode(strings.ToLower(strings.TrimSpace(req.TestMode)))
    if testMode != model.ChannelTestJobModeSelected {
        testMode = model.ChannelTestJobModeAll
    }
    whitelist := sanitizeStringList(req.ModelWhitelist)
    blacklist := sanitizeStringList(req.ModelBlacklist)
    targetModels := sanitizeStringList(req.TargetModels)

    if testMode == model.ChannelTestJobModeSelected && len(targetModels) == 0 {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": "请至少选择一个需要测试的模型",
        })
        return
    }

    job := &model.ChannelTestJob{
        RequestedBy: c.GetInt("id"),
        Concurrency: req.Concurrency,
        IntervalMs:  req.IntervalMs,
        RetryLimit:  req.RetryLimit,
        Status:      model.ChannelTestJobStatusPending,
    }

    options := model.DefaultChannelTestJobOptions()
    options.ChannelIDs = req.ChannelIDs
    options.IncludeAll = req.IncludeAll
    options.IncludeDisabled = req.IncludeDisabled
    if trimmedScope := strings.TrimSpace(req.ModelScope); trimmedScope != "" {
        options.ModelScope = trimmedScope
    }
    options.ModelWhitelist = whitelist
    options.ModelBlacklist = blacklist
    options.UseChannelDefault = req.UseChannelDefault
    options.TestMode = testMode
    options.TargetModels = targetModels

    if err := job.SetOptions(options); err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": fmt.Sprintf("保存任务配置失败: %v", err),
        })
        return
    }

    if err := model.CreateChannelTestJob(job); err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": fmt.Sprintf("创建任务失败: %v", err),
        })
        return
    }

    if err := channeltest.SubmitJob(job.ID); err != nil {
        _ = model.FinalizeChannelTestJob(job.ID, model.ChannelTestJobStatusFailed, err.Error())
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": fmt.Sprintf("任务提交失败: %v", err),
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "",
        "data": gin.H{
            "job_id": job.ID,
        },
    })
}

// GetChannelTestAvailableModels 获取批量测试可选模型列表
func GetChannelTestAvailableModels(c *gin.Context) {
	includeDisabled := false
	raw := strings.TrimSpace(c.Query("include_disabled"))
	if raw != "" {
		lower := strings.ToLower(raw)
		if lower == "1" || lower == "true" || lower == "yes" {
			includeDisabled = true
		}
	}

	models, err := model.GetAllChannelModels(includeDisabled)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("获取模型列表失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    models,
	})
}

// GetChannelBatchTestJobs 返回最近的批量模型测试任务列表
func GetChannelBatchTestJobs(c *gin.Context) {
    limit, _ := strconv.Atoi(c.Query("limit"))
    jobs, err := model.ListChannelTestJobs(limit)
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
        "data": gin.H{
            "jobs": jobs,
        },
    })
}

// GetChannelBatchTestJob 返回单个任务详情
func GetChannelBatchTestJob(c *gin.Context) {
    jobID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil || jobID <= 0 {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": "无效的任务 ID",
        })
        return
    }

    job, err := model.GetChannelTestJob(jobID)
    if err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": err.Error(),
        })
        return
    }

    options, _ := job.GetOptions()
    progress := 0.0
    if job.TotalModels > 0 {
        progress = float64(job.CompletedCount) / float64(job.TotalModels)
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "",
        "data": gin.H{
            "job":      job,
            "options":  options,
            "progress": progress,
        },
    })
}

// GetChannelBatchTestJobResults 分页返回任务结果
func GetChannelBatchTestJobResults(c *gin.Context) {
    jobID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil || jobID <= 0 {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": "无效的任务 ID",
        })
        return
    }

    page, _ := strconv.Atoi(c.Query("page"))
    if page <= 0 {
        page = 1
    }
    pageSize, _ := strconv.Atoi(c.Query("page_size"))
    if pageSize <= 0 || pageSize > 200 {
        pageSize = 50
    }
    offset := (page - 1) * pageSize

    results, total, err := model.ListChannelTestResults(jobID, offset, pageSize)
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
        "data": gin.H{
            "results": results,
            "total":   total,
            "page":    page,
            "page_size": pageSize,
        },
    })
}

// CancelChannelBatchTestJob 取消任务
func CancelChannelBatchTestJob(c *gin.Context) {
    jobID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil || jobID <= 0 {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": "无效的任务 ID",
        })
        return
    }

    if err := channeltest.CancelJob(jobID); err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": err.Error(),
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "任务取消指令已发送",
    })
}

// ExportChannelBatchTestJob 导出任务结果为 CSV
func ExportChannelBatchTestJob(c *gin.Context) {
    jobID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err !=  nil || jobID <= 0 {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": "无效的任务 ID",
        })
        return
    }

    if _, err := model.GetChannelTestJob(jobID); err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": err.Error(),
        })
        return
    }

    filename := fmt.Sprintf("channel-test-report-%d.csv", jobID)
    c.Header("Content-Type", "text/csv; charset=utf-8")
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

    writer := csv.NewWriter(c.Writer)
    defer writer.Flush()

    header := []string{"JobID", "ChannelID", "ChannelName", "Model", "Success", "Duration(ms)", "RetryCount", "ErrorMessage", "CreatedAt"}
    _ = writer.Write(header)

    batchSize := 500
    offset := 0
    for {
        results, _, err := model.ListChannelTestResults(jobID, offset, batchSize)
        if err != nil {
            c.Status(http.StatusInternalServerError)
            return
        }
        if len(results) == 0 {
            break
        }
        for _, item := range results {
            created := time.Unix(item.CreatedAt, 0).Format(time.RFC3339)
            row := []string{
                strconv.FormatInt(jobID, 10),
                strconv.Itoa(item.ChannelID),
                item.ChannelName,
                item.ModelName,
                strconv.FormatBool(item.Success),
                strconv.Itoa(item.DurationMillis),
                strconv.Itoa(item.RetryCount),
                item.ErrorMessage,
                created,
            }
            _ = writer.Write(row)
        }
        writer.Flush()
        if err := writer.Error(); err != nil {
            c.Status(http.StatusInternalServerError)
            return
        }
        if len(results) < batchSize {
            break
        }
        offset += batchSize
    }
}

// RetryChannelBatchTestResult 重新执行单条失败的测试结果
func RetryChannelBatchTestResult(c *gin.Context) {
    jobID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil || jobID <= 0 {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": "无效的任务 ID",
        })
        return
    }

    resultID, err := strconv.ParseInt(c.Param("resultId"), 10, 64)
    if err != nil || resultID <= 0 {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": "无效的结果 ID",
        })
        return
    }

    if _, err := model.GetChannelTestJob(jobID); err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": err.Error(),
        })
        return
    }

    result, err := model.GetChannelTestResult(resultID)
    if err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": err.Error(),
        })
        return
    }

    if result.JobID != jobID {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": "结果与任务不匹配",
        })
        return
    }

    channel, err := model.GetChannelById(result.ChannelID, true)
    if err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": fmt.Sprintf("获取渠道信息失败: %v", err),
        })
        return
    }

    consumed, execErr, openAIError := channeltest.ExecuteChannelTest(channel, result.ModelName)
    durationMillis := int(math.Round(consumed * 1000))
    retryCount := result.RetryCount + 1

    updates := map[string]any{
        "duration_millis": durationMillis,
        "retry_count":     retryCount,
        "created_at":      time.Now().Unix(),
    }

    responseMessage := "模型重试成功"

    if execErr != nil {
        updates["success"] = false
        if openAIError != nil && openAIError.Error.Message != "" {
            updates["error_message"] = fmt.Sprintf("%s (code %v)", openAIError.Error.Message, openAIError.Error.Code)
            responseMessage = fmt.Sprintf("模型重试失败：%s", openAIError.Error.Message)
        } else {
            updates["error_message"] = execErr.Error()
            responseMessage = fmt.Sprintf("模型重试失败：%s", execErr.Error())
        }
    } else {
        updates["success"] = true
        updates["error_message"] = ""
    }

    if err := model.UpdateChannelTestResult(resultID, updates); err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": fmt.Sprintf("更新结果失败: %v", err),
        })
        return
    }

    if err := model.RefreshChannelTestJobStats(jobID); err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": fmt.Sprintf("刷新任务统计失败: %v", err),
        })
        return
    }

    refreshed, err := model.GetChannelTestResult(resultID)
    if err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": fmt.Sprintf("获取更新后的结果失败: %v", err),
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": responseMessage,
        "data": gin.H{
            "result": refreshed,
        },
    })
}

// DeleteFailedModelsByJob 批量删除测试失败的模型
func DeleteFailedModelsByJob(c *gin.Context) {
    jobID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil || jobID <= 0 {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": "无效的任务 ID",
        })
        return
    }

    var req batchDeleteFailedRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": fmt.Sprintf("参数解析失败: %v", err),
        })
        return
    }

    job, err := model.GetChannelTestJob(jobID)
    if err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": err.Error(),
        })
        return
    }

    if job.Status == model.ChannelTestJobStatusRunning || job.Status == model.ChannelTestJobStatusPending {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": "任务未结束，无法执行批量删除",
        })
        return
    }

    query := model.DB.Where("job_id = ? AND success = ?", jobID, false)
    if len(req.ChannelIDs) > 0 {
        query = query.Where("channel_id IN ?", req.ChannelIDs)
    }

    var failedResults []model.ChannelTestResult
    if err := query.Find(&failedResults).Error; err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": fmt.Sprintf("查询失败模型列表时出错: %v", err),
        })
        return
    }

    if len(failedResults) == 0 {
        c.JSON(http.StatusOK, gin.H{
            "success": true,
            "message": "未找到需要删除的失败模型",
            "data": gin.H{
                "summary": []any{},
                "deleted": 0,
            },
        })
        return
    }

    channelFailedMap := make(map[int][]model.ChannelTestResult)
    for _, item := range failedResults {
        channelFailedMap[item.ChannelID] = append(channelFailedMap[item.ChannelID], item)
    }

    summary := make([]gin.H, 0, len(channelFailedMap))
    deletedCount := 0

    for channelID, records := range channelFailedMap {
        channel, err := model.GetChannelById(channelID, true)
        if err != nil {
            summary = append(summary, gin.H{
                "channel_id": channelID,
                "channel_name": records[0].ChannelName,
                "error": fmt.Sprintf("获取渠道信息失败: %v", err),
            })
            continue
        }

        failedModels := make(map[string]struct{})
        for _, r := range records {
            failedModels[r.ModelName] = struct{}{}
        }

        currentModels := channel.Models
        parts := strings.Split(currentModels, ",")
        kept := make([]string, 0, len(parts))
        removed := make([]string, 0, len(parts))
        seen := make(map[string]struct{})
        for _, part := range parts {
            name := strings.TrimSpace(part)
            if name == "" {
                continue
            }
            if _, ok := seen[name]; ok {
                continue
            }
            seen[name] = struct{}{}
            if _, failed := failedModels[name]; failed {
                removed = append(removed, name)
                continue
            }
            kept = append(kept, name)
        }

        if len(removed) == 0 {
            summary = append(summary, gin.H{
                "channel_id": channelID,
                "channel_name": channel.Name,
                "removed": []string{},
                "remaining": len(kept),
            })
            continue
        }

        if len(kept) == 0 {
            summary = append(summary, gin.H{
                "channel_id": channelID,
                "channel_name": channel.Name,
                "error": "删除失败模型后将没有可用模型，已跳过",
                "removed": removed,
            })
            continue
        }

        if !req.DryRun {
            newValue := strings.Join(kept, ",")
            if err := model.DB.Model(&model.Channel{}).
                Where("id = ?", channelID).
                Updates(map[string]any{
                    "models": newValue,
                    "test_time": time.Now().Unix(),
                }).Error; err != nil {
                summary = append(summary, gin.H{
                    "channel_id": channelID,
                    "channel_name": channel.Name,
                    "error": fmt.Sprintf("更新渠道模型失败: %v", err),
                    "removed": removed,
                })
                continue
            }
            deletedCount += len(removed)
        }

        summary = append(summary, gin.H{
            "channel_id": channelID,
            "channel_name": channel.Name,
            "removed": removed,
            "remaining": len(kept),
        })
    }

    message := "操作完成"
    if req.DryRun {
        message = "预览完成，可执行正式删除"
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": message,
        "data": gin.H{
            "summary": summary,
            "deleted": deletedCount,
            "dry_run": req.DryRun,
        },
    })
}
