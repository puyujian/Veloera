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
import React, { useEffect, useState } from 'react';
import { Button, Card, Divider, Form, Radio, Space, Spin, Typography } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';

const SettingsScheduledAutoUpdate = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [channels, setChannels] = useState([]);
  const [settings, setSettings] = useState({
    enabled: false,
    frequency: 60,
    mode: 'incremental',
    enable_auto_rename: false,
    include_vendor: false,
    channel_ids: [],
  });

  const loadChannels = async () => {
    try {
      const res = await API.get('/api/channel/?p=0&page_size=1000');
      const { success, data } = res.data;
      if (success && data) {
        setChannels(data);
      }
    } catch (error) {
      console.error('加载渠道列表失败:', error);
    }
  };

  const loadSettings = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/channel/scheduled-auto-update/settings');
      const { success, data, message } = res.data;
      if (success && data) {
        setSettings({
          enabled: data.enabled || false,
          frequency: data.frequency || 60,
          mode: data.mode || 'incremental',
          enable_auto_rename: data.enable_auto_rename || false,
          include_vendor: data.include_vendor || false,
          channel_ids: data.channel_ids || [],
        });
      } else {
        showError(message || t('加载配置失败'));
      }
    } catch (error) {
      showError(t('加载配置失败') + ': ' + (error.message || ''));
    } finally {
      setLoading(false);
    }
  };

  const saveSettings = async () => {
    setSaving(true);
    try {
      const res = await API.put('/api/channel/scheduled-auto-update/settings', settings);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('保存成功'));
        await loadSettings();
      } else {
        showError(message || t('保存失败'));
      }
    } catch (error) {
      showError(t('保存失败') + ': ' + (error.message || ''));
    } finally {
      setSaving(false);
    }
  };

  useEffect(() => {
    loadChannels();
    loadSettings();
  }, []);

  const handleSettingChange = (key, value) => {
    setSettings((prev) => ({
      ...prev,
      [key]: value,
    }));
  };

  const channelOptions = channels.map((channel) => ({
    value: channel.id,
    label: `[${channel.id}] ${channel.name}`,
  }));

  return (
    <Spin spinning={loading}>
      <Typography.Title heading={5}>{t('定时自动更新模型')}</Typography.Title>
      <Typography.Paragraph>
        {t(
          '定时从上游渠道自动拉取并更新模型列表。配置后将按照设定的频率自动执行，无需手动操作。'
        )}
      </Typography.Paragraph>

      <Form labelPosition='left' labelAlign='left' labelWidth='180px'>
        <Form.Switch
          field='enabled'
          label={t('启用定时自动更新')}
          checked={settings.enabled}
          onChange={(value) => handleSettingChange('enabled', value)}
        />

        {settings.enabled && (
          <>
            <Form.InputNumber
              field='frequency'
              label={t('更新频率（分钟）')}
              labelExtra={t('最小值：5分钟')}
              value={settings.frequency}
              min={5}
              max={10080}
              step={5}
              style={{ width: '200px' }}
              suffix={t('分钟')}
              onChange={(value) =>
                handleSettingChange('frequency', typeof value === 'number' ? value : 5)
              }
            />

            <Form.RadioGroup
              field='mode'
              label={t('更新模式')}
              value={settings.mode}
              onChange={(value) => handleSettingChange('mode', value)}
            >
              <Radio value='incremental'>
                {t('增量模式')}
                <Typography.Text
                  type='tertiary'
                  size='small'
                  style={{ marginLeft: '8px' }}
                >
                  {t('保留现有模型，只添加新增的')}
                </Typography.Text>
              </Radio>
              <Radio value='full' style={{ marginTop: '8px' }}>
                {t('完全同步模式')}
                <Typography.Text
                  type='tertiary'
                  size='small'
                  style={{ marginLeft: '8px' }}
                >
                  {t('完全以上游为准，替换现有模型列表')}
                </Typography.Text>
              </Radio>
            </Form.RadioGroup>

            <Form.Switch
              field='enable_auto_rename'
              label={t('启用自动重命名')}
              labelExtra={t('为新增模型自动应用重命名规则')}
              checked={settings.enable_auto_rename}
              onChange={(value) => handleSettingChange('enable_auto_rename', value)}
            />

            {settings.enable_auto_rename && (
              <Form.Switch
                field='include_vendor'
                label={t('包含厂商前缀')}
                labelExtra={t('重命名时是否包含厂商名称前缀')}
                checked={settings.include_vendor}
                onChange={(value) => handleSettingChange('include_vendor', value)}
              />
            )}

            <Form.Select
              field='channel_ids'
              label={t('选择渠道')}
              labelExtra={t('留空表示更新所有已启用的渠道')}
              placeholder={t('留空表示更新所有已启用的渠道')}
              value={settings.channel_ids}
              onChange={(value) =>
                handleSettingChange(
                  'channel_ids',
                  Array.isArray(value) ? value.map((item) => Number(item)) : []
                )
              }
              multiple
              filter
              style={{ width: '100%' }}
              optionList={channelOptions}
              maxTagCount={5}
            />

            {settings.channel_ids && settings.channel_ids.length > 0 && (
              <div style={{ marginLeft: '180px', marginTop: '-10px' }}>
                <Typography.Text type='tertiary' size='small'>
                  {t('已选择 {{count}} 个渠道', {
                    count: settings.channel_ids.length,
                  })}
                </Typography.Text>
              </div>
            )}
          </>
        )}

        <Divider />

        <div style={{ marginLeft: '180px' }}>
          <Space>
            <Button
              type='primary'
              onClick={saveSettings}
              loading={saving}
              disabled={loading}
            >
              {t('保存配置')}
            </Button>
            <Button onClick={loadSettings} disabled={loading || saving}>
              {t('重置')}
            </Button>
          </Space>
        </div>
      </Form>

      {settings.enabled && (
        <Card
          style={{ marginTop: '20px', backgroundColor: 'var(--semi-color-info-light-default)' }}
        >
          <Typography.Text strong>{t('提示')}</Typography.Text>
          <ul style={{ marginTop: '8px', marginBottom: '0' }}>
            <li>
              <Typography.Text>
                {t('定时任务将在后台自动运行，每隔 {{frequency}} 分钟执行一次', {
                  frequency: settings.frequency,
                })}
              </Typography.Text>
            </li>
            <li>
              <Typography.Text>
                {t(
                  '只会更新状态为"已启用"的渠道，已禁用的渠道不会被更新'
                )}
              </Typography.Text>
            </li>
            <li>
              <Typography.Text>
                {t('更新日志可在系统日志中查看')}
              </Typography.Text>
            </li>
            <li>
              <Typography.Text>
                {t('如需立即执行一次更新，可前往渠道管理页面使用手动更新功能')}
              </Typography.Text>
            </li>
          </ul>
        </Card>
      )}
    </Spin>
  );
};

export default SettingsScheduledAutoUpdate;
