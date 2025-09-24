package channeltest

import (
    "context"
    "errors"
    "fmt"
    "math"
    "strings"
    "sync"
    "sync/atomic"
    "time"
    "veloera/common"
    "veloera/model"

    "github.com/bytedance/gopkg/util/gopool"
)

type channelTestTask struct {
    channel *model.Channel
    model   string
}

type channelTestJobRuntime struct {
    jobID  int64
    ctx    context.Context
    cancel context.CancelFunc
}

type ChannelTestJobRunner struct {
    mu      sync.Mutex
    running map[int64]*channelTestJobRuntime
}

var (
    channelTestRunnerOnce sync.Once
    channelTestRunner     *ChannelTestJobRunner
)

// InitRunner 初始化批量模型测试调度器
func InitRunner() {
    channelTestRunnerOnce.Do(func() {
        channelTestRunner = &ChannelTestJobRunner{
            running: make(map[int64]*channelTestJobRuntime),
        }
        model.DB.Model(&model.ChannelTestJob{}).
            Where("status = ?", model.ChannelTestJobStatusRunning).
            Updates(map[string]any{
                "status":       model.ChannelTestJobStatusFailed,
                "error_message": "服务重启导致任务中断",
                "finished_at":  time.Now().Unix(),
                "updated_at":   time.Now().Unix(),
            })
    })
}

func getRunner() *ChannelTestJobRunner {
    if channelTestRunner == nil {
        InitRunner()
    }
    return channelTestRunner
}

// SubmitJob 将任务提交到后台执行
func SubmitJob(jobID int64) error {
    runner := getRunner()
    runner.mu.Lock()
    if _, exists := runner.running[jobID]; exists {
        runner.mu.Unlock()
        return errors.New("任务已在执行中")
    }
    ctx, cancel := context.WithCancel(context.Background())
    runtime := &channelTestJobRuntime{
        jobID:  jobID,
        ctx:    ctx,
        cancel: cancel,
    }
    runner.running[jobID] = runtime
    runner.mu.Unlock()

    gopool.Go(func() {
        runner.executeJob(runtime)
    })
    return nil
}

// CancelJob 取消正在执行的任务
func CancelJob(jobID int64) error {
    runner := getRunner()
    runner.mu.Lock()
    runtime, exists := runner.running[jobID]
    runner.mu.Unlock()
    if !exists {
        return errors.New("任务未在运行或已结束")
    }
    runtime.cancel()
    return nil
}

func (r *ChannelTestJobRunner) remove(jobID int64) {
    r.mu.Lock()
    delete(r.running, jobID)
    r.mu.Unlock()
}

