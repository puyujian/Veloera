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
import axios from 'axios';
import {
  API,
  isMobile,
  shouldShowPrompt,
  showError,
  showInfo,
  showSuccess,
  showWarning,
  timestamp2string,
} from '../helpers';

import { CHANNEL_OPTIONS, ITEMS_PER_PAGE } from '../constants';
import {
  getQuotaPerUnit,
  renderGroup,
  renderNumberWithPoint,
  renderQuota,
  renderQuotaWithPrompt,
  stringToColor,
} from '../helpers/render';
import {
  Button,
  Divider,
  Dropdown,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Space,
  SplitButtonGroup,
  Switch,
  Table,
  Tag,
  Tooltip,
  Typography,
  Checkbox,
  Layout,
} from '@douyinfe/semi-ui';
import EditChannel from '../pages/Channel/EditChannel';
import {
  IconList,
  IconTreeTriangleDown,
  IconClose,
  IconFilter,
  IconPlus,
  IconRefresh,
  IconSetting,
  IconCopy,
} from '@douyinfe/semi-icons';
import { loadChannelModels } from './utils.js';
import EditTagModal from '../pages/Channel/EditTagModal.js';
import TextNumberInput from './custom/TextNumberInput.js';
import { useTranslation } from 'react-i18next';

function renderTimestamp(timestamp) {
  return <>{timestamp2string(timestamp)}</>;
}

