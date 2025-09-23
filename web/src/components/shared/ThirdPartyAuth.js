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
import { Button, Divider, Icon } from '@douyinfe/semi-ui';
import { IconGithubLogo } from '@douyinfe/semi-icons';
import TelegramLoginButton from 'react-telegram-login';
import {
  onGitHubOAuthClicked,
  onOIDCClicked,
  onLinuxDOOAuthClicked,
  onIDCFlareOAuthClicked,
} from '../utils';
import OIDCIcon from '../OIDCIcon.js';
import WeChatIcon from '../WeChatIcon';
import LinuxDoIcon from '../LinuxDoIcon.js';
import IDCFlareIcon from '../IDCFlareIcon.js';
import { useTranslation } from 'react-i18next';

const ThirdPartyAuth = ({ 
  status, 
  onWeChatLoginClicked, 
  onTelegramLoginClicked 
}) => {
  const { t } = useTranslation();

  const hasThirdPartyAuth = status.github_oauth ||
    status.oidc_enabled ||
    status.wechat_login ||
    status.telegram_oauth ||
    status.linuxdo_oauth ||
    status.idcflare_oauth;

  if (!hasThirdPartyAuth) {
    return null;
  }

  return (
    <>
      <Divider margin='12px' align='center'>
        {t('OR')}
      </Divider>
      <div
        style={{
          display: 'flex',
          flexDirection: 'column',
          gap: '12px',
          marginTop: 20,
        }}
      >
        {status.github_oauth && (
          <Button
            type='tertiary'
            theme='borderless'
            size='large'
            style={{
              border: '1px solid var(--semi-color-border)',
              borderRadius: '6px',
              padding: '12px 16px',
              width: '100%',
              justifyContent: 'center',
              position: 'relative',
            }}
            icon={<IconGithubLogo style={{ position: 'absolute', left: '16px', top: '50%', transform: 'translateY(-50%)' }} />}
            onClick={() =>
              onGitHubOAuthClicked(status.github_client_id)
            }
          >
            使用 GitHub 继续
          </Button>
        )}
        {status.oidc_enabled && (
          <Button
            type='tertiary'
            theme='borderless'
            size='large'
            style={{
              border: '1px solid var(--semi-color-border)',
              borderRadius: '6px',
              padding: '12px 16px',
              width: '100%',
              justifyContent: 'center',
              position: 'relative',
            }}
            icon={<OIDCIcon style={{ position: 'absolute', left: '16px', top: '50%', transform: 'translateY(-50%)' }} />}
            onClick={() =>
              onOIDCClicked(
                status.oidc_authorization_endpoint,
                status.oidc_client_id,
              )
            }
          >
            使用 OIDC 继续
          </Button>
        )}
        {status.linuxdo_oauth && (
          <Button
            type='tertiary'
            theme='borderless'
            size='large'
            style={{
              border: '1px solid var(--semi-color-border)',
              borderRadius: '6px',
              padding: '12px 16px',
              width: '100%',
              justifyContent: 'center',
              position: 'relative',
            }}
            icon={<LinuxDoIcon style={{ position: 'absolute', left: '16px', top: '50%', transform: 'translateY(-50%)' }} />}
            onClick={() =>
              onLinuxDOOAuthClicked(status.linuxdo_client_id)
            }
          >
            使用 LINUX DO 继续
          </Button>
        )}
        {status.idcflare_oauth && (
          <Button
            type='tertiary'
            theme='borderless'
            size='large'
            style={{
              border: '1px solid var(--semi-color-border)',
              borderRadius: '6px',
              padding: '12px 16px',
              width: '100%',
              justifyContent: 'center',
              position: 'relative',
            }}
            icon={<IDCFlareIcon style={{ position: 'absolute', left: '16px', top: '50%', transform: 'translateY(-50%)' }} />}
            onClick={() =>
              onIDCFlareOAuthClicked(status.idcflare_client_id)
            }
          >
            使用 IDC Flare 继续
          </Button>
        )}
        {status.wechat_login && (
          <Button
            type='tertiary'
            theme='borderless'
            size='large'
            style={{
              border: '1px solid var(--semi-color-border)',
              borderRadius: '6px',
              padding: '12px 16px',
              width: '100%',
              justifyContent: 'center',
              position: 'relative',
              color: 'rgba(var(--semi-green-5), 1)',
            }}
            icon={<Icon svg={<WeChatIcon />} style={{ position: 'absolute', left: '16px', top: '50%', transform: 'translateY(-50%)' }} />}
            onClick={onWeChatLoginClicked}
          >
            使用 微信 继续
          </Button>
        )}
      </div>
      {status.telegram_oauth && (
        <div
          style={{
            display: 'flex',
            justifyContent: 'center',
            marginTop: 5,
          }}
        >
          <TelegramLoginButton
            dataOnauth={onTelegramLoginClicked}
            botName={status.telegram_bot_name}
          />
        </div>
      )}
    </>
  );
};

export default ThirdPartyAuth;