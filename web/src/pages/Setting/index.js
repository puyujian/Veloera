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
import { Layout, TabPane, Tabs } from '@douyinfe/semi-ui';
import { useNavigate, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

import { isRoot } from '../../helpers';
import PersonalSetting from '../../components/PersonalSetting';
import BasicSetting from '../../components/BasicSetting.js';
import AuthSetting from '../../components/AuthSetting.js';
import BusinessSetting from '../../components/BusinessSetting.js';
import MonitorSetting from '../../components/MonitorSetting.js';
import ContentSetting from '../../components/ContentSetting.js';
import UISetting from '../../components/UISetting.js';
import ModelSetting from '../../components/ModelSetting.js';

const Setting = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [tabActiveKey, setTabActiveKey] = useState('1');
  let panes = [];

  if (isRoot()) {
    panes.push({
      tab: t('基础配置'),
      content: <BasicSetting />,
      itemKey: 'basic',
    });
    panes.push({
      tab: t('用户与认证'),
      content: <AuthSetting />,
      itemKey: 'auth',
    });
    panes.push({
      tab: t('业务运营'),
      content: <BusinessSetting />,
      itemKey: 'business',
    });
    panes.push({
      tab: t('模型管理'),
      content: <ModelSetting />,
      itemKey: 'models',
    });
    panes.push({
      tab: t('监控与日志'),
      content: <MonitorSetting />,
      itemKey: 'monitor',
    });
    panes.push({
      tab: t('内容管理'),
      content: <ContentSetting />,
      itemKey: 'content',
    });
    panes.push({
      tab: t('界面定制'),
      content: <UISetting />,
      itemKey: 'ui',
    });
  }
  const onChangeTab = (key) => {
    setTabActiveKey(key);
    navigate(`?tab=${key}`);
  };
  useEffect(() => {
    const searchParams = new URLSearchParams(window.location.search);
    const tab = searchParams.get('tab');
    if (tab) {
      setTabActiveKey(tab);
    } else {
      onChangeTab('basic');
    }
  }, [location.search]);
  return (
    <div>
      <Layout>
        <Layout.Content>
          <Tabs
            type='line'
            activeKey={tabActiveKey}
            onChange={(key) => onChangeTab(key)}
          >
            {panes.map((pane) => (
              <TabPane itemKey={pane.itemKey} tab={pane.tab} key={pane.itemKey}>
                {tabActiveKey === pane.itemKey && pane.content}
              </TabPane>
            ))}
          </Tabs>
        </Layout.Content>
      </Layout>
    </div>
  );
};

export default Setting;
