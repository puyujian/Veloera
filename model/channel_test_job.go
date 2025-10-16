package model

import (
    "encoding/json"
    "errors"
    "strings"
    "time"

    "gorm.io/gorm"
)

// ChannelTestJobStatus 表示批量模型测试任务状态
//go:generate stringer -type=ChannelTestJobStatus
// 注：暂不启用 stringer，仅保留注释以便未来扩展

// 定义任务状态常量
type ChannelTestJobStatus string

type ChannelTestJobMode string

const (
    ChannelTestJobStatusPending   ChannelTestJobStatus = "PENDING"
    ChannelTestJobStatusRunning   ChannelTestJobStatus = "RUNNING"
    ChannelTestJobStatusSuccess   ChannelTestJobStatus = "SUCCESS"
    ChannelTestJobStatusFailed    ChannelTestJobStatus = "FAILED"
    ChannelTestJobStatusCanceled  ChannelTestJobStatus = "CANCELED"
)

const (
    ChannelTestJobModeAll      ChannelTestJobMode = "all"
    ChannelTestJobModeSelected ChannelTestJobMode = "selected"
)

const (
    ChannelTestResultStatusSuccess = "SUCCESS"
    ChannelTestResultStatusFailed  = "FAILED"
    ChannelTestResultStatusDeleted = "DELETED"
)

// ChannelTestJob 表示一次批量渠道模型测试任务
type ChannelTestJob struct {
    ID              int64                 `json:"id" gorm:"primary_key;AUTO_INCREMENT"`
    CreatedAt       int64                 `json:"created_at" gorm:"index"`
    UpdatedAt       int64                 `json:"updated_at"`
    StartedAt       int64                 `json:"started_at"`
    FinishedAt      int64                 `json:"finished_at"`
    Status          ChannelTestJobStatus  `json:"status" gorm:"type:varchar(20);index"`
    RequestedBy     int                   `json:"requested_by" gorm:"index"`
    TotalChannels   int                   `json:"total_channels"`
    TotalModels     int                   `json:"total_models"`
    CompletedCount  int                   `json:"completed_count"`
    SuccessCount    int                   `json:"success_count"`
    FailureCount    int                   `json:"failure_count"`
    CancelCount     int                   `json:"cancel_count"`
    Concurrency     int                   `json:"concurrency"`
    IntervalMs      int                   `json:"interval_ms"`
    RetryLimit      int                   `json:"retry_limit"`
    OptionsJSON     string                `json:"options" gorm:"type:text"`
    ErrorMessage    string                `json:"error_message" gorm:"type:text"`
    CurrentChannel  int                   `json:"current_channel"`
    CurrentModel    string                `json:"current_model" gorm:"type:varchar(255)"`
}

// ChannelTestJobOptions 记录任务配置，保存在 OptionsJSON 中
type ChannelTestJobOptions struct {
    ChannelIDs        []int               `json:"channel_ids"`
    IncludeAll        bool                `json:"include_all"`
    IncludeDisabled   bool                `json:"include_disabled"`
    ModelScope        string              `json:"model_scope"`
    ModelWhitelist    []string            `json:"model_whitelist"`
    ModelBlacklist    []string            `json:"model_blacklist"`
    UseChannelDefault bool                `json:"use_channel_default"`
    TestMode          ChannelTestJobMode  `json:"test_mode"`
    TargetModels      []string            `json:"target_models"`
}

// DefaultChannelTestJobOptions 返回默认配置
func DefaultChannelTestJobOptions() ChannelTestJobOptions {
    return ChannelTestJobOptions{
        ModelScope:        "all",
        UseChannelDefault: true,
        TestMode:          ChannelTestJobModeAll,
        TargetModels:      []string{},
    }
}

// SetOptions 将配置序列化到 OptionsJSON
func (j *ChannelTestJob) SetOptions(options ChannelTestJobOptions) error {
    if j == nil {
        return errors.New("job is nil")
    }
    data, err := json.Marshal(options)
    if err != nil {
        return err
    }
    j.OptionsJSON = string(data)
    return nil
}

