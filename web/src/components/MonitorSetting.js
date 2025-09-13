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
import { Card, Spin } from '@douyinfe/semi-ui';
import SettingsLog from '../pages/Setting/Operation/SettingsLog.js';
import SettingsDataDashboard from '../pages/Setting/Operation/SettingsDataDashboard.js';
import SettingsMonitoring from '../pages/Setting/Operation/SettingsMonitoring.js';
import RateLimitSetting from './RateLimitSetting.js';
import { API, showError } from '../helpers';

const MonitorSetting = () => {
  let [inputs, setInputs] = useState({
    LogConsumeEnabled: false,
    LogChatContentEnabled: false,
    LogErrorEnabled: false,
    DisplayInCurrencyEnabled: false,
    DisplayTokenStatEnabled: false,
    DataExportEnabled: false,
    DataExportDefaultTime: 'hour',
    DataExportInterval: 5,
    AutomaticDisableChannelEnabled: false,
    AutomaticEnableChannelEnabled: false,
    ChannelDisableThreshold: 0,
    AutomaticDisableKeywords: '',
  });

  let [loading, setLoading] = useState(false);

  const getOptions = async () => {
    const res = await API.get('/api/option/');
    const { success, message, data } = res.data;
    if (success) {
      let newInputs = {};
      data.forEach((item) => {
        if (item.key in inputs) {
          if (item.key.endsWith('Enabled')) {
            newInputs[item.key] = item.value === 'true' ? true : false;
          } else {
            newInputs[item.key] = item.value;
          }
        }
      });

      setInputs(newInputs);
    } else {
      showError(message);
    }
  };

  async function onRefresh() {
    try {
      setLoading(true);
      await getOptions();
    } catch (error) {
      showError('刷新失败');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    onRefresh();
  }, []);

  return (
    <>
      <Spin spinning={loading} size='large'>
        <Card style={{ marginTop: '10px' }}>
          <SettingsMonitoring options={inputs} refresh={onRefresh} />
        </Card>
        
        <Card style={{ marginTop: '10px' }}>
          <SettingsLog options={inputs} refresh={onRefresh} />
        </Card>
        
        <Card style={{ marginTop: '10px' }}>
          <SettingsDataDashboard options={inputs} refresh={onRefresh} />
        </Card>
        
        <Card style={{ marginTop: '10px' }}>
          <RateLimitSetting />
        </Card>
      </Spin>
    </>
  );
};

export default MonitorSetting;