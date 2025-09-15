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
import React, { useEffect, useState, useRef } from 'react';
import {
  Button,
  Form,
  Row,
  Col,
  Typography,
  Card,
} from '@douyinfe/semi-ui';
const { Text } = Typography;
import {
  removeTrailingSlash,
  showError,
  showSuccess,
  verifyJSON,
} from '../helpers/utils';
import { API } from '../helpers/api';

const PaymentSetting = () => {
  let [inputs, setInputs] = useState({
    ServerAddress: '',
    PayAddress: '',
    EpayId: '',
    EpayKey: '',
    Price: 7.3,
    MinTopUp: 1,
    TopupGroupRatio: '',
    CustomCallbackAddress: '',
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
          if (item.key === 'TopupGroupRatio') {
            item.value = JSON.stringify(JSON.parse(item.value), null, 2);
          } else if (item.key === 'Price' || item.key === 'MinTopUp') {
            item.value = parseFloat(item.value);
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

  const submitPayAddress = async () => {
    if (inputs.ServerAddress === '') {
      showError('请先填写服务器地址');
      return;
    }
    if (originInputs['TopupGroupRatio'] !== inputs.TopupGroupRatio) {
      if (!verifyJSON(inputs.TopupGroupRatio)) {
        showError('充值分组倍率不是合法的 JSON 字符串');
        return;
      }
    }

    const options = [
      { key: 'PayAddress', value: removeTrailingSlash(inputs.PayAddress) },
    ];

    if (inputs.EpayId !== '') {
      options.push({ key: 'EpayId', value: inputs.EpayId });
    }
    if (inputs.EpayKey !== undefined && inputs.EpayKey !== '') {
      options.push({ key: 'EpayKey', value: inputs.EpayKey });
    }
    if (inputs.Price !== '') {
      options.push({ key: 'Price', value: inputs.Price.toString() });
    }
    if (inputs.MinTopUp !== '') {
      options.push({ key: 'MinTopUp', value: inputs.MinTopUp.toString() });
    }
    if (inputs.CustomCallbackAddress !== '') {
      options.push({
        key: 'CustomCallbackAddress',
        value: inputs.CustomCallbackAddress,
      });
    }
    if (originInputs['TopupGroupRatio'] !== inputs.TopupGroupRatio) {
      options.push({ key: 'TopupGroupRatio', value: inputs.TopupGroupRatio });
    }

    await updateOptions(options);
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
        <Card style={{ marginTop: '10px' }}>
          <Form.Section text='支付设置'>
            <Text>
              （当前仅支持易支付接口，默认使用上方服务器地址作为回调地址！）
            </Text>
            <Row
              gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            >
              <Col xs={24} sm={24} md={8} lg={8} xl={8}>
                <Form.Input
                  field='PayAddress'
                  label='支付地址'
                  placeholder='例如：https://yourdomain.com'
                />
              </Col>
              <Col xs={24} sm={24} md={8} lg={8} xl={8}>
                <Form.Input
                  field='EpayId'
                  label='易支付商户ID'
                  placeholder='例如：0001'
                />
              </Col>
              <Col xs={24} sm={24} md={8} lg={8} xl={8}>
                <Form.Input
                  field='EpayKey'
                  label='易支付商户密钥'
                  placeholder='敏感信息不会发送到前端显示'
                  type='password'
                />
              </Col>
            </Row>
            <Row
              gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
              style={{ marginTop: 16 }}
            >
              <Col xs={24} sm={24} md={8} lg={8} xl={8}>
                <Form.Input
                  field='CustomCallbackAddress'
                  label='回调地址'
                  placeholder='例如：https://yourdomain.com'
                />
              </Col>
              <Col xs={24} sm={24} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field='Price'
                  precision={2}
                  label='充值价格（x元/美金）'
                  placeholder='例如：7，就是7元/美金'
                />
              </Col>
              <Col xs={24} sm={24} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field='MinTopUp'
                  label='最低充值美元数量'
                  placeholder='例如：2，就是最低充值2$'
                />
              </Col>
            </Row>
            <Form.TextArea
              field='TopupGroupRatio'
              label='充值分组倍率'
              placeholder='为一个 JSON 文本，键为组名称，值为倍率'
              autosize
            />
            <Button onClick={submitPayAddress}>更新支付设置</Button>
          </Form.Section>
        </Card>
      )}
    </Form>
  );
};

export default PaymentSetting;