const ChannelsTable = () => {
  const { t } = useTranslation();

  let type2label = undefined;

  const renderType = (type) => {
    if (!type2label) {
      type2label = new Map();
      for (let i = 0; i < CHANNEL_OPTIONS.length; i++) {
        type2label[CHANNEL_OPTIONS[i].value] = CHANNEL_OPTIONS[i];
      }
      type2label[0] = { value: 0, label: t('未知类型'), color: 'grey' };
    }
    return (
      <Tag size='large' color={type2label[type]?.color}>
        {type2label[type]?.label}
      </Tag>
    );
  };

  const renderTagType = () => {
    return (
      <Tag
        color='light-blue'
        prefixIcon={<IconList />}
        size='large'
        shape='circle'
        type='light'
      >
        {t('标签聚合')}
      </Tag>
    );
  };

  const renderStatus = (status) => {
    switch (status) {
      case 1:
        return (
          <Tag size='large' color='green'>
            {t('已启用')}
          </Tag>
        );
      case 2:
        return (
          <Tag size='large' color='yellow'>
            {t('已禁用')}
          </Tag>
        );
      case 3:
        return (
          <Tag size='large' color='yellow'>
            {t('自动禁用')}
          </Tag>
        );
      default:
        return (
          <Tag size='large' color='grey'>
            {t('未知状态')}
          </Tag>
        );
    }
  };

  const renderResponseTime = (responseTime) => {
    let time = responseTime / 1000;
    time = time.toFixed(2) + t(' 秒');
    if (responseTime === 0) {
      return (
        <Tag size='large' color='grey'>
          {t('未测试')}
        </Tag>
      );
    } else if (responseTime <= 1000) {
      return (
        <Tag size='large' color='green'>
          {time}
        </Tag>
      );
    } else if (responseTime <= 3000) {
      return (
        <Tag size='large' color='lime'>
          {time}
        </Tag>
      );
    } else if (responseTime <= 5000) {
      return (
        <Tag size='large' color='yellow'>
          {time}
        </Tag>
      );
    } else {
      return (
        <Tag size='large' color='red'>
          {time}
        </Tag>
      );
    }
  };

  // Define column keys for selection
  const COLUMN_KEYS = {
    ID: 'id',
    NAME: 'name',
    GROUP: 'group',
    TYPE: 'type',
    STATUS: 'status',
    RESPONSE_TIME: 'response_time',
    BALANCE: 'balance',
    PRIORITY: 'priority',
    WEIGHT: 'weight',
    OPERATE: 'operate',
  };

  // State for column visibility
  const [visibleColumns, setVisibleColumns] = useState({});
  const [showColumnSelector, setShowColumnSelector] = useState(false);

  // Load saved column preferences from localStorage
  useEffect(() => {
    const savedColumns = localStorage.getItem('channels-table-columns');
    if (savedColumns) {
      try {
        const parsed = JSON.parse(savedColumns);
        // Make sure all columns are accounted for
        const defaults = getDefaultColumnVisibility();
        const merged = { ...defaults, ...parsed };
        setVisibleColumns(merged);
      } catch (e) {
        console.error('Failed to parse saved column preferences', e);
        initDefaultColumns();
      }
    } else {
      initDefaultColumns();
    }
  }, []);

  // Update table when column visibility changes
  useEffect(() => {
    if (Object.keys(visibleColumns).length > 0) {
      // Save to localStorage
      localStorage.setItem(
        'channels-table-columns',
        JSON.stringify(visibleColumns),
      );
    }
  }, [visibleColumns]);

  // Get default column visibility
  const getDefaultColumnVisibility = () => {
    return {
      [COLUMN_KEYS.ID]: true,
      [COLUMN_KEYS.NAME]: true,
      [COLUMN_KEYS.GROUP]: true,
      [COLUMN_KEYS.TYPE]: true,
      [COLUMN_KEYS.STATUS]: true,
      [COLUMN_KEYS.RESPONSE_TIME]: true,
      [COLUMN_KEYS.BALANCE]: true,
      [COLUMN_KEYS.PRIORITY]: true,
      [COLUMN_KEYS.WEIGHT]: true,
      [COLUMN_KEYS.OPERATE]: true,
    };
  };

  // Initialize default column visibility
  const initDefaultColumns = () => {
    const defaults = getDefaultColumnVisibility();
    setVisibleColumns(defaults);
  };

  // Handle column visibility change
  const handleColumnVisibilityChange = (columnKey, checked) => {
    const updatedColumns = { ...visibleColumns, [columnKey]: checked };
    setVisibleColumns(updatedColumns);
  };

  // Handle "Select All" checkbox
  const handleSelectAll = (checked) => {
    const allKeys = Object.keys(COLUMN_KEYS).map((key) => COLUMN_KEYS[key]);
    const updatedColumns = {};

    allKeys.forEach((key) => {
      updatedColumns[key] = checked;
    });

    setVisibleColumns(updatedColumns);
  };

  // Define all columns with keys
  const allColumns = [
    {
      key: COLUMN_KEYS.ID,
      title: t('ID'),
      dataIndex: 'id',
    },
    {
      key: COLUMN_KEYS.NAME,
      title: t('名称'),
      dataIndex: 'name',
    },
    {
      key: COLUMN_KEYS.GROUP,
      title: t('分组'),
      dataIndex: 'group',
      render: (text, record, index) => {
        return (
          <div>
            <Space spacing={2}>
              {text
                ?.split(',')
                .sort((a, b) => {
                  if (a === 'default') return -1;
                  if (b === 'default') return 1;
                  return a.localeCompare(b);
                })
                .map((item, index) => {
                  return renderGroup(item);
                })}
            </Space>
          </div>
        );
      },
    },
    {
      key: COLUMN_KEYS.TYPE,
      title: t('类型'),
      dataIndex: 'type',
      render: (text, record, index) => {
        if (record.children === undefined) {
          return <>{renderType(text)}</>;
        } else {
          return <>{renderTagType()}</>;
        }
      },
    },
    {
      key: COLUMN_KEYS.STATUS,
      title: t('状态'),
      dataIndex: 'status',
      render: (text, record, index) => {
        if (text === 3) {
          if (record.other_info === '') {
            record.other_info = '{}';
          }
          let otherInfo = JSON.parse(record.other_info);
          let reason = otherInfo['status_reason'];
          let time = otherInfo['status_time'];
          return (
            <div>
              <Tooltip
                content={
                  t('原因：') + reason + t('，时间：') + timestamp2string(time)
                }
              >
                {renderStatus(text)}
              </Tooltip>
            </div>
          );
        } else {
          return renderStatus(text);
        }
      },
    },
    {
      key: COLUMN_KEYS.RESPONSE_TIME,
      title: t('响应时间'),
      dataIndex: 'response_time',
      render: (text, record, index) => {
        return <div>{renderResponseTime(text)}</div>;
      },
    },
    {
      key: COLUMN_KEYS.BALANCE,
      title: t('已用/剩余'),
      dataIndex: 'expired_time',
      render: (text, record, index) => {
        if (record.children === undefined) {
          return (
            <div>
              <Space spacing={1}>
                <Tooltip content={t('已用额度')}>
                  <Tag color='white' type='ghost' size='large'>
                    {renderQuota(record.used_quota)}
                  </Tag>
                </Tooltip>
                <Tooltip
                  content={t('剩余额度') + record.balance + t('，点击更新')}
                >
                  <Tag
                    color='white'
                    type='ghost'
                    size='large'
                    onClick={() => {
                      updateChannelBalance(record);
                    }}
                  >
                    ${renderNumberWithPoint(record.balance)}
                  </Tag>
                </Tooltip>
              </Space>
            </div>
          );
        } else {
          return (
            <Tooltip content={t('已用额度')}>
              <Tag color='white' type='ghost' size='large'>
                {renderQuota(record.used_quota)}
              </Tag>
            </Tooltip>
          );
        }
      },
    },
    {
      key: COLUMN_KEYS.PRIORITY,
      title: t('优先级'),
      dataIndex: 'priority',
      render: (text, record, index) => {
        if (record.children === undefined) {
          return (
            <div>
              <InputNumber
                style={{ width: 70 }}
                name='priority'
                onBlur={(e) => {
                  manageChannel(record.id, 'priority', record, e.target.value);
                }}
                keepFocus={true}
                innerButtons
                defaultValue={record.priority}
                min={-999}
              />
            </div>
          );
        } else {
          return (
            <>
              <InputNumber
                style={{ width: 70 }}
                name='priority'
                keepFocus={true}
                onBlur={(e) => {
                  Modal.warning({
                    title: t('修改子渠道优先级'),
                    content:
                      t('确定要修改所有子渠道优先级为 ') +
                      e.target.value +
                      t(' 吗？'),
                    onOk: () => {
                      if (e.target.value === '') {
                        return;
                      }
                      submitTagEdit('priority', {
                        tag: record.key,
                        priority: e.target.value,
                      });
                    },
                  });
                }}
                innerButtons
                defaultValue={record.priority}
                min={-999}
              />
            </>
          );
        }
      },
    },
    {
      key: COLUMN_KEYS.WEIGHT,
      title: t('权重'),
      dataIndex: 'weight',
      render: (text, record, index) => {
        if (record.children === undefined) {
          return (
            <div>
              <InputNumber
                style={{ width: 70 }}
                name='weight'
                onBlur={(e) => {
                  manageChannel(record.id, 'weight', record, e.target.value);
                }}
                keepFocus={true}
                innerButtons
                defaultValue={record.weight}
                min={0}
              />
            </div>
          );
        } else {
          return (
            <InputNumber
              style={{ width: 70 }}
              name='weight'
              keepFocus={true}
              onBlur={(e) => {
                Modal.warning({
                  title: t('修改子渠道权重'),
                  content:
                    t('确定要修改所有子渠道权重为 ') +
                    e.target.value +
                    t(' 吗？'),
                  onOk: () => {
                    if (e.target.value === '') {
                      return;
                    }
                    submitTagEdit('weight', {
                      tag: record.key,
                      weight: e.target.value,
                    });
                  },
                });
              }}
              innerButtons
              defaultValue={record.weight}
              min={-999}
            />
          );
        }
      },
    },
    {
      key: COLUMN_KEYS.OPERATE,
      title: '',
      dataIndex: 'operate',
      render: (text, record, index) => {
        if (record.children === undefined) {
          return (
            <div>
              <SplitButtonGroup
                style={{ marginRight: 1 }}
                aria-label={t('测试单个渠道操作项目组')}
              >
                <Button
                  theme='light'
                  onClick={() => {
                    testChannel(record, '');
                  }}
                >
                  {t('测试')}
                </Button>
                <Button
                  style={{ padding: '8px 4px' }}
                  type='primary'
                  icon={<IconTreeTriangleDown />}
                  onClick={() => {
                    setCurrentTestChannel(record);
                    setShowModelTestModal(true);
                  }}
                ></Button>
              </SplitButtonGroup>
              <Popconfirm
                title={t('确定是否要删除此渠道？')}
                content={t('此修改将不可逆')}
                okType={'danger'}
                position={'left'}
                onConfirm={() => {
                  manageChannel(record.id, 'delete', record).then(() => {
                    removeRecord(record);
                  });
                }}
              >
                <Button theme='light' type='danger' style={{ marginRight: 1 }}>
                  {t('删除')}
                </Button>
              </Popconfirm>
              {record.status === 1 ? (
                <Button
                  theme='light'
                  type='warning'
                  style={{ marginRight: 1 }}
                  onClick={async () => {
                    manageChannel(record.id, 'disable', record);
                  }}
                >
                  {t('禁用')}
                </Button>
              ) : (
                <Button
                  theme='light'
                  type='secondary'
                  style={{ marginRight: 1 }}
                  onClick={async () => {
                    manageChannel(record.id, 'enable', record);
                  }}
                >
                  {t('启用')}
                </Button>
              )}
              <Button
                theme='light'
                type='tertiary'
                style={{ marginRight: 1 }}
                onClick={() => {
                  setEditingChannel(record);
                  setShowEdit(true);
                }}
              >
                {t('编辑')}
              </Button>
              <Popconfirm
                title={t('确定是否要复制此渠道？')}
                content={t('复制渠道的所有信息')}
                okType={'danger'}
                position={'left'}
                onConfirm={async () => {
                  copySelectedChannel(record);
                }}
              >
                <Button theme='light' type='primary' style={{ marginRight: 1 }}>
                  {t('复制')}
                </Button>
              </Popconfirm>
            </div>
          );
        } else {
          return (
            <>
              <Button
                theme='light'
                type='secondary'
                style={{ marginRight: 1 }}
                onClick={async () => {
                  manageTag(record.key, 'enable');
                }}
              >
                {t('启用全部')}
              </Button>
              <Button
                theme='light'
                type='warning'
                style={{ marginRight: 1 }}
                onClick={async () => {
                  manageTag(record.key, 'disable');
                }}
              >
                {t('禁用全部')}
              </Button>
              <Button
                theme='light'
                type='tertiary'
                style={{ marginRight: 1 }}
                onClick={() => {
                  setShowEditTag(true);
                  setEditingTag(record.key);
                }}
              >
                {t('编辑')}
              </Button>
            </>
          );
        }
      },
    },
  ];

  // Filter columns based on visibility settings
  const getVisibleColumns = () => {
    return allColumns.filter((column) => visibleColumns[column.key]);
  };

  // Column selector modal
  const renderColumnSelector = () => {
    return (
      <Modal
        title={t('列设置')}
        visible={showColumnSelector}
        onCancel={() => setShowColumnSelector(false)}
        footer={
          <>
            <Button onClick={() => initDefaultColumns()}>{t('重置')}</Button>
            <Button onClick={() => setShowColumnSelector(false)}>
              {t('取消')}
            </Button>
            <Button type='primary' onClick={() => setShowColumnSelector(false)}>
              {t('确定')}
            </Button>
          </>
        }
        style={{ width: isMobile() ? '90%' : 500 }}
        bodyStyle={{ padding: '24px' }}
      >
        <div style={{ marginBottom: 20 }}>
          <Checkbox
            checked={Object.values(visibleColumns).every((v) => v === true)}
            indeterminate={
              Object.values(visibleColumns).some((v) => v === true) &&
              !Object.values(visibleColumns).every((v) => v === true)
            }
            onChange={(e) => handleSelectAll(e.target.checked)}
          >
            {t('全选')}
          </Checkbox>
        </div>
        <div
          style={{
            display: 'flex',
            flexWrap: 'wrap',
            maxHeight: '400px',
            overflowY: 'auto',
            border: '1px solid var(--semi-color-border)',
            borderRadius: '6px',
            padding: '16px',
          }}
        >
          {allColumns.map((column) => {
            // Skip columns without title
            if (!column.title) {
              return null;
            }

            return (
              <div
                key={column.key}
                style={{
                  width: isMobile() ? '100%' : '50%',
                  marginBottom: 16,
                  paddingRight: 8,
                }}
              >
                <Checkbox
                  checked={!!visibleColumns[column.key]}
                  onChange={(e) =>
                    handleColumnVisibilityChange(column.key, e.target.checked)
                  }
                >
                  {column.title}
                </Checkbox>
              </div>
            );
          })}
        </div>
      </Modal>
    );
  };

  const [channels, setChannels] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [idSort, setIdSort] = useState(false);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searchGroup, setSearchGroup] = useState('');
  const [searchModel, setSearchModel] = useState('');
  const [searching, setSearching] = useState(false);
  const [selectedTypes, setSelectedTypes] = useState([]); // 新增：类型多选
  const [selectedStatuses, setSelectedStatuses] = useState([]); // 新增：状态多选
  const [selectedTags, setSelectedTags] = useState([]); // 新增：标签多选
  const [tagOptions, setTagOptions] = useState([]); // 新增：标签选项

  const [updatingBalance, setUpdatingBalance] = useState(false);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [showPrompt, setShowPrompt] = useState(
    shouldShowPrompt('channel-test'),
  );
  const [channelCount, setChannelCount] = useState(pageSize);
  const [groupOptions, setGroupOptions] = useState([]);
  const [showEdit, setShowEdit] = useState(false);
  const [enableBatchDelete, setEnableBatchDelete] = useState(false);
  const [editingChannel, setEditingChannel] = useState({
    id: undefined,
  });
  const [showEditTag, setShowEditTag] = useState(false);
  const [editingTag, setEditingTag] = useState('');
  const [selectedChannels, setSelectedChannels] = useState([]);
  const [showEditPriority, setShowEditPriority] = useState(false);
  const [enableTagMode, setEnableTagMode] = useState(() => {
    const savedState = localStorage.getItem('channel-tag-mode');
    return savedState !== null ? JSON.parse(savedState) : false;
  });
  useEffect(() => {
    localStorage.setItem('channel-tag-mode', JSON.stringify(enableTagMode));
  }, [enableTagMode]);
  const [showBatchSetTag, setShowBatchSetTag] = useState(false);
  const [batchSetTagValue, setBatchSetTagValue] = useState('');
  const [showModelTestModal, setShowModelTestModal] = useState(false);
  const [currentTestChannel, setCurrentTestChannel] = useState(null);
  const [modelSearchKeyword, setModelSearchKeyword] = useState('');
  // 测试结果缓存
  const [testResultsCache, setTestResultsCache] = useState(() => Object.create(null));
  // 批量测试相关状态
  const [isBatchTesting, setIsBatchTesting] = useState(false);
  const [batchTestResults, setBatchTestResults] = useState([]);
  const [currentTestingModel, setCurrentTestingModel] = useState('');
  const [testProgress, setTestProgress] = useState({ completed: 0, total: 0 });
  const [concurrentLimit, setConcurrentLimit] = useState(3); // 默认并发数
  const [batchTestDelay, setBatchTestDelay] = useState(200); // 批次间延迟(ms)
  // 批量测试控制
  const [batchTestAbortController, setBatchTestAbortController] = useState(null);

  const removeRecord = (record) => {
    let newDataSource = [...channels];
    if (record.id != null) {
      let idx = newDataSource.findIndex((data) => {
        if (data.children !== undefined) {
          for (let i = 0; i < data.children.length; i++) {
            if (data.children[i].id === record.id) {
              data.children.splice(i, 1);
              return false;
            }
          }
        } else {
          return data.id === record.id;
        }
      });

      if (idx > -1) {
        newDataSource.splice(idx, 1);
        setChannels(newDataSource);
      }
    }
  };

  const setChannelFormat = (channels, enableTagMode) => {
    let channelDates = [];
    let channelTags = {};
    for (let i = 0; i < channels.length; i++) {
      // 为React Table设置key属性，同时保留原始的key字段（API密钥）
      channels[i].rowKey = '' + channels[i].id;
      if (!enableTagMode) {
        channelDates.push(channels[i]);
      } else {
        let tag = channels[i].tag ? channels[i].tag : '';
        // find from channelTags
        let tagIndex = channelTags[tag];
        let tagChannelDates = undefined;
        if (tagIndex === undefined) {
          // not found, create a new tag
          channelTags[tag] = 1;
          tagChannelDates = {
            rowKey: tag,  // React Table的key
            key: tag,     // 标签名
            id: tag,
            tag: tag,
            name: '标签：' + tag,
            group: '',
            used_quota: 0,
            response_time: 0,
            priority: -1,
            weight: -1,
          };
          tagChannelDates.children = [];
          channelDates.push(tagChannelDates);
        } else {
          // found, add to the tag
          tagChannelDates = channelDates.find((item) => item.key === tag);
        }
        if (tagChannelDates.priority === -1) {
          tagChannelDates.priority = channels[i].priority;
        } else {
          if (tagChannelDates.priority !== channels[i].priority) {
            tagChannelDates.priority = '';
          }
        }
        if (tagChannelDates.weight === -1) {
          tagChannelDates.weight = channels[i].weight;
        } else {
          if (tagChannelDates.weight !== channels[i].weight) {
            tagChannelDates.weight = '';
          }
        }

        if (tagChannelDates.group === '') {
          tagChannelDates.group = channels[i].group;
        } else {
          let channelGroupsStr = channels[i].group;
          channelGroupsStr.split(',').forEach((item, index) => {
            if (tagChannelDates.group.indexOf(item) === -1) {
              // join
              tagChannelDates.group += ',' + item;
            }
          });
        }

        tagChannelDates.children.push(channels[i]);
        if (channels[i].status === 1) {
          tagChannelDates.status = 1;
        }
        tagChannelDates.used_quota += channels[i].used_quota;
        tagChannelDates.response_time += channels[i].response_time;
        tagChannelDates.response_time = tagChannelDates.response_time / 2;
      }
    }
    // 为React Table设置rowKey属性
    setChannels(channelDates);
    if (channelDates.length >= pageSize) {
      setChannelCount(channelDates.length + pageSize);
    } else {
      setChannelCount(channelDates.length);
    }
  };

  const loadChannels = async (startIdx, pageSize, idSort, enableTagMode) => {
    setLoading(true);
    const res = await API.get(
      `/api/channel/?p=${startIdx}&page_size=${pageSize}&id_sort=${idSort}&tag_mode=${enableTagMode}`,
    );
    if (res === undefined) {
      return;
    }
    const { success, message, data } = res.data;
    if (success) {
      if (startIdx === 0) {
        setChannelFormat(data, enableTagMode);
      } else {
        let newChannels = [...channels];
        newChannels.splice(startIdx * pageSize, data.length, ...data);
        setChannelFormat(newChannels, enableTagMode);
      }
    } else {
      showError(message);
    }
    setLoading(false);
  };

  const copySelectedChannel = async (record) => {
    const channelToCopy = record;
    channelToCopy.name += t('_复制');
    channelToCopy.created_time = null;
    channelToCopy.balance = 0;
    channelToCopy.used_quota = 0;
    if (!channelToCopy) {
      showError(t('渠道未找到，请刷新页面后重试。'));
      return;
    }
    try {
      const newChannel = { ...channelToCopy, id: undefined };
      const response = await API.post('/api/channel/', newChannel);
      if (response.data.success) {
        showSuccess(t('渠道复制成功'));
        await refresh();
      } else {
        showError(response.data.message);
      }
    } catch (error) {
      showError(t('渠道复制失败: ') + error.message);
    }
  };

   const refresh = async () => {
    if (searchKeyword === '' && searchGroup === '' && searchModel === '') {
      await loadChannels(activePage - 1, pageSize, idSort, enableTagMode);
    } else {
      await searchChannels(
        searchKeyword,
        searchGroup,
        searchModel,
        enableTagMode,
      );
    }
  };

   useEffect(() => {
    // console.log('default effect')
    const localIdSort = localStorage.getItem('id-sort') === 'true';
    const localPageSize =
      parseInt(localStorage.getItem('page-size')) || ITEMS_PER_PAGE;
    setIdSort(localIdSort);
    setPageSize(localPageSize);
    loadChannels(0, localPageSize, localIdSort, enableTagMode)
      .then()
      .catch((reason) => {
        showError(reason);
      });
    fetchGroups().then();
    loadChannelModels().then();
    // 加载测试结果缓存
    loadTestResultsCache();
  }, []);

  // 监听模态框可见性变化，加载缓存的测试结果
  useEffect(() => {
    if (showModelTestModal && currentTestChannel) {
      const cachedResults = getCachedTestResults(currentTestChannel.id);
      if (cachedResults) {
        setBatchTestResults(cachedResults);
        setTestProgress({ 
          completed: cachedResults.length, 
          total: currentTestChannel.models?.split(',').filter(m => m.trim()).length || 0 
        });
      }
    }
  }, [showModelTestModal, currentTestChannel]);

  // 加载测试结果缓存
  const loadTestResultsCache = () => {
    try {
      const cached = localStorage.getItem('channel-test-results-cache');
      if (cached) {
        const parsedCache = JSON.parse(cached);
        // 清理过期的缓存（超过24小时）
        const now = Date.now();
        const validCache = Object.create(null);
        
        Object.keys(parsedCache).forEach(channelId => {
          // 验证channelId是安全的字符串，防止原型污染
          if (channelId === '__proto__' || 
              channelId === 'constructor' || 
              channelId === 'prototype') {
            return;
          }
          
          const channelCache = parsedCache[channelId];
          // 验证channelCache是对象且timestamp是有限数字
          if (channelCache && 
              typeof channelCache === 'object' &&
              channelCache !== null &&
              typeof channelCache.timestamp === 'number' &&
              Number.isFinite(channelCache.timestamp) &&
              (now - channelCache.timestamp) < 24 * 60 * 60 * 1000) {
            validCache[channelId] = channelCache;
          }
        });
        
        setTestResultsCache(validCache);
        // 更新localStorage中的有效缓存
        if (Object.keys(validCache).length !== Object.keys(parsedCache).length) {
          localStorage.setItem('channel-test-results-cache', JSON.stringify(validCache));
        }
      }
    } catch (e) {
      console.error('加载测试结果缓存失败:', e);
      localStorage.removeItem('channel-test-results-cache');
    }
  };

  // 保存测试结果缓存
  const saveTestResultsCache = (channelId, results) => {
    try {
      // 将channelId转换为字符串并验证安全性，防止原型污染
      const id = String(channelId);
      if (id === '__proto__' || 
          id === 'constructor' || 
          id === 'prototype') {
        console.warn('无效的channelId，跳过缓存保存:', channelId);
        return;
      }
      
      const newCache = Object.assign(Object.create(null), testResultsCache, {
        [id]: {
          results: results,
          timestamp: Date.now()
        }
      });
      setTestResultsCache(newCache);
      localStorage.setItem('channel-test-results-cache', JSON.stringify(newCache));
    } catch (e) {
      console.error('保存测试结果缓存失败:', e);
    }
  };

  // 获取缓存的测试结果
  const getCachedTestResults = (channelId) => {
    // 将channelId转换为字符串并验证安全性，防止原型污染
    const id = String(channelId);
    if (id === '__proto__' || 
        id === 'constructor' || 
        id === 'prototype') {
      return null;
    }
    
    const channelCache = testResultsCache[id];
    if (channelCache && (Date.now() - channelCache.timestamp) < 24 * 60 * 60 * 1000) {
      return channelCache.results;
    }
    return null;
  };

  const manageChannel = async (id, action, record, value) => {
    let data = { id };
    let res;
    switch (action) {
      case 'delete':
        res = await API.delete(`/api/channel/${id}/`);
        break;
      case 'enable':
        data.status = 1;
        res = await API.put('/api/channel/', data);
        break;
      case 'disable':
        data.status = 2;
        res = await API.put('/api/channel/', data);
        break;
      case 'priority':
        if (value === '') {
          return;
        }
        data.priority = parseInt(value);
        res = await API.put('/api/channel/', data);
        break;
      case 'weight':
        if (value === '') {
          return;
        }
        data.weight = parseInt(value);
        if (data.weight < 0) {
          data.weight = 0;
        }
        res = await API.put('/api/channel/', data);
        break;
    }
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('操作成功完成！'));
      let channel = res.data.data;
      let newChannels = [...channels];
      if (action === 'delete') {
      } else {
        record.status = channel.status;
      }
      setChannels(newChannels);
    } else {
      showError(message);
    }
  };

  const manageTag = async (tag, action) => {
    console.log(tag, action);
    let res;
    switch (action) {
      case 'enable':
        res = await API.post('/api/channel/tag/enabled', {
          tag: tag,
        });
        break;
      case 'disable':
        res = await API.post('/api/channel/tag/disabled', {
          tag: tag,
        });
        break;
    }
    const { success, message } = res.data;
    if (success) {
      showSuccess('操作成功完成！');
      let newChannels = [...channels];
      for (let i = 0; i < newChannels.length; i++) {
        if (newChannels[i].tag === tag) {
          let status = action === 'enable' ? 1 : 2;
          newChannels[i]?.children?.forEach((channel) => {
            channel.status = status;
          });
          newChannels[i].status = status;
        }
      }
      setChannels(newChannels);
    } else {
      showError(message);
    }
  };

  // 支持传入覆盖的类型/状态/标签参数，避免因setState异步导致筛选失效
  const searchChannels = async (
    searchKeyword,
    searchGroup,
    searchModel,
    enableTagMode,
    typesOverride = null,
    statusesOverride = null,
    tagsOverride = null,
  ) => {
    const nextTypes = typesOverride !== null ? typesOverride : selectedTypes;
    const nextStatuses = statusesOverride !== null ? statusesOverride : selectedStatuses;
    const nextTags = tagsOverride !== null ? tagsOverride : selectedTags;

    const noBasic = searchKeyword === '' && searchGroup === '' && searchModel === '';
    const noAdvanced = (!nextTypes || nextTypes.length === 0) && (!nextStatuses || nextStatuses.length === 0) && (!nextTags || nextTags.length === 0);
    if (noBasic && noAdvanced) {
      await loadChannels(activePage - 1, pageSize, idSort, enableTagMode);
      return;
    }
    setSearching(true);
    const params = new URLSearchParams();
    
    // 添加基础搜索参数
    params.append('keyword', searchKeyword);
    params.append('group', searchGroup);
    params.append('model', searchModel);
    params.append('id_sort', String(idSort));
    params.append('tag_mode', String(enableTagMode));
    
    // 条件添加数组参数
    if (nextTypes && nextTypes.length > 0) {
      params.append('types', nextTypes.map(Number).join(','));
    }
    if (nextStatuses && nextStatuses.length > 0) {
      params.append('statuses', nextStatuses.map(Number).join(','));
    }
    if (nextTags && nextTags.length > 0) {
      params.append('tags', nextTags.join(','));
    }
    
    const res = await API.get(`/api/channel/search?${params.toString()}`);
    const { success, message, data } = res.data;
    if (success) {
      setChannelFormat(data, enableTagMode);
      setActivePage(1);
    } else {
      showError(message);
    }
    setSearching(false);
  };

  const updateChannelProperty = (channelId, updateFn) => {
    // Create a new copy of channels array
    const newChannels = [...channels];
    let updated = false;

    // Find and update the correct channel
    newChannels.forEach((channel) => {
      if (channel.children !== undefined) {
        // If this is a tag group, search in its children
        channel.children.forEach((child) => {
          if (child.id === channelId) {
            updateFn(child);
            updated = true;
          }
        });
      } else if (channel.id === channelId) {
        // Direct channel match
        updateFn(channel);
        updated = true;
      }
    });

    // Only update state if we actually modified a channel
    if (updated) {
      setChannels(newChannels);
    }
  };

  const testChannel = async (record, model) => {
    const res = await API.get(`/api/channel/test/${record.id}?model=${model}`);
    const { success, message, time } = res.data;
    if (success) {
      // Also update the channels state to persist the change
      updateChannelProperty(record.id, (channel) => {
        channel.response_time = time * 1000;
        channel.test_time = Date.now() / 1000;
      });

      showInfo(
        t('通道 ${name} 测试成功，耗时 ${time.toFixed(2)} 秒。')
          .replace('${name}', record.name)
          .replace('${time.toFixed(2)}', time.toFixed(2)),
      );
    } else {
      showError(message);
    }
  };

  // 统一的取消检测函数
  const isCancellationError = (error) => {
    return (
      error.name === 'AbortError' ||
      error.name === 'CanceledError' ||
      axios.isCancel(error) ||
      error.code === 'ERR_CANCELED'
    );
  };

  // 单个模型测试函数（用于并发）- 支持重试
  const testSingleModel = async (model, abortController, maxRetries = 2) => {
    let lastError = null;
    
    for (let retryCount = 0; retryCount <= maxRetries; retryCount++) {
      try {
        const res = await API.get(`/api/channel/test/${currentTestChannel.id}?model=${encodeURIComponent(model)}`, {
          signal: abortController?.signal
        });
        const { success, message, time } = res.data;

        const result = {
          model,
          success,
          message: success ? `测试成功，耗时 ${time.toFixed(2)} 秒` : message,
          time: success ? time : null,
          retryCount,
          isRetryable: !success && retryCount < maxRetries
        };

        if (success) {
          // 更新渠道状态
          updateChannelProperty(currentTestChannel.id, (channel) => {
            channel.response_time = time * 1000;
            channel.test_time = Date.now() / 1000;
          });
        }

        return result;
      } catch (error) {
        lastError = error;
        
        // 如果是取消请求，立即抛出错误以停止执行
        if (isCancellationError(error) || abortController?.signal?.aborted) {
          throw error;
        }

        // 如果达到最大重试次数，返回失败结果
        if (retryCount >= maxRetries) {
          return {
            model,
            success: false,
            message: error.message || '测试失败',
            time: null,
            retryCount,
            isRetryable: true, // 仍然可以手动重试
            error: error
          };
        }

        // 重试前等待一段时间（指数退避）
        const delay = Math.min(1000 * Math.pow(2, retryCount), 5000);
        await new Promise(resolve => setTimeout(resolve, delay));
      }
    }
  };

  // 分批并发处理函数
  const processBatch = async (models, batchSize, abortController) => {
    const results = [];

    for (let i = 0; i < models.length; i += batchSize) {
      // 4) 每次迭代都检查取消状态
      if (abortController?.signal?.aborted) {
        throw new Error('测试被用户取消');
      }

      const batch = models.slice(i, i + batchSize);
      setCurrentTestingModel(`批次 ${Math.floor(i / batchSize) + 1}: ${batch.join(', ')}`);

      // 4) 在await之前再次检查取消状态
      if (abortController?.signal?.aborted) {
        throw new Error('测试被用户取消');
      }

      // 并发执行当前批次
      const batchPromises = batch.map(model => testSingleModel(model, abortController));
      const batchResults = await Promise.allSettled(batchPromises);

      // 4) 在await之后检查取消状态
      if (abortController?.signal?.aborted) {
        throw new Error('测试被用户取消');
      }

      // 处理批次结果
      const processedResults = batchResults.map((result, index) => {
        if (result.status === 'fulfilled') {
          return result.value;
        } else {
          // 如果是取消错误，直接抛出
          if (isCancellationError(result.reason) || abortController?.signal?.aborted) {
            throw result.reason;
          }
          return {
            model: batch[index],
            success: false,
            message: result.reason?.message || '测试失败',
            time: null
          };
        }
      });

      results.push(...processedResults);

      // 更新进度和结果
      setTestProgress({ completed: results.length, total: models.length });
      setBatchTestResults([...results]);

      // 3) 批次间使用可配置的延迟
      if (i + batchSize < models.length) {
        // 4) 在延迟期间也要检查取消状态
        if (abortController?.signal?.aborted) {
          throw new Error('测试被用户取消');
        }
        await new Promise(resolve => setTimeout(resolve, batchTestDelay));
      }
    }

    return results;
  };

  // 重试失败的模型
  const retryFailedModels = async () => {
    if (!currentTestChannel || isBatchTesting) {
      return;
    }

    const failedModels = batchTestResults.filter(r => !r.success && r.isRetryable);
    if (failedModels.length === 0) {
      showInfo(t('没有可重试的失败模型'));
      return;
    }

    const controller = new AbortController();
    setBatchTestAbortController(controller);
    setIsBatchTesting(true);
    setCurrentTestingModel('重试失败模型...');

    try {
      const failedModelNames = failedModels.map(r => r.model);
      const retryResults = await processBatch(failedModelNames, concurrentLimit, controller);
      
      // 更新批量测试结果，合并重试结果
      const updatedResults = batchTestResults.map(result => {
        const retryResult = retryResults.find(r => r.model === result.model);
        return retryResult || result;
      });
      
      setBatchTestResults(updatedResults);
      
      const newSuccessCount = retryResults.filter(r => r.success).length;
      showInfo(t(`重试完成：成功 ${newSuccessCount} 个，失败 ${failedModelNames.length - newSuccessCount} 个`));
      
    } catch (error) {
      if (isCancellationError(error)) {
        showInfo(t('重试已取消'));
      } else {
        showError(t('重试过程中发生错误：') + error.message);
      }
    } finally {
      setIsBatchTesting(false);
      setCurrentTestingModel('');
      setBatchTestAbortController(null);
    }
  };

  // 批量测试所有模型（并发版本）
  const testAllModels = async () => {
    // 1) 验证currentTestChannel存在且具有期望的属性
    if (!currentTestChannel || 
        typeof currentTestChannel !== 'object' ||
        !currentTestChannel.id ||
        !currentTestChannel.models ||
        typeof currentTestChannel.models !== 'string') {
      showError(t('当前测试渠道无效或缺少必需属性'));
      return;
    }

    // 修剪空白、去重并过滤空字符串
    const models = [...new Set(
      currentTestChannel.models
        .split(',')
        .map(model => model.trim())
        .filter(model => model.length > 0)
    )];
    if (models.length === 0) {
      showError(t('没有可测试的模型'));
      return;
    }

    // 创建 AbortController
    const controller = new AbortController();
    setBatchTestAbortController(controller);

    setIsBatchTesting(true);
    setBatchTestResults([]);
    setCurrentTestingModel('');
    setTestProgress({ completed: 0, total: models.length });

    // 2) 使用try-catch-finally包装整个异步测试逻辑
    try {
      const results = await processBatch(models, concurrentLimit, controller);

      const successCount = results.filter(r => r.success).length;
      const failCount = results.filter(r => !r.success).length;

      // 保存测试结果到缓存
      saveTestResultsCache(currentTestChannel.id, results);

      showInfo(t(`批量测试完成：成功 ${successCount} 个，失败 ${failCount} 个`));
    } catch (error) {
      if (isCancellationError(error) || error.message.includes('取消')) {
        showInfo(t('测试已取消'));
      } else {
        showError(t('批量测试过程中发生错误：') + error.message);
      }
    } finally {
      // 3) 确保在finally中重置状态
      setIsBatchTesting(false);
      setCurrentTestingModel('');
      setBatchTestAbortController(null);
    }
  };

  const updateChannelBalance = async (record) => {
    const res = await API.get(`/api/channel/update_balance/${record.id}/`);
    const { success, message, balance } = res.data;
    if (success) {
      updateChannelProperty(record.id, (channel) => {
        channel.balance = balance;
        channel.balance_updated_time = Date.now() / 1000;
      });
      showInfo(
        t('通道 ${name} 余额更新成功！').replace('${name}', record.name),
      );
    } else {
      showError(message);
    }
  };

  const testAllChannels = async () => {
    const res = await API.get(`/api/channel/test`);
    const { success, message } = res.data;
    if (success) {
      showInfo(t('已成功开始测试所有已启用通道，请刷新页面查看结果。'));
    } else {
      showError(message);
    }
  };

  const deleteAllDisabledChannels = async () => {
    const res = await API.delete(`/api/channel/disabled`);
    const { success, message, data } = res.data;
    if (success) {
      showSuccess(
        t('已删除所有禁用渠道，共计 ${data} 个').replace('${data}', data),
      );
      await refresh();
    } else {
      showError(message);
    }
  };

  const updateAllChannelsBalance = async () => {
    setUpdatingBalance(true);
    const res = await API.get(`/api/channel/update_balance`);
    const { success, message } = res.data;
    if (success) {
      showInfo(t('已更新完毕所有已启用通道余额！'));
    } else {
      showError(message);
    }
    setUpdatingBalance(false);
  };

  const batchDeleteChannels = async () => {
    if (selectedChannels.length === 0) {
      showError(t('请先选择要删除的通道！'));
      return;
    }
    setLoading(true);
    let ids = [];
    selectedChannels.forEach((channel) => {
      ids.push(channel.id);
    });
    const res = await API.post(`/api/channel/batch`, { ids: ids });
    const { success, message, data } = res.data;
    if (success) {
      showSuccess(t('已删除 ${data} 个通道！').replace('${data}', data));
      await refresh();
    } else {
      showError(message);
    }
    setLoading(false);
  };

  const fixChannelsAbilities = async () => {
    const res = await API.post(`/api/channel/fix`);
    const { success, message, data } = res.data;
    if (success) {
      showSuccess(t('已修复 ${data} 个通道！').replace('${data}', data));
      await refresh();
    } else {
      showError(message);
    }
  };

  let pageData = channels.slice(
    (activePage - 1) * pageSize,
    activePage * pageSize,
  );

  const handlePageChange = (page) => {
    setActivePage(page);
    if (page === Math.ceil(channels.length / pageSize) + 1) {
      // In this case we have to load more data and then append them.
      loadChannels(page - 1, pageSize, idSort, enableTagMode).then((r) => {});
    }
  };

  const handlePageSizeChange = async (size) => {
    localStorage.setItem('page-size', size + '');
    setPageSize(size);
    setActivePage(1);
    loadChannels(0, size, idSort, enableTagMode)
      .then()
      .catch((reason) => {
        showError(reason);
      });
  };

  const fetchGroups = async () => {
    try {
      let res = await API.get(`/api/group/`);
      // add 'all' option
      // res.data.data.unshift('all');
      if (res === undefined) {
        return;
      }
      setGroupOptions(
        res.data.data.map((group) => ({
          label: group,
          value: group,
        })),
      );
    } catch (error) {
      showError(error.message);
    }
  };

  const submitTagEdit = async (type, data) => {
    switch (type) {
      case 'priority':
        if (data.priority === undefined || data.priority === '') {
          showInfo('优先级必须是整数！');
          return;
        }
        data.priority = parseInt(data.priority);
        break;
      case 'weight':
        if (
          data.weight === undefined ||
          data.weight < 0 ||
          data.weight === ''
        ) {
          showInfo('权重必须是非负整数！');
          return;
        }
        data.weight = parseInt(data.weight);
        break;
    }

    try {
      const res = await API.put('/api/channel/tag', data);
      if (res?.data?.success) {
        showSuccess('更新成功！');
        await refresh();
      }
    } catch (error) {
      showError(error);
    }
  };

  const closeEdit = () => {
    setShowEdit(false);
  };

  const handleRow = (record, index) => {
    if (record.status !== 1) {
      return {
        style: {
          background: 'var(--semi-color-disabled-border)',
        },
      };
    } else {
      return {};
    }
  };

  const batchSetChannelTag = async () => {
    if (selectedChannels.length === 0) {
      showError(t('请先选择要设置标签的渠道！'));
      return;
    }
    if (batchSetTagValue === '') {
      showError(t('标签不能为空！'));
      return;
    }
    let ids = selectedChannels.map((channel) => channel.id);
    const res = await API.post('/api/channel/batch/tag', {
      ids: ids,
      tag: batchSetTagValue === '' ? null : batchSetTagValue,
    });
    if (res.data.success) {
      showSuccess(
        t('已为 ${count} 个渠道设置标签！').replace('${count}', res.data.data),
      );
      await refresh();
      setShowBatchSetTag(false);
    } else {
      showError(res.data.message);
    }
  };

  return (
    <>
      {renderColumnSelector()}
      <EditTagModal
        visible={showEditTag}
        tag={editingTag}
        handleClose={() => setShowEditTag(false)}
        refresh={refresh}
      />
      <EditChannel
        refresh={refresh}
        visible={showEdit}
        handleClose={closeEdit}
        editingChannel={editingChannel}
      />
      <Form
        onSubmit={() => {
          searchChannels(
            searchKeyword,
            searchGroup,
            searchModel,
            enableTagMode,
          );
        }}
        labelPosition='left'
      >
        <div style={{ display: 'flex' }}>
          <Space>
            <Form.Input
              field='search_keyword'
              label={t('搜索渠道关键词')}
              placeholder={t('搜索渠道的 ID，名称和密钥 ...')}
              value={searchKeyword}
              loading={searching}
              onChange={(v) => {
                setSearchKeyword(v.trim());
              }}
            />
            <Form.Input
              field='search_model'
              label={t('模型')}
              placeholder={t('模型关键字')}
              value={searchModel}
              loading={searching}
              onChange={(v) => {
                setSearchModel(v.trim());
              }}
            />
            <Form.Select
              field='group'
              label={t('分组')}
              optionList={[
                { label: t('选择分组'), value: null },
                ...groupOptions,
              ]}
              onChange={(v) => {
                setSearchGroup(v);
                searchChannels(searchKeyword, v, searchModel, enableTagMode);
              }}
            />
            {/* 新增：类型多选 */}
            <Form.Select
              field='types'
              label={t('渠道类型')}
              multiple
              optionList={CHANNEL_OPTIONS.map(o => ({ label: o.label, value: o.value }))}
              onChange={(v) => {
                const next = (v || []).map(Number);
                setSelectedTypes(next);
                searchChannels(searchKeyword, searchGroup, searchModel, enableTagMode, next, null, null);
              }}
            />
            {/* 新增：状态多选 */}
            <Form.Select
              field='statuses'
              label={t('渠道状态')}
              multiple
              optionList={[{label:t('已启用'), value:1}, {label:t('已禁用'), value:2}, {label:t('自动禁用'), value:3}]}
              onChange={(v) => {
                const next = (v || []).map(Number);
                setSelectedStatuses(next);
                searchChannels(searchKeyword, searchGroup, searchModel, enableTagMode, null, next, null);
              }}
            />
            {/* 新增：标签多选 */}
            <Form.Select
              field='tags'
              label={t('标签')}
              multiple
              optionList={tagOptions}
              filter
              onFocus={async ()=>{
                if (tagOptions.length===0) {
                  const res = await API.get('/api/channel/tags?offset=0&limit=200');
                  const opts = (res?.data?.data||[]).filter(Boolean).map(t=>({label:t, value:t}));
                  setTagOptions(opts);
                }
              }}
              onChange={(v) => {
                const next = (v || []);
                setSelectedTags(next);
                searchChannels(searchKeyword, searchGroup, searchModel, enableTagMode, null, null, next);
              }}
            />
            <Button
              theme='light'
              type='tertiary'
              onClick={() => {
                setSearchKeyword('');
                setSearchGroup('');
                setSearchModel('');
                setSelectedTypes([]);
                setSelectedStatuses([]);
                setSelectedTags([]);
                loadChannels(0, pageSize, idSort, enableTagMode);
              }}
            >{t('重置条件')}</Button>


            <Button
              label={t('查询')}
              type='primary'
              htmlType='submit'
              className='btn-margin-right'
              style={{ marginRight: 8 }}
            >
              {t('查询')}
            </Button>
          </Space>
        </div>
      </Form>
      <Divider style={{ marginBottom: 15 }} />
      <div
        style={{
          display: 'flex',
          flexDirection: isMobile() ? 'column' : 'row',
          marginTop: isMobile() ? 0 : -45,
          zIndex: 999,
          pointerEvents: 'none',
        }}
      >
        <Space
          style={{
            pointerEvents: 'auto',
            marginTop: isMobile() ? 0 : 45,
            marginBottom: isMobile() ? 16 : 0,
            display: 'flex',
            flexWrap: isMobile() ? 'wrap' : 'nowrap',
            gap: '8px',
          }}
        >
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              marginRight: 16,
              flexWrap: 'nowrap',
            }}
          >
            <Typography.Text strong style={{ marginRight: 8 }}>
              {t('使用ID排序')}
            </Typography.Text>
            <Switch
              checked={idSort}
              label={t('使用ID排序')}
              aria-label={t('是否用ID排序')}
              onChange={(v) => {
                localStorage.setItem('id-sort', v + '');
                setIdSort(v);
                loadChannels(0, pageSize, v, enableTagMode)
                  .then()
                  .catch((reason) => {
                    showError(reason);
                  });
              }}
            ></Switch>
          </div>

          <div
            style={{
              display: 'flex',
              flexWrap: 'wrap',
              gap: '8px',
            }}
          >
            <Button
              theme='light'
              type='primary'
              icon={<IconPlus />}
              onClick={() => {
                setEditingChannel({
                  id: undefined,
                });
                setShowEdit(true);
              }}
            >
              {t('添加渠道')}
            </Button>

            <Button
              theme='light'
              type='primary'
              icon={<IconRefresh />}
              onClick={refresh}
            >
              {t('刷新')}
            </Button>

            <Dropdown
              trigger='click'
              render={
                <Dropdown.Menu>
                  <Dropdown.Item>
                    <Popconfirm
                      title={t('确定？')}
                      okType={'warning'}
                      onConfirm={testAllChannels}
                      position={isMobile() ? 'top' : 'top'}
                    >
                      <Button
                        theme='light'
                        type='warning'
                        style={{ width: '100%' }}
                      >
                        {t('测试所有通道')}
                      </Button>
                    </Popconfirm>
                  </Dropdown.Item>
                  <Dropdown.Item>
                    <Popconfirm
                      title={t('确定？')}
                      okType={'secondary'}
                      onConfirm={updateAllChannelsBalance}
                    >
                      <Button
                        theme='light'
                        type='secondary'
                        style={{ width: '100%' }}
                      >
                        {t('更新所有已启用通道余额')}
                      </Button>
                    </Popconfirm>
                  </Dropdown.Item>
                  <Dropdown.Item>
                    <Popconfirm
                      title={t('确定是否要删除禁用通道？')}
                      content={t('此修改将不可逆')}
                      okType={'danger'}
                      onConfirm={deleteAllDisabledChannels}
                    >
                      <Button
                        theme='light'
                        type='danger'
                        style={{ width: '100%' }}
                      >
                        {t('删除禁用通道')}
                      </Button>
                    </Popconfirm>
                  </Dropdown.Item>
                </Dropdown.Menu>
              }
            >
              <Button theme='light' type='tertiary' icon={<IconSetting />}>
                {t('批量操作')}
              </Button>
            </Dropdown>
          </div>
        </Space>
      </div>
      <div
        style={{
          marginTop: 20,
          display: 'flex',
          flexDirection: isMobile() ? 'column' : 'row',
          alignItems: isMobile() ? 'flex-start' : 'center',
          gap: isMobile() ? '8px' : '16px',
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            marginBottom: isMobile() ? 8 : 0,
          }}
        >
          <Typography.Text strong style={{ marginRight: 8 }}>
            {t('开启批量操作')}
          </Typography.Text>
          <Switch
            label={t('开启批量操作')}
            aria-label={t('是否开启批量操作')}
            onChange={(v) => {
              setEnableBatchDelete(v);
            }}
          />
        </div>

        <div
          style={{
            display: 'flex',
            flexWrap: 'wrap',
            gap: '8px',
          }}
        >
          <Popconfirm
            title={t('确定是否要删除所选通道？')}
            content={t('此修改将不可逆')}
            okType={'danger'}
            onConfirm={batchDeleteChannels}
            disabled={!enableBatchDelete}
          >
            <Button disabled={!enableBatchDelete} theme='light' type='danger'>
              {t('删除所选通道')}
            </Button>
          </Popconfirm>
          <Popconfirm
            title={t('确定是否要修复数据库一致性？')}
            content={t(
              '进行该操作时，可能导致渠道访问错误，请仅在数据库出现问题时使用',
            )}
            okType={'warning'}
            onConfirm={fixChannelsAbilities}
          >
            <Button theme='light' type='secondary'>
              {t('修复数据库一致性')}
            </Button>
          </Popconfirm>
        </div>
      </div>

      <div
        style={{
          marginTop: 20,
          marginBottom: 20,
          display: 'flex',
          flexDirection: isMobile() ? 'column' : 'row',
          alignItems: isMobile() ? 'flex-start' : 'center',
          gap: isMobile() ? '8px' : '16px',
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            marginBottom: isMobile() ? 8 : 0,
          }}
        >
          <Typography.Text strong style={{ marginRight: 8 }}>
            {t('标签聚合模式')}
          </Typography.Text>
          <Switch
            checked={enableTagMode}
            label={t('标签聚合模式')}
            aria-label={t('是否启用标签聚合')}
            onChange={(v) => {
              setEnableTagMode(v);
              loadChannels(0, pageSize, idSort, v);
            }}
          />
        </div>

        <div
          style={{
            display: 'flex',
            flexWrap: 'wrap',
            gap: '8px',
          }}
        >
          <Button
            disabled={!enableBatchDelete}
            theme='light'
            type='primary'
            onClick={() => setShowBatchSetTag(true)}
          >
            {t('批量设置标签')}
          </Button>

          <Button
            theme='light'
            type='tertiary'
            icon={<IconSetting />}
            onClick={() => setShowColumnSelector(true)}
          >
            {t('列设置')}
          </Button>
        </div>
      </div>

      <Table
        loading={loading}
        columns={getVisibleColumns()}
        dataSource={pageData}
        rowKey="rowKey"
        pagination={{
          currentPage: activePage,
          pageSize: pageSize,
          total: channelCount,
          pageSizeOpts: [10, 20, 50, 100],
          showSizeChanger: true,
          formatPageText: (page) => '',
          onPageSizeChange: (size) => {
            handlePageSizeChange(size).then();
          },
          onPageChange: handlePageChange,
        }}
        expandAllRows={false}
        onRow={handleRow}
        rowSelection={
          enableBatchDelete
            ? {
                onChange: (selectedRowKeys, selectedRows) => {
                  // console.log(`selectedRowKeys: ${selectedRowKeys}`, 'selectedRows: ', selectedRows);
                  setSelectedChannels(selectedRows);
                },
              }
            : null
        }
      />
      <Modal
        title={t('批量设置标签')}
        visible={showBatchSetTag}
        onOk={batchSetChannelTag}
        onCancel={() => setShowBatchSetTag(false)}
        maskClosable={false}
        centered={true}
        style={{ width: isMobile() ? '90%' : 500 }}
      >
        <div style={{ marginBottom: 20 }}>
          <Typography.Text>{t('请输入要设置的标签名称')}</Typography.Text>
        </div>
        <Input
          placeholder={t('请输入标签名称')}
          value={batchSetTagValue}
          onChange={(v) => setBatchSetTagValue(v)}
          size='large'
        />
        <div style={{ marginTop: 16 }}>
          <Typography.Text type='secondary'>
            {t('已选择 ${count} 个渠道').replace(
              '${count}',
              selectedChannels.length,
            )}
          </Typography.Text>
        </div>
      </Modal>

      {/* 模型测试弹窗 */}
      <Modal
        title={t('选择模型进行测试')}
        visible={showModelTestModal && currentTestChannel !== null}
        onCancel={() => {
          // 停止正在进行的批量测试
          if (batchTestAbortController) {
            batchTestAbortController.abort();
            setBatchTestAbortController(null);
          }
          setShowModelTestModal(false);
          setModelSearchKeyword('');
          // 重置批量测试相关状态
          setIsBatchTesting(false);
          setBatchTestResults([]);
          setCurrentTestingModel('');
          setTestProgress({ completed: 0, total: 0 });
        }}
        footer={null}
        maskClosable={true}
        centered={true}
      >
        <div style={{ maxHeight: '500px', overflowY: 'auto', padding: '10px' }}>
          {currentTestChannel && (
            <div>
              <Typography.Title heading={6} style={{ marginBottom: '16px' }}>
                {t('渠道')}: {currentTestChannel.name}
              </Typography.Title>

              {/* 操作按钮区域 */}
              <div style={{ marginBottom: '16px' }}>
                {/* 第一行：主要操作按钮 */}
                <div style={{ 
                  display: 'flex', 
                  flexWrap: 'wrap',
                  gap: '8px', 
                  alignItems: 'center', 
                  marginBottom: '8px' 
                }}>
                  <Button
                    type='primary'
                    loading={isBatchTesting}
                    onClick={testAllModels}
                    disabled={!currentTestChannel?.models}
                  >
                    {isBatchTesting ? t('测试中...') : t('测试全部')}
                  </Button>

                  {/* 并发数设置 */}
                  {!isBatchTesting && (
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <Typography.Text type='secondary'>{t('并发数')}:</Typography.Text>
                      <InputNumber
                        value={concurrentLimit}
                        onChange={setConcurrentLimit}
                        min={1}
                        max={10}
                        style={{ width: '80px' }}
                        size='small'
                      />
                    </div>
                  )}
                </div>

                {/* 第二行：失败模型处理按钮 */}
                {batchTestResults.length > 0 && batchTestResults.filter(r => !r.success).length > 0 && (
                  <div style={{ 
                    display: 'flex', 
                    flexWrap: 'wrap',
                    gap: '8px', 
                    alignItems: 'center'
                  }}>
                    {/* 重试失败模型按钮 */}
                    {batchTestResults.filter(r => !r.success && r.isRetryable).length > 0 && (
                      <Button
                        type='secondary'
                        loading={isBatchTesting}
                        onClick={retryFailedModels}
                        disabled={isBatchTesting}
                        style={{ 
                          backgroundColor: '#ffa940', 
                          borderColor: '#ffa940', 
                          color: '#fff',
                          minWidth: 'fit-content'
                        }}
                      >
                        {isBatchTesting ? t('重试中...') : t(`重试失败模型 (${batchTestResults.filter(r => !r.success && r.isRetryable).length})`)}
                      </Button>
                    )}

                    <Button
                      type='danger'
                      style={{ minWidth: 'fit-content' }}
                      onClick={() => {
                        const failedModels = batchTestResults.filter(r => !r.success);
                        const currentModels = currentTestChannel.models.split(',').map(m => m.trim());
                        const failedModelNames = failedModels.map(r => r.model);
                        const remainingModels = currentModels.filter(m => !failedModelNames.includes(m));
                        
                        // 检查是否会删除所有模型
                        if (remainingModels.length === 0) {
                          Modal.warning({
                            title: t('无法删除所有模型'),
                            content: t('删除这些失败的模型会导致渠道没有任何可用模型，这将使渠道无法正常工作。请至少保留一个模型或先添加新的可用模型。'),
                          });
                          return;
                        }
                        
                        Modal.confirm({
                          title: t('批量删除失败模型'),
                          content: (
                            <div>
                              <p>{t('确定要删除所有 ${count} 个测试失败的模型吗？此操作不可撤销。').replace('${count}', failedModels.length)}</p>
                              <p style={{ color: '#52c41a', marginTop: '8px' }}>
                                {t('删除后将保留 ${remaining} 个正常模型：').replace('${remaining}', remainingModels.length)}
                              </p>
                              <div style={{ 
                                maxHeight: '100px', 
                                overflowY: 'auto', 
                                background: '#f5f5f5', 
                                padding: '8px', 
                                borderRadius: '4px',
                                marginTop: '4px'
                              }}>
                                {remainingModels.map((model, index) => (
                                  <div key={index} style={{ fontSize: '12px' }}>{model}</div>
                                ))}
                              </div>
                            </div>
                          ),
                          onOk: async () => {
                            try {
                              const newModels = remainingModels;

                              // 构建正确的更新数据格式，参考EditChannel.js的实现
                              const updateData = {
                                id: currentTestChannel.id,
                                type: currentTestChannel.type,
                                name: currentTestChannel.name,
                                key: currentTestChannel.key,
                                base_url: currentTestChannel.base_url || '',
                                other: currentTestChannel.other || '',
                                models: newModels.join(','), // 更新后的模型列表，转换为逗号分隔的字符串
                                group: currentTestChannel.group || 'default',
                                model_mapping: currentTestChannel.model_mapping || '',
                                status: currentTestChannel.status,
                                priority: currentTestChannel.priority ?? 0,
                                weight: currentTestChannel.weight ?? 0,
                                auto_ban: currentTestChannel.auto_ban ?? 1, // 确保是数字类型
                                test_model: currentTestChannel.test_model || '',
                                openai_organization: currentTestChannel.openai_organization || '',
                                status_code_mapping: currentTestChannel.status_code_mapping || '',
                                tag: currentTestChannel.tag || '',
                                setting: currentTestChannel.setting || '',
                                param_override: currentTestChannel.param_override || '',
                                model_prefix: currentTestChannel.model_prefix || '',
                                system_prompt: currentTestChannel.system_prompt || ''
                              };

                              const res = await API.put('/api/channel/', updateData);
                              if (res.data.success) {
                                // 更新本地状态
                                updateChannelProperty(currentTestChannel.id, (channel) => {
                                  channel.models = newModels.join(',');
                                });
                                setCurrentTestChannel({
                                  ...currentTestChannel,
                                  models: newModels.join(',')
                                });

                                // 从测试结果中移除被删除的失败模型
                                setBatchTestResults(prev => prev.filter(r => !failedModelNames.includes(r.model)));

                                // 刷新数据以确保与服务器同步
                                await refresh();

                                showSuccess(t('已删除 ${count} 个失败的模型').replace('${count}', failedModels.length));
                              } else {
                                showError(res.data.message || t('批量删除失败'));
                              }
                            } catch (error) {
                              showError(error.message || t('批量删除失败'));
                            }
                          }
                        });
                      }}
                    >
                      {t('删除失败模型')} ({batchTestResults.filter(r => !r.success).length})
                    </Button>
                  </div>
                )}

                {/* 测试进度显示 */}
                {isBatchTesting && (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <Typography.Text type='secondary' size='small'>
                        {t('进度')}: {testProgress.completed}/{testProgress.total}
                      </Typography.Text>
                      <div style={{
                        flex: 1,
                        height: '4px',
                        backgroundColor: '#f0f0f0',
                        borderRadius: '2px',
                        overflow: 'hidden'
                      }}>
                        <div style={{
                          width: `${testProgress.total > 0 ? (testProgress.completed / testProgress.total) * 100 : 0}%`,
                          height: '100%',
                          backgroundColor: '#1890ff',
                          transition: 'width 0.3s ease'
                        }} />
                      </div>
                    </div>
                    {currentTestingModel && (
                      <Typography.Text type='secondary' size='small'>
                        {t('正在测试')}: {currentTestingModel}
                      </Typography.Text>
                    )}
                  </div>
                )}
              </div>

              {/* 搜索框 */}
              <Input
                placeholder={t('搜索模型...')}
                value={modelSearchKeyword}
                onChange={(v) => setModelSearchKeyword(v)}
                style={{ marginBottom: '16px' }}
                prefix={<IconFilter />}
                showClear
              />

              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))',
                  gap: '12px',
                  marginBottom: '16px',
                }}
              >
                {currentTestChannel.models
                  .split(',')
                  .filter((model) =>
                    model
                      .toLowerCase()
                      .includes(modelSearchKeyword.toLowerCase()),
                  )
                  .map((model, index) => {
                    // 获取该模型的测试结果
                    const testResult = batchTestResults.find(r => r.model === model.trim());
                    const isSuccess = testResult?.success;
                    const isFailed = testResult && !testResult.success;
                    const isTesting = isBatchTesting && currentTestingModel.includes(model.trim());
                    const hasRetries = testResult?.retryCount > 0;
                    const isRetryable = testResult?.isRetryable;

                    // 构造状态描述
                    let statusDesc = '';
                    if (hasRetries && isSuccess) {
                      statusDesc = `（重试${testResult.retryCount}次后成功）`;
                    } else if (hasRetries && isFailed) {
                      statusDesc = `（已重试${testResult.retryCount}次）`;
                    }

                    return (
                      <div key={index} style={{
                        border: testResult ? (isSuccess ? '2px solid #00b42a' : '2px solid #f53f3f') : '1px solid #e0e0e0',
                        borderRadius: '8px',
                        backgroundColor: testResult ? (isSuccess ? '#f6ffed' : '#fef2f2') : 'transparent',
                        padding: '8px',
                        position: 'relative'
                      }}>
                        <Space spacing={4} style={{ alignItems: 'stretch', width: '100%' }}>
                          <Button
                            theme='light'
                            type='tertiary'
                            loading={isTesting}
                            style={{
                              height: '100%',
                              padding: '10px 12px',
                              textAlign: 'center',
                              whiteSpace: 'nowrap',
                              overflow: 'hidden',
                              textOverflow: 'ellipsis',
                              width: '100%',
                              borderRadius: '6px',
                              backgroundColor: testResult ? (isSuccess ? '#f6ffed' : '#fef2f2') : 'transparent',
                              color: testResult ? (isSuccess ? '#00b42a' : '#f53f3f') : 'inherit',
                              border: 'none'
                            }}
                            onClick={() => {
                              testChannel(currentTestChannel, model);
                            }}
                          >
                            <div style={{ lineHeight: '1.2' }}>
                              <div>{model}</div>
                              {statusDesc && (
                                <div style={{ 
                                  fontSize: '10px', 
                                  opacity: 0.8,
                                  marginTop: '2px' 
                                }}>
                                  {statusDesc}
                                </div>
                              )}
                            </div>
                          </Button>
                          <Button
                            theme='light'
                            type='tertiary'
                            style={{
                              height: '100%',
                              padding: '10px 12px',
                              borderRadius: '6px',
                            }}
                            icon={<IconCopy />}
                            onClick={() => {
                              navigator.clipboard
                                .writeText(model)
                                .then(() => showSuccess(t('模型名称已复制')))
                                .catch(() => showError(t('复制失败')));
                            }}
                          />
                          {/* 单个模型重试按钮（仅在测试失败且可重试时显示） */}
                          {isFailed && isRetryable && (
                            <Button
                              type='secondary'
                              size='small'
                              loading={isTesting}
                              style={{
                                height: '100%',
                                padding: '10px 8px',
                                borderRadius: '6px',
                                backgroundColor: '#ffa940',
                                borderColor: '#ffa940',
                                color: '#fff'
                              }}
                              icon={<IconRefresh />}
                              onClick={async () => {
                                try {
                                  setCurrentTestingModel(`重试 ${model.trim()}...`);
                                  const controller = new AbortController();
                                  const result = await testSingleModel(model.trim(), controller, 1);
                                  
                                  // 更新单个模型的测试结果
                                  setBatchTestResults(prev => 
                                    prev.map(r => r.model === model.trim() ? result : r)
                                  );
                                  
                                  if (result.success) {
                                    showSuccess(t(`模型 ${model.trim()} 重试成功`));
                                  } else {
                                    showError(t(`模型 ${model.trim()} 重试失败：${result.message}`));
                                  }
                                } catch (error) {
                                  showError(t(`重试失败：${error.message}`));
                                } finally {
                                  setCurrentTestingModel('');
                                }
                              }}
                            />
                          )}
                          {/* 单个模型删除按钮（仅在测试失败时显示） */}
                          {isFailed && (
                            <Button
                              type='danger'
                              size='small'
                              style={{
                                height: '100%',
                                padding: '10px 8px',
                                borderRadius: '6px',
                              }}
                              icon={<IconClose />}
                              onClick={() => {
                                // 检查删除后是否还有模型剩余
                                const currentModels = currentTestChannel.models.split(',').map(m => m.trim());
                                const newModels = currentModels.filter(m => m !== model.trim());
                                
                                if (newModels.length === 0) {
                                  Modal.warning({
                                    title: t('无法删除模型'),
                                    content: t('删除模型 "${model}" 会导致渠道没有任何可用模型，这将使渠道无法正常工作。请先添加其他可用模型。').replace('${model}', model),
                                  });
                                  return;
                                }
                                
                                Modal.confirm({
                                  title: t('确认删除模型'),
                                  content: (
                                    <div>
                                      <p>{t('确定要从渠道中删除模型 "${model}" 吗？此操作不可撤销。').replace('${model}', model)}</p>
                                      <p style={{ color: '#52c41a', marginTop: '8px' }}>
                                        {t('删除后将剩余 ${remaining} 个模型').replace('${remaining}', newModels.length)}
                                      </p>
                                    </div>
                                  ),
                                  onOk: async () => {
                                    try {
                                      // 使用之前计算的newModels，避免重复计算

                                      // 构建正确的更新数据格式
                                      const updateData = {
                                        id: currentTestChannel.id,
                                        type: currentTestChannel.type,
                                        name: currentTestChannel.name,
                                        key: currentTestChannel.key,
                                        base_url: currentTestChannel.base_url || '',
                                        other: currentTestChannel.other || '',
                                        models: newModels.join(','), // 更新后的模型列表，转换为逗号分隔的字符串
                                        group: currentTestChannel.group || 'default',
                                        model_mapping: currentTestChannel.model_mapping || '',
                                        status: currentTestChannel.status,
                                        priority: currentTestChannel.priority ?? 0,
                                        weight: currentTestChannel.weight ?? 0,
                                        auto_ban: currentTestChannel.auto_ban ?? 1, // 确保是数字类型
                                        test_model: currentTestChannel.test_model || '',
                                        openai_organization: currentTestChannel.openai_organization || '',
                                        status_code_mapping: currentTestChannel.status_code_mapping || '',
                                        tag: currentTestChannel.tag || '',
                                        setting: currentTestChannel.setting || '',
                                        param_override: currentTestChannel.param_override || '',
                                        model_prefix: currentTestChannel.model_prefix || '',
                                        system_prompt: currentTestChannel.system_prompt || ''
                                      };

                                      const res = await API.put('/api/channel/', updateData);
                                      if (res.data.success) {
                                        // 更新本地状态
                                        updateChannelProperty(currentTestChannel.id, (channel) => {
                                          channel.models = newModels.join(',');
                                        });
                                        setCurrentTestChannel({
                                          ...currentTestChannel,
                                          models: newModels.join(',')
                                        });

                                        // 从测试结果中移除该模型
                                        setBatchTestResults(prev => prev.filter(r => r.model !== model.trim()));

                                        // 刷新数据以确保与服务器同步
                                        await refresh();

                                        showSuccess(t('模型 "${model}" 已删除').replace('${model}', model));
                                      } else {
                                        showError(res.data.message || t('删除失败'));
                                      }
                                    } catch (error) {
                                      showError(error.message || t('删除失败'));
                                    }
                                  }
                                });
                              }}
                            />
                          )}
                        </Space>

                        {/* 测试结果信息 */}
                        {testResult && (
                          <div style={{
                            marginTop: '4px',
                            fontSize: '11px',
                            color: isSuccess ? '#00b42a' : '#f53f3f',
                            textAlign: 'center'
                          }}>
                            {testResult.message}
                          </div>
                        )}
                      </div>
                    );
                  })}
              </div>

              {/* 显示搜索结果数量 */}
              {modelSearchKeyword && (
                <Typography.Text type='secondary' style={{ display: 'block' }}>
                  {t('找到')}{' '}
                  {
                    currentTestChannel.models
                      .split(',')
                      .filter((model) =>
                        model
                          .toLowerCase()
                          .includes(modelSearchKeyword.toLowerCase()),
                      ).length
                  }{' '}
                  {t('个模型')}
                </Typography.Text>
              )}
            </div>
          )}
        </div>
      </Modal>


    </>
  );
};

export default ChannelsTable;
