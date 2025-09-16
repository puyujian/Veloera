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
import SettingsGeneral from '../pages/Setting/Operation/SettingsGeneral.js';
import SettingsCheckIn from '../pages/Setting/Operation/SettingsCheckIn.js';
import SettingsCreditLimit from '../pages/Setting/Operation/SettingsCreditLimit.js';
import SettingsRebate from '../pages/Setting/Operation/SettingsRebate.js';
import PaymentSetting from './PaymentSetting.js';
import ModelRatioSettings from '../pages/Setting/Operation/ModelRatioSettings.js';
import GroupRatioSettings from '../pages/Setting/Operation/GroupRatioSettings.js';
import ModelCommonRatioSettings from '../pages/Setting/Operation/ModelCommonRatioSettings.js';
import ModelSettingsVisualEditor from '../pages/Setting/Operation/ModelSettingsVisualEditor.js';
import ModelRatioNotSetEditor from '../pages/Setting/Operation/ModelRationNotSetEditor.js';
import { API, showError } from '../helpers';

const BusinessSetting = () => {
  let [inputs, setInputs] = useState({
    QuotaForNewUser: 0,
    QuotaForInviter: 0,
    QuotaForInvitee: 0,
    QuotaRemindThreshold: 0,
    PreConsumedQuota: 0,
    StreamCacheQueueLength: 0,
    ModelRatio: '',
    CacheRatio: '',
    CompletionRatio: '',
    ModelPrice: '',
    GroupRatio: '',
    UserUsableGroups: '',
    TopUpLink: '',
    'general_setting.docs_link': '',
    QuotaPerUnit: 0,
    AutomaticDisableChannelEnabled: false,
    AutomaticEnableChannelEnabled: false,
    ChannelDisableThreshold: 0,
    DefaultCollapseSidebar: false,
    RetryTimes: 0,
    DemoSiteEnabled: false,
    SelfUseModeEnabled: false,
    AutomaticDisableKeywords: '',
    CheckInEnabled: false,
    CheckInQuota: '',
    CheckInMaxQuota: '',
    RebateEnabled: false,
    RebatePercentage: 0,
    AffEnabled: false,
    PayAddress: '',
    EpayId: '',
    EpayKey: '',
    Price: 7.3,
    MinTopUp: 1,
    TopupGroupRatio: '',
    CustomCallbackAddress: '',
    DisplayInCurrencyEnabled: false,
    DisplayTokenStatEnabled: false,
  });

  let [loading, setLoading] = useState(false);

  const getOptions = async () => {
    const res = await API.get('/api/option/');
    const { success, message, data } = res.data;
    if (success) {
      let newInputs = {};
      data.forEach((item) => {
        if (item.key in inputs) {
          if (item.key === 'TopupGroupRatio') {
            item.value = JSON.stringify(JSON.parse(item.value), null, 2);
          }
          if (item.key === 'ModelRatio' || item.key === 'GroupRatio' || item.key === 'CompletionRatio' || item.key === 'ModelPrice') {
            item.value = JSON.stringify(JSON.parse(item.value), null, 2);
          }
          if (
            item.key.endsWith('Enabled') ||
            ['DefaultCollapseSidebar'].includes(item.key)
          ) {
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
          <SettingsGeneral options={inputs} refresh={onRefresh} />
        </Card>
        
        <PaymentSetting />
        
        <Card style={{ marginTop: '10px' }}>
          <ModelSettingsVisualEditor options={inputs} refresh={onRefresh} />
        </Card>
        
        <Card style={{ marginTop: '10px' }}>
          <ModelRatioSettings options={inputs} refresh={onRefresh} />
        </Card>
        
        <Card style={{ marginTop: '10px' }}>
          <ModelCommonRatioSettings options={inputs} refresh={onRefresh} />
        </Card>
        
        <Card style={{ marginTop: '10px' }}>
          <GroupRatioSettings options={inputs} refresh={onRefresh} />
        </Card>
        
        <Card style={{ marginTop: '10px' }}>
          <ModelRatioNotSetEditor />
        </Card>
        
        <Card style={{ marginTop: '10px' }}>
          <SettingsCheckIn options={inputs} refresh={onRefresh} />
        </Card>
        
        <Card style={{ marginTop: '10px' }}>
          <SettingsCreditLimit options={inputs} refresh={onRefresh} />
        </Card>
        
        <Card style={{ marginTop: '10px' }}>
          <SettingsRebate options={inputs} refresh={onRefresh} />
        </Card>
      </Spin>
    </>
  );
};

export default BusinessSetting;