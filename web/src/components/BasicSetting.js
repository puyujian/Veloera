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
import React, { useEffect, useState, useRef, useContext } from 'react';
import {
  Button,
  Form,
  Row,
  Col,
  Typography,
  Card,
  Select,
  Space,
} from '@douyinfe/semi-ui';
const { Text } = Typography;
import {
  removeTrailingSlash,
  showError,
  showSuccess,
  timestamp2string,
} from '../helpers/utils';
import { API } from '../helpers/api';
import { useTranslation } from 'react-i18next';
import { StatusContext } from '../context/Status/index.js';

const BasicSetting = () => {
  const { t } = useTranslation();
  const [statusState] = useContext(StatusContext);
  let [inputs, setInputs] = useState({
    ServerAddress: '',
    WorkerUrl: '',
    WorkerValidKey: '',
    ReverseProxyEnabled: false,
    ReverseProxyProvider: '',
  });

  const [originInputs, setOriginInputs] = useState({});
  const [loading, setLoading] = useState(false);
  const [isLoaded, setIsLoaded] = useState(false);
  const formApiRef = useRef(null);

  const getOptions = async () => {
    setLoading(true);
    const res = await API.get('/api/option/');
    const { success, message, data } = res.data;
    if (success) {
      let newInputs = {};
      data.forEach((item) => {
        if (item.key in inputs) {
          if (item.key === 'ReverseProxyEnabled') {
            item.value = item.value === 'true';
          }
          newInputs[item.key] = item.value;
        }
      });
      setInputs(newInputs);
      setOriginInputs(newInputs);
      if (formApiRef.current) {
        formApiRef.current.setValues(newInputs);
      }
      setIsLoaded(true);
    } else {
      showError(message);
    }
    setLoading(false);
  };

  useEffect(() => {
    getOptions();
  }, []);

  const updateOptions = async (options) => {
    setLoading(true);
    try {
      const requestQueue = options.map((opt) =>
        API.put('/api/option/', {
          key: opt.key,
          value: typeof opt.value === 'boolean' ? opt.value.toString() : opt.value,
        }),
      );

      const results = await Promise.all(requestQueue);
      const errorResults = results.filter((res) => !res.data.success);
      
      if (errorResults.length > 0) {
        errorResults.forEach((res) => {
          showError(res.data.message);
        });
      } else {
        showSuccess('更新成功');
        const newInputs = { ...inputs };
        options.forEach((opt) => {
          newInputs[opt.key] = opt.value;
        });
        setInputs(newInputs);
      }
    } catch (error) {
      showError('更新失败');
    }
    setLoading(false);
  };

  const handleFormChange = (values) => {
    setInputs(values);
  };

  const submitServerAddress = async () => {
    let ServerAddress = removeTrailingSlash(inputs.ServerAddress);
    await updateOptions([{ key: 'ServerAddress', value: ServerAddress }]);
  };

  const submitWorker = async () => {
    let WorkerUrl = removeTrailingSlash(inputs.WorkerUrl);
    await updateOptions([
      { key: 'WorkerUrl', value: WorkerUrl },
      { key: 'WorkerValidKey', value: inputs.WorkerValidKey },
    ]);
  };

  const submitReverseProxy = async () => {
    const options = [];

    if (originInputs['ReverseProxyEnabled'] !== inputs.ReverseProxyEnabled) {
      options.push({ key: 'ReverseProxyEnabled', value: inputs.ReverseProxyEnabled });
    }
    if (originInputs['ReverseProxyProvider'] !== inputs.ReverseProxyProvider) {
      options.push({ key: 'ReverseProxyProvider', value: inputs.ReverseProxyProvider });
    }

    if (options.length > 0) {
      await updateOptions(options);
    }
  };

  const handleCheckboxChange = async (optionKey, event) => {
    const value = event.target.checked;
    await updateOptions([{ key: optionKey, value }]);
  };

  const getStartTimeString = () => {
    const timestamp = statusState?.status?.start_time;
    return statusState.status ? timestamp2string(timestamp) : '';
  };

  if (!isLoaded) {
    return null;
  }

  return (
    <Form
      initValues={inputs}
      onValueChange={handleFormChange}
      getFormApi={(api) => (formApiRef.current = api)}
    >
      {({ formState, values, formApi }) => (
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            gap: '10px',
            marginTop: '10px',
          }}
        >
          <Card>
            <Form.Section text={t('系统信息')}>
              <Row>
                <Col span={16}>
                  <Space>
                    <Text>
                      {t('当前版本')}：
                      {statusState?.status?.version || t('未知')}
                    </Text>
                  </Space>
                </Col>
              </Row>
              <Row>
                <Col span={16}>
                  <Text>
                    {t('启动时间')}：{getStartTimeString()}
                  </Text>
                </Col>
              </Row>
            </Form.Section>
          </Card>

          <Card>
            <Form.Section text='服务器配置'>
              <Form.Input
                field='ServerAddress'
                label='服务器地址'
                placeholder='例如：https://yourdomain.com'
                style={{ width: '100%' }}
              />
              <Button onClick={submitServerAddress}>更新服务器地址</Button>
            </Form.Section>
          </Card>
          
          <Card>
            <Form.Section text='反向代理设置'>
              <Text>用以支持系统在反向代理后运行时正确识别客户端IP地址</Text>
              <Form.Checkbox
                field='ReverseProxyEnabled'
                noLabel
                onChange={(e) =>
                  handleCheckboxChange('ReverseProxyEnabled', e)
                }
              >
                系统在反向代理后运行
              </Form.Checkbox>
              {inputs.ReverseProxyEnabled && (
                <Row
                  gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
                  style={{ marginTop: 16 }}
                >
                  <Col xs={24} sm={24} md={12} lg={12} xl={12}>
                    <Form.Select
                      field='ReverseProxyProvider'
                      label='反向代理提供商'
                      placeholder='请选择反向代理提供商'
                      style={{ width: '100%' }}
                    >
                      <Select.Option value='nginx'>Nginx / OpenResty (通用)</Select.Option>
                      <Select.Option value='cloudflare'>Cloudflare</Select.Option>
                    </Form.Select>
                  </Col>
                </Row>
              )}
              <Button onClick={submitReverseProxy} style={{ marginTop: 16 }}>
                保存反向代理设置
              </Button>
            </Form.Section>
          </Card>

          <Card>
            <Form.Section text='代理设置'>
              <Text>
                （支持{' '}
                <a
                  href='https://github.com/Calcium-Ion/new-api-worker'
                  target='_blank'
                  rel='noreferrer'
                >
                  new-api-worker
                </a>
                ）
              </Text>
              <Row
                gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
              >
                <Col xs={24} sm={24} md={12} lg={12} xl={12}>
                  <Form.Input
                    field='WorkerUrl'
                    label='Worker地址'
                    placeholder='例如：https://workername.yourdomain.workers.dev'
                  />
                </Col>
                <Col xs={24} sm={24} md={12} lg={12} xl={12}>
                  <Form.Input
                    field='WorkerValidKey'
                    label='Worker密钥'
                    placeholder='敏感信息不会发送到前端显示'
                    type='password'
                  />
                </Col>
              </Row>
              <Button onClick={submitWorker}>更新Worker设置</Button>
            </Form.Section>
          </Card>
        </div>
      )}
    </Form>
  );
};

export default BasicSetting;