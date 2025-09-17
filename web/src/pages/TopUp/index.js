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
import {
  API,
  isMobile,
  showError,
  showInfo,
  showSuccess,
} from '../../helpers';
import {
  renderNumber,
  renderQuota,
  renderQuotaWithAmount,
} from '../../helpers/render';
import {
  Layout,
  Card,
  Button,
  Form,
  Divider,
  Space,
  Modal,
  Toast,
  Banner,
  RadioGroup,
  Radio,
  Row,
  Col,
} from '@douyinfe/semi-ui';
import Title from '@douyinfe/semi-ui/lib/es/typography/title';
import { useTranslation } from 'react-i18next';
import { useTurnstile } from '../../hooks/useTurnstile';
import TurnstileWrapper from '../../components/shared/TurnstileWrapper';

const TopUp = () => {
  const { t } = useTranslation();

  // --- 原有状态 ---
  const [redemptionCode, setRedemptionCode] = useState('');
  const [topUpCode, setTopUpCode] = useState('');
  const [topUpCount, setTopUpCount] = useState(0);
  const [minTopUpCount, setMinTopUpCount] = useState(1);
  const [amount, setAmount] = useState(0.0);
  const [minTopUp, setMinTopUp] = useState(1);
  const [topUpLink, setTopUpLink] = useState('');
  const [enableOnlineTopUp, setEnableOnlineTopUp] = useState(false);
  const [userQuota, setUserQuota] = useState(0);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [open, setOpen] = useState(false);
  const [payWay, setPayWay] = useState('zfb');
  const [showRedeemModal, setShowRedeemModal] = useState(false);
  const [showTopUpModal, setShowTopUpModal] = useState(false);
  const [redeemError, setRedeemError] = useState('');
  const [topUpError, setTopUpError] = useState('');
  const [showPaymentConfirm, setShowPaymentConfirm] = useState(false);
  const [unitPrice, setUnitPrice] = useState(0);

  // --- 新增：标记 code 是否来源于 URL 且尚未重置 ---
  const [useUrlCode, setUseUrlCode] = useState(false);

  // Turnstile hook
  const {
    turnstileEnabled,
    turnstileSiteKey,
    turnstileToken,
    setTurnstileToken,
    validateTurnstile,
  } = useTurnstile();

  // 从 URL 读取 code、以及初始化管理员配置 & 用户余额
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const codeFromUrl = params.get('code');
    if (codeFromUrl) {
      setRedemptionCode(codeFromUrl);
      setUseUrlCode(true);
    }

    // 读取本地管理员配置
    let status = localStorage.getItem('status');
    if (status) {
      status = JSON.parse(status);
      if (status.top_up_link) {
        setTopUpLink(status.top_up_link);
      }
      if (status.min_topup) {
        setMinTopUp(parseInt(status.min_topup));
      }
      if (status.enable_online_topup) {
        setEnableOnlineTopUp(status.enable_online_topup);
      }
    }

    getUserQuota();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 点击“点我更改”
  const handleResetCode = () => {
    setRedemptionCode('');
    setUseUrlCode(false);
    setRedeemError('');
  };

  // 获取用户当前额度
  const getUserQuota = async () => {
    try {
      const res = await API.get('/api/user/self');
      const { success, message, data } = res.data;
      if (success) {
        setUserQuota(data.quota);
      } else {
        showError(message);
      }
    } catch (err) {
      showError(t('请求失败'));
    }
  };

  // 兑换余额
  const topUp = async () => {
    setRedeemError('');
    
    if (!redemptionCode) {
      setRedeemError(t('请输入兑换码！'));
      return;
    }
    if (!validateTurnstile()) {
      setRedeemError(t('请完成人机验证！'));
      return;
    }
    setIsSubmitting(true);
    try {
      const res = await API.post(`/api/user/topup?turnstile=${turnstileToken}`, {
        key: redemptionCode,
      });
      const { success, message, data } = res.data;
      if (success) {
        let successMessage = t('兑换成功！');
        if (data.is_gift) {
          successMessage = t('礼品码兑换成功！');
        }
        showSuccess(successMessage);

        const quotaAmount = parseInt(data.quota, 10);
        Modal.success({
          title: successMessage,
          content: t('成功兑换额度：') + renderQuotaWithAmount(quotaAmount),
          centered: true,
        });
        setUserQuota((q) => q + quotaAmount);

        // 兑换成功后清空，并恢复输入框
        setRedemptionCode('');
        setUseUrlCode(false);
        setShowRedeemModal(false);
        setRedeemError('');
      } else {
        setRedeemError(message);
      }
    } catch (err) {
      setRedeemError(t('请求失败'));
    } finally {
      setIsSubmitting(false);
    }
  };

  // 打开外部充值链接
  const openTopUpLink = () => {
    if (!topUpLink) {
      showError(t('超级管理员未设置充值链接！'));
      return;
    }
    window.open(topUpLink, '_blank');
  };

  // 预充值：校验后显示确认信息
  const preTopUp = async (payment) => {
    setTopUpError('');
    
    if (!enableOnlineTopUp) {
      setTopUpError(t('管理员未开启在线充值！'));
      return;
    }
    
    if (topUpCount < minTopUp / 500000) {
      setTopUpError(t('充值金额不能小于 $') + (minTopUp / 500000).toFixed(2));
      return;
    }
    
    // 请求计算实际支付金额和单价
    await getAmount(topUpCount);
    
    setPayWay(payment);
    setShowPaymentConfirm(true);
  };

  // 执行在线充值
  const onlineTopUp = async () => {
    if (amount === 0) {
      await getAmount(topUpCount);
    }
    if (topUpCount < minTopUp) {
      showError(t('充值数量不能小于') + minTopUp);
      return;
    }
    setShowTopUpModal(false);
    setShowPaymentConfirm(false);
    try {
      const res = await API.post('/api/user/pay', {
        amount: parseInt(topUpCount),
        top_up_code: topUpCode,
        payment_method: payWay,
      });
      const { message, data, url } = res.data;
      if (message === 'success') {
        // 动态构建表单并提交
        const form = document.createElement('form');
        form.action = url;
        form.method = 'POST';
        const isSafari =
          navigator.userAgent.indexOf('Safari') > -1 &&
          navigator.userAgent.indexOf('Chrome') < 0;
        if (!isSafari) {
          form.target = '_blank';
        }
        Object.keys(data).forEach((key) => {
          const input = document.createElement('input');
          input.type = 'hidden';
          input.name = key;
          input.value = data[key];
          form.appendChild(input);
        });
        document.body.appendChild(form);
        form.submit();
        document.body.removeChild(form);
      } else {
        showError(data);
      }
    } catch (err) {
      console.error(err);
      showError(t('支付请求失败'));
    }
  };

  // 获取实际需支付金额
  const getAmount = async (value) => {
    if (value === undefined) {
      value = topUpCount;
    }
    try {
      const res = await API.post('/api/user/amount', {
        amount: parseFloat(value),
        top_up_code: topUpCode,
      });
      const { message, data } = res.data;
      if (message === 'success') {
        // 兼容新旧两种响应格式
        if (typeof data === 'object' && data.amount !== undefined) {
          setAmount(parseFloat(data.amount));
          setUnitPrice(parseFloat(data.unit_price));
        } else {
          // 旧格式兼容
          setAmount(parseFloat(data));
          const calculatedPrice = parseFloat(data) / parseFloat(value);
          setUnitPrice(calculatedPrice);
        }
      } else {
        setAmount(0);
        setUnitPrice(0);
        Toast.error({ content: '错误：' + data, id: 'getAmount' });
      }
    } catch (err) {
      console.error(err);
    }
  };

  const handleCancel = () => {
    setOpen(false);
    setTopUpError('');
    setShowPaymentConfirm(false);
  };

  // 渲染实付金额文字
  const renderAmountText = () => {
    return amount + ' ' + t('元');
  };

  const formatQuotaAsCurrency = (quota) => {
    const amount = quota / 500000;
    return `$${amount.toFixed(2)}`;
  };

  return (
    <Layout>
        <Layout.Header>
            <h3>计费与订阅</h3>
        </Layout.Header>
      <Layout.Content>

        {/* 兑换码弹窗 */}
        <Modal
          title={t('使用兑换码')}
          visible={showRedeemModal}
          onCancel={() => {
            setShowRedeemModal(false);
            setRedeemError('');
          }}
          footer={
              <Space style={{ marginTop: 16 }}>
                  {topUpLink && (
                      <Button
                          type="primary"
                          theme='borderless'
                          onClick={openTopUpLink}
                      >
                          {t('获取兑换码')}
                      </Button>
                  )}
                  <Button
                      type="primary"
                      theme="solid"
                      onClick={() => {
                          topUp();
                          setShowRedeemModal(false);
                      }}
                      disabled={isSubmitting}
                  >
                      {isSubmitting ? t('兑换中...') : t('兑换')}
                  </Button>
              </Space>
          }
          maskClosable={true}
          width={500}
          centered
        >
          <Form>
            {useUrlCode ? (
              <p>
                {t('已识别到兑换码')}:&nbsp;
                <strong>{redemptionCode}</strong>。&nbsp;
                <a
                  style={{ cursor: 'pointer', color: '#1890ff' }}
                  onClick={handleResetCode}
                >
                  {t('点我更改')}
                </a>
              </p>
            ) : (
              <Form.Input
                field="redemptionCode"
                label={t('兑换码')}
                placeholder={t('请输入兑换码')}
                value={redemptionCode}
                onChange={(value) => {
                  setRedemptionCode(value);
                  setRedeemError('');
                }}
                size='large'
                validateStatus={redeemError ? 'error' : 'default'}
                error={redeemError}
              />
            )}

            <TurnstileWrapper
              enabled={turnstileEnabled}
              siteKey={turnstileSiteKey}
              onVerify={setTurnstileToken}
            />

          </Form>
        </Modal>

        {/* 充值弹窗 */}
        <Modal
          title={showPaymentConfirm ? t('确认付款信息') : t('在线充值')}
          visible={showTopUpModal}
          onCancel={() => {
            setShowTopUpModal(false);
            setTopUpError('');
            setShowPaymentConfirm(false);
          }}
          footer={
            enableOnlineTopUp ? (
              <div style={{ textAlign: 'right' }}>
                {showPaymentConfirm && (
                  <Button
                    onClick={() => setShowPaymentConfirm(false)}
                    style={{ marginRight: 8 }}
                  >
                    {t('返回')}
                  </Button>
                )}
                <Button
                  type="primary"
                  theme="solid"
                  onClick={() => {
                    if (showPaymentConfirm) {
                      onlineTopUp();
                    } else {
                      preTopUp(payWay);
                    }
                  }}
                >
                  {showPaymentConfirm ? t('确认支付') : t('继续')}
                </Button>
              </div>
            ) : null
          }
          maskClosable={true}
          width={500}
          centered
        >
          {enableOnlineTopUp ? (
            showPaymentConfirm ? (
              <div>
                {/* 支付确认表格 */}
                <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                  <thead>
                    <tr style={{ borderBottom: '1px solid var(--semi-color-border)' }}>
                      <th style={{ padding: '12px', textAlign: 'left', fontWeight: 'normal', color: 'var(--semi-color-text-2)' }}>
                        {t('描述')}
                      </th>
                      <th style={{ padding: '12px', textAlign: 'center', fontWeight: 'normal', color: 'var(--semi-color-text-2)' }}>
                        {t('单价')}
                      </th>
                      <th style={{ padding: '12px', textAlign: 'right', fontWeight: 'normal', color: 'var(--semi-color-text-2)' }}>
                        {t('总价')}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr>
                      <td style={{ padding: '16px 12px' }}>
                        {localStorage.getItem('sitename') || 'API'} {t('额度')}
                      </td>
                      <td style={{ padding: '16px 12px', textAlign: 'center' }}>
                        {unitPrice ? `$ ${Number(unitPrice).toFixed(4)}` : '-'}
                      </td>
                      <td style={{ padding: '16px 12px', textAlign: 'right', fontWeight: 'bold', fontSize: '16px' }}>
                        ￥{amount.toFixed(2)}
                      </td>
                    </tr>
                  </tbody>
                  <tfoot>
                    <tr style={{ borderTop: '2px solid var(--semi-color-border)' }}>
                      <td style={{ padding: '16px 12px', fontWeight: 'bold' }}>
                        {t('总计')}
                      </td>
                      <td style={{ padding: '16px 12px' }}>
                      </td>
                      <td style={{ padding: '16px 12px', textAlign: 'right', fontWeight: 'bold', fontSize: '18px', color: 'var(--semi-color-primary)' }}>
                        ￥{amount.toFixed(2)}
                      </td>
                    </tr>
                  </tfoot>
                </table>
                
                <div style={{ marginTop: 24, paddingTop: 20, borderTop: '1px solid var(--semi-color-border)' }}>
                  <Form.Label style={{ marginBottom: 8, display: 'block' }}>
                    {t('支付方式')}
                  </Form.Label>
                  <RadioGroup 
                    type='pureCard' 
                    value={payWay}
                    onChange={(e) => setPayWay(e.target.value)}
                    direction='vertical' 
                    aria-label="支付方式选择"
                    name="payment-method"
                    style={{ rowGap: '0px' }}
                  >
                    <Radio 
                      value='zfb'
                      style={{ width: '100%', marginBottom: 4 }}
                    >
                      {t('支付宝')}
                    </Radio>
                    <Radio 
                      value='wx'
                      style={{ width: '100%' }}
                    >
                      {t('微信')}
                    </Radio>
                  </RadioGroup>
                </div>
              </div>
            ) : (
              <Form>
                <Form.Input
                  disabled={!enableOnlineTopUp}
                  field="redemptionCount"
                  label="充值金额"
                  placeholder={t('请输入充值金额')}
                  type="number"
                  value={topUpCount}
                  onChange={(value) => {
                    if (value < 1) value = 1;
                    setTopUpCount(value);
                    setTopUpError('');
                  }}
                  prefix="$"
                  size='large'
                  validateStatus={topUpError ? 'error' : 'default'}
                  error={topUpError}
                  extraText={
                    topUpError || (
                      <span style={{ color: 'var(--semi-color-text-2)', fontSize: '12px' }}>
                        {t('请输入大于')} ${(minTopUp / 500000).toFixed(2)} {t('的数值')}
                      </span>
                    )
                  }
                />
                
                <div style={{ marginTop: 20 }}>
                  <Form.Label style={{ marginBottom: 8, display: 'block' }}>
                    {t('支付方式')}
                  </Form.Label>
                  <RadioGroup 
                    type='pureCard' 
                    value={payWay}
                    onChange={(e) => setPayWay(e.target.value)}
                    direction='vertical' 
                    aria-label="支付方式选择"
                    name="payment-method"
                    style={{ rowGap: '0px' }}
                  >
                    <Radio 
                      value='zfb'
                      style={{ width: '100%', marginBottom: 4 }}
                    >
                      {t('支付宝')}
                    </Radio>
                    <Radio 
                      value='wx'
                      style={{ width: '100%' }}
                    >
                      {t('微信')}
                    </Radio>
                  </RadioGroup>
                </div>
              </Form>
            )
          ) : (
            <Banner
              fullMode={false}
              type="info"
              bordered
              icon={null}
              closeIcon={null}
              title={
                <div
                  style={{
                    fontWeight: 'bold',
                    fontSize: '14px',
                    lineHeight: '22px',
                  }}
                >
                  {t('管理员已关闭在线充值')}
                </div>
              }
            />
          )}
        </Modal>

        {/* 主界面内容 */}
        <div
          style={{
            padding: '4rem',
            maxWidth: '100%',
            '@media (max-width: 768px)': {
              padding: '1rem',
            },
          }}
        >
          <Row gutter={[32, 32]}>
            <Col span={12}>
              <div style={{ marginBottom: '1rem' }}>
                <span style={{ fontSize: '1rem', color: 'var(--semi-color-text-1)' }}>
                  当前余额
                </span>
              </div>
              
              <h2 style={{ 
                fontSize: '2rem', 
                fontWeight: 'bold',
                margin: '0 0 1.5rem 0',
                color: 'var(--semi-color-text-0)'
              }}>
                {formatQuotaAsCurrency(userQuota)}
              </h2>
            </Col>
            
            <Col span={12}>
              <div style={{ marginBottom: '1rem' }}>
                <span style={{ fontSize: '1rem', color: 'var(--semi-color-text-1)' }}>
                  当前计划
                </span>
              </div>
              
              <h2 style={{ 
                fontSize: '2rem', 
                fontWeight: 'bold',
                margin: '0 0 1.5rem 0',
                color: 'var(--semi-color-text-0)'
              }}>
                即用即付
              </h2>
            </Col>
          </Row>

          <Space wrap>
            <Button
              type="primary"
              theme="solid"
              size="large"
              onClick={() => setShowTopUpModal(true)}
            >
              {t('充值')}
            </Button>
            <Button
              theme="light"
              size="large"
              onClick={() => setShowRedeemModal(true)}
            >
              {t('使用兑换码')}
            </Button>
          </Space>
        </div>
      </Layout.Content>
    </Layout>
  );
};

export default TopUp;