func (r *ChannelTestJobRunner) executeJob(runtime *channelTestJobRuntime) {
    defer r.remove(runtime.jobID)

    job, err := model.GetChannelTestJob(runtime.jobID)
    if err != nil {
        _ = model.FinalizeChannelTestJob(runtime.jobID, model.ChannelTestJobStatusFailed, fmt.Sprintf("无法读取任务: %v", err))
        return
    }

    if job.Status != model.ChannelTestJobStatusPending {
        _ = model.FinalizeChannelTestJob(runtime.jobID, model.ChannelTestJobStatusFailed, "任务状态异常，无法执行")
        return
    }

    options, err := job.GetOptions()
    if err != nil {
        _ = model.FinalizeChannelTestJob(runtime.jobID, model.ChannelTestJobStatusFailed, fmt.Sprintf("解析任务配置失败: %v", err))
        return
    }

    channels, err := r.fetchChannels(options)
    if err != nil {
        _ = model.FinalizeChannelTestJob(runtime.jobID, model.ChannelTestJobStatusFailed, err.Error())
        return
    }
    if len(channels) == 0 {
        _ = model.FinalizeChannelTestJob(runtime.jobID, model.ChannelTestJobStatusFailed, "未找到可用渠道，请检查筛选条件")
        return
    }

    tasks := r.buildTasks(channels, options)
    if len(tasks) == 0 {
        _ = model.FinalizeChannelTestJob(runtime.jobID, model.ChannelTestJobStatusFailed, "未匹配到任何模型，无法执行测试")
        return
    }

    concurrency := sanitizeConcurrency(job.Concurrency)
    interval := sanitizeInterval(job.IntervalMs)
    retryLimit := sanitizeRetryLimit(job.RetryLimit)

    _ = model.UpdateChannelTestJob(job, map[string]any{
        "status":         model.ChannelTestJobStatusRunning,
        "started_at":     time.Now().Unix(),
        "total_channels": len(channels),
        "total_models":   len(tasks),
        "concurrency":    concurrency,
        "interval_ms":    interval,
        "retry_limit":    retryLimit,
    })

    taskCh := make(chan channelTestTask)
    var processed int64
    var skipped int64

    workerWG := sync.WaitGroup{}
    for i := 0; i < concurrency; i++ {
        workerWG.Add(1)
        go func() {
            defer workerWG.Done()
            for task := range taskCh {
                if runtime.ctx.Err() != nil {
                    atomic.AddInt64(&skipped, 1)
                    continue
                }

                _ = model.UpdateChannelTestJob(job, map[string]any{
                    "current_channel": task.channel.Id,
                    "current_model":   task.model,
                })

                outcome, canceled := r.executeSingleTask(runtime.ctx, job, task, retryLimit, interval)
                if canceled {
                    atomic.AddInt64(&skipped, 1)
                    continue
                }

                if outcome != nil {
                    _ = model.AddChannelTestResult(outcome)
                    _ = model.IncrementChannelTestJobCounters(job.ID, outcome.ChannelID, outcome.ModelName, outcome.Success)
                    atomic.AddInt64(&processed, 1)
                }
            }
        }()
    }

    dispatcherDone := make(chan struct{})
    go func() {
        defer close(taskCh)
        defer close(dispatcherDone)
        ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
        defer ticker.Stop()
        for idx, task := range tasks {
            if idx == 0 {
                if runtime.ctx.Err() != nil {
                    atomic.AddInt64(&skipped, int64(len(tasks)))
                    return
                }
                taskCh <- task
                continue
            }
            select {
            case <-runtime.ctx.Done():
                remaining := len(tasks) - idx
                if remaining > 0 {
                    atomic.AddInt64(&skipped, int64(remaining))
                }
                return
            case <-ticker.C:
                taskCh <- task
            }
        }
    }()

    workerWG.Wait()
    <-dispatcherDone

    totalTasks := len(tasks)
    completed := int(atomic.LoadInt64(&processed))
    canceledCount := int(atomic.LoadInt64(&skipped))

    if canceledCount > 0 {
        _ = model.IncreaseChannelTestJobCancelCount(job.ID, canceledCount)
    }

    if runtime.ctx.Err() != nil {
        _ = model.FinalizeChannelTestJob(job.ID, model.ChannelTestJobStatusCanceled, "任务已被用户取消")
        return
    }

    if completed == 0 {
        _ = model.FinalizeChannelTestJob(job.ID, model.ChannelTestJobStatusFailed, "未成功执行任何模型测试")
        return
    }

    if completed+canceledCount < totalTasks {
        _ = model.FinalizeChannelTestJob(job.ID, model.ChannelTestJobStatusFailed, "部分任务未完成，请重试")
        return
    }

    failureCount := job.FailureCount
    refreshedJob, err := model.GetChannelTestJob(job.ID)
    if err == nil {
        failureCount = refreshedJob.FailureCount
    }

    message := ""
    if failureCount > 0 {
        message = fmt.Sprintf("任务完成，其中 %d 个模型测试失败", failureCount)
    }

    _ = model.FinalizeChannelTestJob(job.ID, model.ChannelTestJobStatusSuccess, message)
}

func (r *ChannelTestJobRunner) fetchChannels(options model.ChannelTestJobOptions) ([]model.Channel, error) {
    query := model.DB.Model(&model.Channel{})

    if !options.IncludeAll {
        validIDs := make([]int, 0, len(options.ChannelIDs))
        seen := make(map[int]struct{})
        for _, id := range options.ChannelIDs {
            if id <= 0 {
                continue
            }
            if _, ok := seen[id]; ok {
                continue
            }
            seen[id] = struct{}{}
            validIDs = append(validIDs, id)
        }
        if len(validIDs) == 0 {
            return nil, errors.New("未选择任何渠道")
        }
        query = query.Where("id IN ?", validIDs)
    }

    if !options.IncludeDisabled {
        query = query.Where("status = ?", common.ChannelStatusEnabled)
    }

    var channels []model.Channel
    if err := query.Find(&channels).Error; err != nil {
        return nil, err
    }
    return channels, nil
}

func (r *ChannelTestJobRunner) buildTasks(channels []model.Channel, options model.ChannelTestJobOptions) []channelTestTask {
    tasks := make([]channelTestTask, 0)
    for idx := range channels {
        channel := &channels[idx]
        models := pickModelsForChannel(channel, options)
        for _, m := range models {
            tasks = append(tasks, channelTestTask{
                channel: channel,
                model:   m,
            })
        }
    }
    return tasks
}

