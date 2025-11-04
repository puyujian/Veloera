# 修复定时自动更新配置保存错误

## 问题描述

在前端"定时自动更新模型"配置页面保存时出现错误：
```
参数错误: json: cannot unmarshal object into Go struct field scheduledSettingsRequest.mode of type string
```

## 原因分析

Semi UI 的 `Form.RadioGroup` 组件的 `onChange` 事件可能会返回事件对象而不是纯字符串值，导致前端向后端发送的 `mode` 字段是一个对象而非字符串。

## 修复方案

### 修改文件
`web/src/pages/Setting/Operation/SettingsScheduledAutoUpdate.js`

### 修改内容

1. **修复 RadioGroup 的 onChange 处理**
   ```javascript
   // 修改前
   onChange={(value) => handleSettingChange('mode', value)}
   
   // 修改后
   onChange={(value) => {
     const nextValue =
       typeof value === 'string' ? value : value?.target?.value || 'incremental';
     handleSettingChange('mode', nextValue);
   }}
   ```

2. **加强数据类型验证**
   - 在 `loadSettings` 函数中添加了严格的数据类型检查和转换
   - 在 `saveSettings` 函数中构建 payload 时添加了数据清洗逻辑
   - 确保所有发送到后端的数据类型都符合预期

3. **改进错误处理**
   - 移除了调试用的 console.log
   - 改进错误信息展示

## 测试建议

1. 保存配置时选择不同的更新模式（增量/完全同步）
2. 测试启用/禁用定时更新开关
3. 测试频率、渠道选择等其他配置项
4. 验证配置保存后重新加载是否正确显示

## 修复后的预期行为

- 配置可以正常保存，不再出现 JSON 反序列化错误
- 所有配置项都能正确保存和加载
- 更新模式切换正常工作