// GetOptions 反序列化任务配置
func (j *ChannelTestJob) GetOptions() (ChannelTestJobOptions, error) {
    opts := DefaultChannelTestJobOptions()
    if j == nil {
        return opts, errors.New("job is nil")
    }
    if strings.TrimSpace(j.OptionsJSON) == "" {
        return opts, nil
    }
    if err := json.Unmarshal([]byte(j.OptionsJSON), &opts); err != nil {
        return opts, err
    }
    return opts, nil
}

// ChannelTestResult 表示单个模型测试结果
type ChannelTestResult struct {
    ID             int64  `json:"id" gorm:"primary_key;AUTO_INCREMENT"`
    JobID          int64  `json:"job_id" gorm:"index"`
    ChannelID      int    `json:"channel_id" gorm:"index"`
    ChannelName    string `json:"channel_name" gorm:"type:varchar(255)"`
    ModelName      string `json:"model_name" gorm:"type:varchar(255);index"`
    Success        bool   `json:"success" gorm:"index"`
    ResultStatus   string `json:"result_status" gorm:"type:varchar(20);index"`
    DurationMillis int    `json:"duration_millis"`
    RetryCount     int    `json:"retry_count"`
    ErrorMessage   string `json:"error_message" gorm:"type:text"`
    CreatedAt      int64  `json:"created_at" gorm:"index"`
}
// CreateChannelTestJob 创建批量测试任务
func CreateChannelTestJob(job *ChannelTestJob) error {
    if job == nil {
        return errors.New("job is nil")
    }
    now := time.Now().Unix()
    if job.Status == "" {
        job.Status = ChannelTestJobStatusPending
    }
    job.CreatedAt = now
    job.UpdatedAt = now
    return DB.Create(job).Error
}

// UpdateChannelTestJob 更新任务信息
func UpdateChannelTestJob(job *ChannelTestJob, fields map[string]any) error {
    if job == nil {
        return errors.New("job is nil")
    }
    if job.ID == 0 {
        return errors.New("job id is zero")
    }
    fields["updated_at"] = time.Now().Unix()
    return DB.Model(job).Where("id = ?", job.ID).Updates(fields).Error
}

// GetChannelTestJob 按 ID 查询任务
func GetChannelTestJob(id int64) (*ChannelTestJob, error) {
    if id <= 0 {
        return nil, errors.New("invalid job id")
    }
    job := &ChannelTestJob{}
    err := DB.Where("id = ?", id).First(job).Error
    if err != nil {
        return nil, err
    }
    return job, nil
}

// ListChannelTestJobs 返回最近的批量测试任务列表
func ListChannelTestJobs(limit int) ([]ChannelTestJob, error) {
    if limit <= 0 || limit > 200 {
        limit = 50
    }
    jobList := make([]ChannelTestJob, 0, limit)
    err := DB.Model(&ChannelTestJob{}).
        Order("id desc").
        Limit(limit).
        Find(&jobList).Error
    return jobList, err
}

// AddChannelTestResult 写入单条测试结果
func AddChannelTestResult(result *ChannelTestResult) error {
    if result == nil {
        return errors.New("result is nil")
    }
    if result.JobID == 0 {
        return errors.New("job id is zero")
    }
    if strings.TrimSpace(result.ResultStatus) == "" {
        if result.Success {
            result.ResultStatus = ChannelTestResultStatusSuccess
        } else {
            result.ResultStatus = ChannelTestResultStatusFailed
        }
    }
    result.CreatedAt = time.Now().Unix()
    return DB.Create(result).Error
}

// ListChannelTestResults 分页获取指定任务的测试结果
func ListChannelTestResults(jobID int64, offset, limit int) ([]ChannelTestResult, int64, error) {
    if jobID <= 0 {
        return nil, 0, errors.New("invalid job id")
    }
    if limit <= 0 || limit > 200 {
        limit = 50
    }
    results := make([]ChannelTestResult, 0, limit)
    query := DB.Model(&ChannelTestResult{}).Where("job_id = ?", jobID)
    var total int64
    if err := query.Count(&total).Error; err != nil {
        return nil, 0, err
    }
    err := query.Order("id asc").Offset(offset).Limit(limit).Find(&results).Error
    return results, total, err
}