func (r *ChannelTestJobRunner) executeSingleTask(ctx context.Context, job *model.ChannelTestJob, task channelTestTask, retryLimit int, interval int) (*model.ChannelTestResult, bool) {
    attempts := retryLimit + 1
    var lastErr error
    var openAIErrorMessage string
    var consumed float64

    for attempt := 0; attempt < attempts; attempt++ {
        if ctx.Err() != nil {
            return nil, true
        }
        consumed, lastErr, errWithCode := ExecuteChannelTest(task.channel, task.model)
        if errWithCode != nil && errWithCode.Error.Message != "" {
            openAIErrorMessage = fmt.Sprintf("%s (code %v)", errWithCode.Error.Message, errWithCode.Error.Code)
        }
        if lastErr == nil {
            result := &model.ChannelTestResult{
                JobID:          job.ID,
                ChannelID:      task.channel.Id,
                ChannelName:    task.channel.Name,
                ModelName:      task.model,
                Success:        true,
                DurationMillis: int(math.Round(consumed * 1000)),
                RetryCount:     attempt,
            }
            return result, false
        }
        if attempt < retryLimit {
            select {
            case <-ctx.Done():
                return nil, true
            case <-time.After(time.Duration(interval) * time.Millisecond):
            }
        }
    }

    message := ""
    if openAIErrorMessage != "" {
        message = openAIErrorMessage
    } else if lastErr != nil {
        message = lastErr.Error()
    } else {
        message = "未知错误"
    }

    result := &model.ChannelTestResult{
        JobID:          job.ID,
        ChannelID:      task.channel.Id,
        ChannelName:    task.channel.Name,
        ModelName:      task.model,
        Success:        false,
        DurationMillis: int(math.Round(consumed * 1000)),
        RetryCount:     retryLimit,
        ErrorMessage:   message,
    }
    return result, false
}

func sanitizeConcurrency(value int) int {
    if value <= 0 {
        return 2
    }
    if value > 16 {
        return 16
    }
    return value
}

func sanitizeInterval(value int) int {
    if value < 100 {
        return 200
    }
    if value > 5000 {
        return 5000
    }
    return value
}

func sanitizeRetryLimit(value int) int {
    if value < 0 {
        return 0
    }
    if value > 5 {
        return 5
    }
    return value
}

func pickModelsForChannel(channel *model.Channel, options model.ChannelTestJobOptions) []string {
    models := channel.GetModels()
    unique := make([]string, 0, len(models))
    visited := make(map[string]struct{})
    for _, modelName := range models {
        m := strings.TrimSpace(modelName)
        if m == "" {
            continue
        }
        if _, ok := visited[m]; ok {
            continue
        }
        visited[m] = struct{}{}
        unique = append(unique, m)
    }

    defaultModel := ""
    if channel.TestModel != nil {
        defaultModel = strings.TrimSpace(*channel.TestModel)
    }

    var selected []string
    if options.TestMode == model.ChannelTestJobModeSelected && len(options.TargetModels) > 0 {
        targetSet := make(map[string]struct{}, len(options.TargetModels))
        for _, item := range options.TargetModels {
            trimmed := strings.TrimSpace(item)
            if trimmed == "" {
                continue
            }
            targetSet[trimmed] = struct{}{}
        }
        if defaultModel != "" {
            if _, ok := targetSet[defaultModel]; ok {
                selected = append(selected, defaultModel)
            }
        }
        for _, m := range unique {
            if _, ok := targetSet[m]; ok {
                selected = append(selected, m)
            }
        }
    } else {
        switch options.ModelScope {
        case "default":
            if defaultModel != "" {
                selected = []string{defaultModel}
            } else if len(unique) > 0 {
                selected = []string{unique[0]}
            }
        default:
            selected = append(selected, unique...)
            if options.UseChannelDefault && defaultModel != "" {
                if _, ok := visited[defaultModel]; !ok {
                    selected = append([]string{defaultModel}, selected...)
                }
            }
        }
    }

    if len(options.ModelWhitelist) > 0 {
        allow := make(map[string]struct{})
        for _, item := range options.ModelWhitelist {
            allow[strings.TrimSpace(item)] = struct{}{}
        }
        filtered := make([]string, 0)
        seen := make(map[string]struct{})
        for _, modelName := range selected {
            if _, ok := allow[modelName]; ok {
                if _, existed := seen[modelName]; existed {
                    continue
                }
                seen[modelName] = struct{}{}
                filtered = append(filtered, modelName)
            }
        }
        selected = filtered
    }

    if len(options.ModelBlacklist) > 0 {
        deny := make(map[string]struct{})
        for _, item := range options.ModelBlacklist {
            deny[strings.TrimSpace(item)] = struct{}{}
        }
        filtered := make([]string, 0)
        seen := make(map[string]struct{})
        for _, modelName := range selected {
            if _, ok := deny[modelName]; ok {
                continue
            }
            if _, existed := seen[modelName]; existed {
                continue
            }
            seen[modelName] = struct{}{}
            filtered = append(filtered, modelName)
        }
        selected = filtered
    } else {
        filtered := make([]string, 0)
        seen := make(map[string]struct{})
        for _, modelName := range selected {
            if _, existed := seen[modelName]; existed {
                continue
            }
            seen[modelName] = struct{}{}
            filtered = append(filtered, modelName)
        }
        selected = filtered
    }

    if len(selected) == 0 && defaultModel != "" && options.TestMode != model.ChannelTestJobModeSelected {
        selected = []string{defaultModel}
    }

    return selected
}
