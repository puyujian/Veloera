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
import { Icon } from '@douyinfe/semi-ui';

const IDCFlareIcon = (props) => {
  function CustomIcon() {
    return (
        <svg
            className='icon'
            viewBox='0 0 16 16'
            version='1.1'
            xmlns='http://www.w3.org/2000/svg'
            width='1em'
            height='1em'
            {...props}>
            <title>IDC Flare</title>
            <circle fill="#A81818" cx="512" cy="512" r="448"/>
            <g transform="translate(252,304)" fill="#fff">
                <path d="M213 0h307v89H338v73h155v84H338v170H213V0z"/>
                <rect width="125" height="416"/>
                <path d="m470 416-43-25v-50l43-25 43 25v50l-43 25z"/>
            </g>
        </svg>
    );
  }

  return <Icon svg={<CustomIcon />} />;
};

export default IDCFlareIcon;