// GetChannelTestResult 根据 ID 查询单条测试结果
func GetChannelTestResult(resultID int64) (*ChannelTestResult, error) {
    if resultID <= 0 {
        return nil, errors.New("invalid result id")
    }
    result := &ChannelTestResult{}
    if err := DB.Where("id = ?", resultID).First(result).Error; err != nil {
        return nil, err
    }
    return result, nil
}

// UpdateChannelTestResult 更新指定测试结果的字段
func UpdateChannelTestResult(resultID int64, updates map[string]any) error {
    if resultID <= 0 {
        return errors.New("invalid result id")
    }
    if len(updates) == 0 {
        return nil
    }
    return DB.Model(&ChannelTestResult{}).Where("id = ?", resultID).Updates(updates).Error
}

// MarkChannelTestResultsDeleted 批量将结果标记为已删除
func MarkChannelTestResultsDeleted(resultIDs []int64) error {
    if len(resultIDs) == 0 {
        return nil
    }
    return DB.Model(&ChannelTestResult{}).
        Where("id IN ?", resultIDs).
        Updates(map[string]any{
            "result_status": ChannelTestResultStatusDeleted,
        }).Error
}

// RefreshChannelTestJobStats 重新统计任务的成功/失败数量
func RefreshChannelTestJobStats(jobID int64) error {
    if jobID <= 0 {
        return errors.New("invalid job id")
    }
    success, failure, err := AggregateChannelTestResults(jobID)
    if err != nil {
        return err
    }
    return DB.Model(&ChannelTestJob{}).
        Where("id = ?", jobID).
        Updates(map[string]any{
            "success_count": int(success),
            "failure_count": int(failure),
            "updated_at":    time.Now().Unix(),
        }).Error
}

// IncrementChannelTestJobCounters 更新任务计数器
func IncrementChannelTestJobCounters(jobID int64, channelID int, modelName string, success bool) error {
    if jobID <= 0 {
        return errors.New("invalid job id")
    }
    updates := map[string]any{
        "completed_count": gorm.Expr("completed_count + ?", 1),
        "current_channel": channelID,
        "current_model":   modelName,
        "updated_at":      time.Now().Unix(),
    }
    if success {
        updates["success_count"] = gorm.Expr("success_count + ?", 1)
    } else {
        updates["failure_count"] = gorm.Expr("failure_count + ?", 1)
    }
    return DB.Model(&ChannelTestJob{}).Where("id = ?", jobID).Updates(updates).Error
}

// IncreaseChannelTestJobCancelCount 记录取消的测试数量
func IncreaseChannelTestJobCancelCount(jobID int64, cancelNum int) error {
    if jobID <= 0 || cancelNum <= 0 {
        return nil
    }
    return DB.Model(&ChannelTestJob{}).
        Where("id = ?", jobID).
        Updates(map[string]any{
            "cancel_count": gorm.Expr("cancel_count + ?", cancelNum),
            "updated_at":   time.Now().Unix(),
        }).Error
}

// AggregateChannelTestResults 统计测试结果
func AggregateChannelTestResults(jobID int64) (success, failure int64, err error) {
    if jobID <= 0 {
        return 0, 0, errors.New("invalid job id")
    }
    type aggResult struct {
        Success bool
        Total   int64
    }
    var agg []aggResult
    err = DB.Model(&ChannelTestResult{}).
        Select("success, COUNT(*) as total").
        Where("job_id = ? AND (result_status IS NULL OR result_status != ?)", jobID, ChannelTestResultStatusDeleted).
        Group("success").
        Find(&agg).Error
    if err != nil {
        return
    }
    for _, item := range agg {
        if item.Success {
            success += item.Total
        } else {
            failure += item.Total
        }
    }
    return
}

// FinalizeChannelTestJob 设置任务最终状态及错误信息
func FinalizeChannelTestJob(jobID int64, status ChannelTestJobStatus, errMsg string) error {
    if jobID <= 0 {
        return errors.New("invalid job id")
    }
    updates := map[string]any{
        "status":      status,
        "finished_at": time.Now().Unix(),
        "updated_at":  time.Now().Unix(),
    }
    if strings.TrimSpace(errMsg) != "" {
        updates["error_message"] = errMsg
    }
    return DB.Model(&ChannelTestJob{}).Where("id = ?", jobID).Updates(updates).Error
}
