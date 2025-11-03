# 定时自动更新模型功能

## 功能说明

将原有的手动更新模型功能改造为定时自动更新功能。系统会按照配置的频率，自动从上游渠道拉取最新的模型列表并更新到本地。

## 主要改动

### 后端改动

1. **model/option.go**
   - 新增6个配置选项:
     - `ScheduledAutoUpdateEnabled`: 是否启用定时更新
     - `ScheduledAutoUpdateFrequency`: 更新频率（分钟）
     - `ScheduledAutoUpdateMode`: 更新模式（incremental/full）
     - `ScheduledAutoUpdateEnableRename`: 是否启用自动重命名
     - `ScheduledAutoUpdateIncludeVendor`: 是否包含厂商前缀
     - `ScheduledAutoUpdateChannelIds`: 要更新的渠道ID列表（逗号分隔）

2. **controller/channel_auto_update.go**
   - 新增 `ScheduledAutoUpdateChannelModels(defaultFrequency int)` 定时任务函数
   - 新增 `GetScheduledAutoUpdateSettings(c *gin.Context)` API处理函数
   - 新增 `UpdateScheduledAutoUpdateSettings(c *gin.Context)` API处理函数
   - 新增辅助函数 `parseScheduledAutoUpdateChannelIDs` 和 `formatScheduledAutoUpdateChannelIDs`

3. **router/api-router.go**
   - 新增路由:
     - `GET /api/channel/scheduled-auto-update/settings` - 获取配置
     - `PUT /api/channel/scheduled-auto-update/settings` - 更新配置

4. **main.go**
   - 启动定时任务: `go controller.ScheduledAutoUpdateChannelModels(60)`

### 前端改动

1. **web/src/pages/Setting/Operation/SettingsScheduledAutoUpdate.js** (新建)
   - 定时自动更新配置界面
   - 功能包括:
     - 启用/禁用定时更新
     - 设置更新频率（最小5分钟）
     - 选择更新模式（增量/完全同步）
     - 配置自动重命名选项
     - 选择要更新的渠道（支持多选，留空则更新所有启用的渠道）

2. **web/src/components/OperationSetting.js**
   - 在运营设置页面中添加"定时自动更新设置"卡片

## 使用说明

### 配置入口

1. 登录管理员账户
2. 进入"设置" -> "运营设置"
3. 在"定时自动更新模型"卡片中进行配置

### 配置说明

- **启用定时自动更新**: 开关，控制是否启用定时任务
- **更新频率**: 设置定时任务的执行间隔（分钟），最小值为5分钟
- **更新模式**:
  - 增量模式：保留现有模型，只添加新增的模型
  - 完全同步模式：完全以上游为准，替换现有模型列表
- **启用自动重命名**: 为新增模型自动应用重命名规则
- **包含厂商前缀**: 重命名时是否包含厂商名称前缀
- **选择渠道**: 可以选择特定渠道进行更新，留空则更新所有已启用的渠道

### 注意事项

1. 定时任务只会更新状态为"已启用"的渠道
2. 定时任务在后台自动运行，无需手动触发
3. 更新日志可在系统日志中查看
4. 如需立即执行一次更新，可前往渠道管理页面使用手动更新功能
5. 修改配置后会立即生效，下次定时任务执行时会使用新配置

## API说明

### 获取配置

```http
GET /api/channel/scheduled-auto-update/settings
```

响应:
```json
{
  "success": true,
  "data": {
    "enabled": false,
    "frequency": 60,
    "mode": "incremental",
    "enable_auto_rename": false,
    "include_vendor": false,
    "channel_ids": []
  }
}
```

### 更新配置

```http
PUT /api/channel/scheduled-auto-update/settings
Content-Type: application/json

{
  "enabled": true,
  "frequency": 120,
  "mode": "incremental",
  "enable_auto_rename": true,
  "include_vendor": true,
  "channel_ids": [1, 2, 3]
}
```

响应:
```json
{
  "success": true,
  "message": "更新成功",
  "data": {
    "enabled": true,
    "frequency": 120,
    "mode": "incremental",
    "enable_auto_rename": true,
    "include_vendor": true,
    "channel_ids": [1, 2, 3]
  }
}
```

## 日志示例

系统日志中会记录定时任务的执行情况：

```
[定时更新模型] 开始执行
[定时更新模型] 渠道 1(OpenAI Official) 成功，新增 3 个模型
[定时更新模型] 渠道 2(Gemini) 成功，新增 0 个模型
[定时更新模型] 渠道 3(Test Channel) 失败: 获取上游模型失败: 请求上游失败: status code: 401
[定时更新模型] 完成，成功: 2, 失败: 1
```
