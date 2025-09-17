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
import React from 'react';
import { Layout, Card, Button, Space } from '@douyinfe/semi-ui';
// import { IconCheckCircle } from '@douyinfe/semi-icons';
import Title from '@douyinfe/semi-ui/lib/es/typography/title';
import Text from '@douyinfe/semi-ui/lib/es/typography/text';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';

const TopUpSuccess = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const handleBackToWallet = () => {
    navigate('/app/wallet');
  };

  const handleGoHome = () => {
    navigate('/');
  };

  return (
    <Layout>
      <Layout.Header>
        <h3>{t('充值成功')}</h3>
      </Layout.Header>
      <Layout.Content>
        <div
          style={{
            marginTop: 60,
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            minHeight: '60vh',
          }}
        >
          <Card 
            style={{ 
              width: '500px', 
              padding: '40px 20px',
              textAlign: 'center',
              boxShadow: '0 4px 12px rgba(0, 0, 0, 0.1)',
            }}
          >
            {/* 成功图标 */}
            <div style={{ marginBottom: 24 }}>

            </div>

            {/* 标题 */}
            <Title 
              level={2} 
              style={{ 
                textAlign: 'center',
                marginBottom: 16,
                color: 'var(--semi-color-text-0)'
              }}
            >
              {t('充值成功')}
            </Title>

            {/* 描述文字 */}
            <Text 
              type="secondary" 
              style={{ 
                fontSize: '16px',
                lineHeight: '24px',
                display: 'block',
                marginBottom: 32
              }}
            >
              {t('额度已被添加到您的账户。')}
            </Text>

            {/* 操作按钮 */}
            <Space size="large">
              <Button 
                type="primary" 
                theme="solid"
                onClick={handleBackToWallet}
                size="large"
              >
                {t('返回计费页面')}
              </Button>
              <Button 
                type="tertiary"
                onClick={handleGoHome}
                size="large"
              >
                {t('返回首页')}
              </Button>
            </Space>
          </Card>
        </div>
      </Layout.Content>
    </Layout>
  );
};

export default TopUpSuccess;