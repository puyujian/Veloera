/*
Copyright (c) 2025 Tethys Plex

This file is part of Veloera.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import React, { useState, useEffect } from 'react';
import {
  Modal,
  Button,
  RadioGroup,
  Radio,
  Select,
  TextArea,
  Checkbox,
  Collapse,
  Typography,
  Space,
  Spin,
  Banner,
  Table,
  Tag,
  Divider,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess, showWarning } from '../helpers';

const { Text, Title } = Typography;

const AutoRenameModal = ({ visible, onClose, selectedChannelIds, onSuccess }) => {
  const [step, setStep] = useState(1);
  const [loading, setLoading] = useState(false);
  const [config, setConfig] = useState({
    mode: 'system',
    aiModel: '',
    prompt: `输入模型列表：
【删除中文中括号本身和里面的全部内容】
重定向规则：

按照"厂商/标准模型名"格式重定向名称，左边为重定向后的名称，右边为原来名称。
厂商列表包括但不限于：
Anthropic
OpenAI
Gemini
Moonshot
智谱（包括 GLM/BigModel 前缀的全部归为"智谱"）
通义千问
DeepSeek
腾讯混元
MistralAI
xAI
Llama
penAl
识别模型名的厂商归属，去掉模型名里的日期或多余后缀，只保留主模型名，如 claude-3-5-haiku，不要 20241022。
按照"厂商/模型名"格式输出，如：Anthropic/claude-3.5-haiku、Gemini/Gemini-xxx、智谱/GLM-4.5、MistralAI/MistralAI-xxx 等。
输出格式：

以对象形式输出，每个条目为 "重定向名称": "原来名称"`,
    enabledOnly: true,
  });
  const [previewData, setPreviewData] = useState(null);
  const [applyMode, setApplyMode] = useState('append');
  const [availableModels, setAvailableModels] = useState([]);

  // 加载可用的AI模型列表
  useEffect(() => {
    if (visible && config.mode === 'ai') {
      loadAvailableModels();
    }
  }, [visible, config.mode]);

  const loadAvailableModels = async () => {
    try {
      const res = await API.get('/api/user/models');
      const { success, message, data } = res.data;
      if (success) {
        setAvailableModels(data || []);
      } else {
        showError(message || '加载模型列表失败');
        console.error('加载模型列表失败:', message);
      }
    } catch (error) {
      showError('加载模型列表失败: ' + (error.response?.data?.message || error.message));
      console.error('加载模型列表失败:', error);
    }
  };

  const handleGenerate = async () => {
    if (config.mode === 'ai') {
      if (!config.aiModel) {
        showWarning('请选择AI模型');
        return;
      }
      if (!config.prompt) {
        showWarning('请输入提示词');
        return;
      }
    }

    setLoading(true);
    try {
      const payload = {
        mode: config.mode,
        enabled_only: config.enabledOnly,
      };

      if (selectedChannelIds && selectedChannelIds.length > 0) {
        payload.channel_ids = selectedChannelIds;
      }

      if (config.mode === 'ai') {
        payload.ai_model = config.aiModel;
        payload.prompt = config.prompt;
      }

      const res = await API.post('/api/channel/auto-rename/generate', payload);

      if (res.data.success) {
        setPreviewData(res.data.data);
        setStep(2);
      } else {
        showError(res.data.message || '生成预览失败');
      }
    } catch (error) {
      showError('生成预览失败: ' + (error.response?.data?.message || error.message));
    } finally {
      setLoading(false);
    }
  };

  const handleApply = async () => {
    if (!previewData) return;

    setLoading(true);
    try {
      const payload = {
        session_id: previewData.session_id,
        mode: applyMode,
        channel_ids: previewData.channels.map(ch => ch.channel_id),
        mapping: previewData.global_mapping,
      };

      const res = await API.post('/api/channel/auto-rename/apply', payload);

      if (res.data.success) {
        const result = res.data.data;
        showSuccess(`应用成功！成功 ${result.success} 个，失败 ${result.failed} 个`);
        setStep(3);
        if (onSuccess) onSuccess();
      } else {
        showError(res.data.message || '应用失败');
      }
    } catch (error) {
      showError('应用失败: ' + (error.response?.data?.message || error.message));
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    setStep(1);
    setPreviewData(null);
    setApplyMode('append');
    onClose();
  };

  const renderStep1 = () => (
    <div style={{ padding: '20px 0' }}>
      <Title heading={6}>处理模式</Title>
      <RadioGroup
        value={config.mode}
        onChange={(e) => setConfig({ ...config, mode: e.target.value })}
        type="button"
      >
        <Radio value="system">系统处理</Radio>
        <Radio value="ai">AI处理</Radio>
      </RadioGroup>

      {config.mode === 'ai' && (
        <div style={{ marginTop: 20 }}>
          <Title heading={6}>选择AI模型</Title>
          <Select
            value={config.aiModel}
            onChange={(value) => setConfig({ ...config, aiModel: value })}
            placeholder="请选择AI模型"
            filter
            style={{ width: '100%' }}
          >
            {availableModels.map(model => (
              <Select.Option key={model} value={model}>
                {model}
              </Select.Option>
            ))}
          </Select>

          <Title heading={6} style={{ marginTop: 20 }}>AI提示词</Title>
          <TextArea
            value={config.prompt}
            onChange={(value) => setConfig({ ...config, prompt: value })}
            placeholder="输入提示词..."
            rows={10}
            style={{ fontFamily: 'monospace' }}
          />
        </div>
      )}

      <div style={{ marginTop: 20 }}>
        <Title heading={6}>处理范围</Title>
        <Checkbox
          checked={config.enabledOnly}
          onChange={(e) => setConfig({ ...config, enabledOnly: e.target.checked })}
        >
          仅处理启用的渠道
        </Checkbox>
        {selectedChannelIds && selectedChannelIds.length > 0 && (
          <div style={{ marginTop: 10 }}>
            <Tag>已选中 {selectedChannelIds.length} 个渠道</Tag>
          </div>
        )}
      </div>
    </div>
  );

  const renderStep2 = () => {
    if (!previewData) return null;

    const { statistics, global_mapping, channels } = previewData;

    const channelColumns = [
      {
        title: '渠道',
        dataIndex: 'channel_name',
        render: (text, record) => (
          <div>
            <div>{text}</div>
            <Text type="tertiary" size="small">{record.group}</Text>
          </div>
        ),
      },
      {
        title: '影响模型',
        dataIndex: 'affected_models',
        render: (models) => models.length,
      },
      {
        title: '未变化',
        dataIndex: 'unchanged_models',
        render: (models) => models.length,
      },
    ];

    return (
      <div style={{ padding: '20px 0' }}>
        <Banner
          type="info"
          description={
            <div>
              影响渠道：{statistics.total_channels} 个 |
              模型总数：{statistics.total_models} 个（去重后 {statistics.unique_models} 个）|
              将重命名：{statistics.renamed_models} 个
            </div>
          }
        />

        <Title heading={6} style={{ marginTop: 20 }}>全局映射预览</Title>
        <div style={{
          maxHeight: 200,
          overflow: 'auto',
          background: '#f7f7f7',
          padding: 10,
          borderRadius: 4,
          fontFamily: 'monospace',
          fontSize: 12,
        }}>
          {Object.entries(global_mapping).map(([newName, oldName]) => (
            <div key={oldName}>
              <Text type="success">{newName}</Text> ← <Text type="tertiary">{oldName}</Text>
            </div>
          ))}
        </div>

        <Title heading={6} style={{ marginTop: 20 }}>各渠道影响详情</Title>
        <Table
          dataSource={channels}
          columns={channelColumns}
          pagination={{ pageSize: 10 }}
          size="small"
        />

        <Divider />

        <Title heading={6}>选择应用模式</Title>
        <RadioGroup
          value={applyMode}
          onChange={(e) => setApplyMode(e.target.value)}
          type="button"
        >
          <Radio value="append">追加模式 - 保留原有映射，新增/覆盖重命名项</Radio>
          <Radio value="replace">替换模式 - 清空原有映射，仅保留新生成的映射</Radio>
        </RadioGroup>

        <Banner
          type="warning"
          style={{ marginTop: 20 }}
          description={
            <div>
              ⚠️ 此操作将修改 {statistics.total_channels} 个渠道的模型重定向配置
              <br />
              ✅ 系统已自动保存快照，可随时撤销
            </div>
          }
        />
      </div>
    );
  };

  const renderStep3 = () => (
    <div style={{ padding: '20px 0', textAlign: 'center' }}>
      <Banner
        type="success"
        description="重命名已成功应用！"
      />
      <div style={{ marginTop: 20 }}>
        <Text>如需撤销，请前往"历史快照"页面</Text>
      </div>
    </div>
  );

  const getFooter = () => {
    if (step === 1) {
      return (
        <div>
          <Button onClick={handleClose}>取消</Button>
          <Button
            type="primary"
            onClick={handleGenerate}
            loading={loading}
            style={{ marginLeft: 8 }}
          >
            生成预览
          </Button>
        </div>
      );
    } else if (step === 2) {
      return (
        <div>
          <Button onClick={() => setStep(1)}>返回修改</Button>
          <Button
            type="primary"
            onClick={handleApply}
            loading={loading}
            style={{ marginLeft: 8 }}
          >
            应用到渠道
          </Button>
        </div>
      );
    } else {
      return (
        <div>
          <Button type="primary" onClick={handleClose}>
            关闭
          </Button>
        </div>
      );
    }
  };

  return (
    <Modal
      title="🔄 批量自动重命名"
      visible={visible}
      onCancel={handleClose}
      footer={getFooter()}
      width={800}
      bodyStyle={{ maxHeight: 600, overflow: 'auto' }}
    >
      <Spin spinning={loading}>
        {step === 1 && renderStep1()}
        {step === 2 && renderStep2()}
        {step === 3 && renderStep3()}
      </Spin>
    </Modal>
  );
};

export default AutoRenameModal;
