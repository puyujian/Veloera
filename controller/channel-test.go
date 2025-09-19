// Copyright (c) 2025 Tethys Plex
//
// This file is part of Veloera.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.
package controller

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
	"veloera/common"
	"veloera/dto"
	"veloera/model"
	"veloera/service"

	"github.com/bytedance/gopkg/util/gopool"

	"github.com/gin-gonic/gin"
)

func TestChannel(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	testModel := c.Query("model")
	consumedTime, execErr, _ := service.ExecuteChannelTest(channel, testModel)

	if execErr != nil {
		milliseconds := int64(consumedTime * 1000)
		if milliseconds > 0 {
			go channel.UpdateResponseTime(milliseconds)
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": execErr.Error(),
			"time":    consumedTime,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"time":    consumedTime,
	})
}

var testAllChannelsLock sync.Mutex
var testAllChannelsRunning bool = false

func testAllChannels(notify bool) error {

	testAllChannelsLock.Lock()
	if testAllChannelsRunning {
		testAllChannelsLock.Unlock()
		return errors.New("测试已在运行中")
	}
	testAllChannelsRunning = true
	testAllChannelsLock.Unlock()
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		return err
	}
	var disableThreshold = int64(common.ChannelDisableThreshold * 1000)
	if disableThreshold == 0 {
		disableThreshold = 10000000 // a impossible value
	}
	gopool.Go(func() {
		for _, channel := range channels {
			isChannelEnabled := channel.Status == common.ChannelStatusEnabled
			consumedTime, execErr, openaiWithStatusErr := service.ExecuteChannelTest(channel, "")
			milliseconds := int64(consumedTime * 1000)

			shouldBanChannel := false
			err := execErr

			if openaiWithStatusErr != nil {
				oaiErr := openaiWithStatusErr.Error
				err = errors.New(fmt.Sprintf("type %s, httpCode %d, code %v, message %s", oaiErr.Type, openaiWithStatusErr.StatusCode, oaiErr.Code, oaiErr.Message))
				shouldBanChannel = service.ShouldDisableChannel(channel.Type, openaiWithStatusErr)
			}

			if milliseconds > disableThreshold {
				err = errors.New(fmt.Sprintf("响应时间 %.2fs 超过阈值 %.2fs", float64(milliseconds)/1000.0, float64(disableThreshold)/1000.0))
				shouldBanChannel = true
			}

			if isChannelEnabled && shouldBanChannel && err != nil && channel.GetAutoBan() {
				service.DisableChannel(channel.Id, channel.Name, err.Error())
			}

			if !isChannelEnabled && service.ShouldEnableChannel(err, openaiWithStatusErr, channel.Status) {
				service.EnableChannel(channel.Id, channel.Name)
			}

			if milliseconds > 0 {
				channel.UpdateResponseTime(milliseconds)
			}
			time.Sleep(common.RequestInterval)
		}
		testAllChannelsLock.Lock()
		testAllChannelsRunning = false
		testAllChannelsLock.Unlock()
		if notify {
			service.NotifyRootUser(dto.NotifyTypeChannelTest, "通道测试完成", "所有通道测试已完成")
		}
	})
	return nil
}

func TestAllChannels(c *gin.Context) {
	err := testAllChannels(true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func AutomaticallyTestChannels(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Minute)
		common.SysLog("testing all channels")
		_ = testAllChannels(false)
		common.SysLog("channel test finished")
	}
}
