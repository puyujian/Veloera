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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"veloera/common"
	"veloera/constant"
	"veloera/model"
	"veloera/setting"
	"veloera/setting/operation_setting"
	"veloera/setting/system_setting"

	"github.com/gin-gonic/gin"
)

func TestStatus(c *gin.Context) {
	err := model.PingDB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "数据库连接失败",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Server is running",
	})
	return
}

func GetStatus(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	affEnabled := common.OptionMap["AffEnabled"] == "true"
	logChatContentEnabled := common.OptionMap["LogChatContentEnabled"] == "true"
	common.OptionMapRWMutex.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"version":                      common.Version,
			"start_time":                   common.StartTime,
			"email_verification":           common.EmailVerificationEnabled,
			"github_oauth":                 common.GitHubOAuthEnabled,
			"github_client_id":             common.GitHubClientId,
			"linuxdo_oauth":                common.LinuxDOOAuthEnabled,
			"linuxdo_client_id":            common.LinuxDOClientId,
			"linuxdo_minimum_trust_level":  common.LinuxDOMinimumTrustLevel,
			"idcflare_oauth":               common.IDCFlareOAuthEnabled,
			"idcflare_client_id":           common.IDCFlareClientId,
			"idcflare_minimum_trust_level": common.IDCFlareMinimumTrustLevel,
			"telegram_oauth":               common.TelegramOAuthEnabled,
			"telegram_bot_name":            common.TelegramBotName,
			"system_name":                  common.SystemName,
			"logo":                         common.Logo,
			"footer_html":                  common.Footer,
			"wechat_qrcode":                common.WeChatAccountQRCodeImageURL,
			"wechat_login":                 common.WeChatAuthEnabled,
			"server_address":               setting.ServerAddress,
			"price":                        setting.Price,
			"min_topup":                    setting.MinTopUp,
			"turnstile_check":              common.TurnstileCheckEnabled,
			"turnstile_site_key":           common.TurnstileSiteKey,
			"top_up_link":                  common.TopUpLink,
			"docs_link":                    operation_setting.GetGeneralSetting().DocsLink,
			"quota_per_unit":               common.QuotaPerUnit,
			"display_in_currency":          common.DisplayInCurrencyEnabled,
			"enable_batch_update":          common.BatchUpdateEnabled,
			"enable_drawing":               common.DrawingEnabled,
			"enable_task":                  common.TaskEnabled,
			"enable_data_export":           common.DataExportEnabled,
			"data_export_default_time":     common.DataExportDefaultTime,
			"default_collapse_sidebar":     common.DefaultCollapseSidebar,
			"enable_online_topup":          setting.PayAddress != "" && setting.EpayId != "" && setting.EpayKey != "",
			"mj_notify_enabled":            setting.MjNotifyEnabled,
			"chats":                        setting.Chats,
			"demo_site_enabled":            operation_setting.DemoSiteEnabled,
			"self_use_mode_enabled":        operation_setting.SelfUseModeEnabled,
			"oidc_enabled":                 system_setting.GetOIDCSettings().Enabled,
			"oidc_client_id":               system_setting.GetOIDCSettings().ClientId,
			"oidc_authorization_endpoint":  system_setting.GetOIDCSettings().AuthorizationEndpoint,
			"setup":                        constant.Setup,
			"check_in_enabled":             common.CheckInEnabled,
			"aff_enabled":                  affEnabled,
			"log_chat_content_enabled":     logChatContentEnabled,
		},
	})
	return
}

func GetNotice(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    common.OptionMap["Notice"],
	})
	return
}

func GetAbout(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    common.OptionMap["About"],
	})
	return
}

func GetMidjourney(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    common.OptionMap["Midjourney"],
	})
	return
}

func GetHomePageContent(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	content := common.OptionMap["HomePageContent"]
	common.OptionMapRWMutex.RUnlock()

	// For HTML content starting with <!DOCTYPE, we need to prevent JSON encoding from escaping HTML
	// Check if it's a complete HTML document
	if strings.HasPrefix(strings.TrimSpace(content), "<!DOCTYPE") || strings.HasPrefix(strings.TrimSpace(content), "<html") {
		// Use custom JSON encoder that doesn't escape HTML
		response := map[string]interface{}{
			"success": true,
			"message": "",
			"data":    content,
		}

		encoder := json.NewEncoder(c.Writer)
		encoder.SetEscapeHTML(false) // Don't escape HTML characters
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusOK)
		if err := encoder.Encode(response); err != nil {
			// Fallback to regular JSON if encoding fails
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Failed to encode response",
			})
		}
		return
	}

	// For non-HTML content, use regular JSON encoding
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    content,
	})
	return
}

func SendEmailVerification(c *gin.Context) {
	email := c.Query("email")
	if err := common.Validate.Var(email, "required,email"); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的邮箱地址",
		})
		return
	}
	localPart := parts[0]
	domainPart := parts[1]
	if common.EmailDomainRestrictionEnabled {
		allowed := false
		for _, domain := range common.EmailDomainWhitelist {
			if domainPart == domain {
				allowed = true
				break
			}
		}
		if !allowed {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "The administrator has enabled the email domain name whitelist, and your email address is not allowed due to special symbols or it's not in the whitelist.",
			})
			return
		}
	}
	if common.EmailAliasRestrictionEnabled {
		containsSpecialSymbols := strings.Contains(localPart, "+") || strings.Contains(localPart, ".")
		if containsSpecialSymbols {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "管理员已启用邮箱地址别名限制，您的邮箱地址由于包含特殊符号而被拒绝。",
			})
			return
		}
	}

	if model.IsEmailAlreadyTaken(email) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "邮箱地址已被占用",
		})
		return
	}
	code := common.GenerateVerificationCode(6)
	common.RegisterVerificationCodeWithKey(email, code, common.EmailVerificationPurpose)
	subject := fmt.Sprintf("%s邮箱验证邮件", common.SystemName)
	content := fmt.Sprintf("<p>您好，你正在进行%s邮箱验证。</p>"+
		"<p>您的验证码为: <strong>%s</strong></p>"+
		"<p>验证码 %d 分钟内有效，如果不是本人操作，请忽略。</p>", common.SystemName, code, common.VerificationValidMinutes)
	err := common.SendEmail(subject, email, content)
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

func SendPasswordResetEmail(c *gin.Context) {
	email := c.Query("email")
	if err := common.Validate.Var(email, "required,email"); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	if !model.IsEmailAlreadyTaken(email) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "该邮箱地址未注册",
		})
		return
	}
	code := common.GenerateVerificationCode(0)
	common.RegisterVerificationCodeWithKey(email, code, common.PasswordResetPurpose)
	link := fmt.Sprintf("%s/user/reset?email=%s&token=%s", setting.ServerAddress, email, code)
	subject := fmt.Sprintf("%s密码重置", common.SystemName)
	content := fmt.Sprintf("<p>您好，你正在进行%s密码重置。</p>"+
		"<p>点击 <a href='%s'>此处</a> 进行密码重置。</p>"+
		"<p>如果链接无法点击，请尝试点击下面的链接或将其复制到浏览器中打开：<br> %s </p>"+
		"<p>重置链接 %d 分钟内有效，如果不是本人操作，请忽略。</p>", common.SystemName, link, link, common.VerificationValidMinutes)
	err := common.SendEmail(subject, email, content)
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

type PasswordResetRequest struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

func ResetPassword(c *gin.Context) {
	var req PasswordResetRequest
	err := json.NewDecoder(c.Request.Body).Decode(&req)
	if req.Email == "" || req.Token == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	if !common.VerifyCodeWithKey(req.Email, req.Token, common.PasswordResetPurpose) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "重置链接非法或已过期",
		})
		return
	}
	password := common.GenerateVerificationCode(12)
	err = model.ResetUserPasswordByEmail(req.Email, password)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	common.DeleteKey(req.Email, common.PasswordResetPurpose)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    password,
	})
	return
}

// GetCustomCSS serves the global CSS content
func GetCustomCSS(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	cssContent := common.OptionMap["global_css"]
	common.OptionMapRWMutex.RUnlock()

	if cssContent == "" {
		c.Status(http.StatusNoContent)
		return
	}

	// Basic security validation to prevent script tag injection in CSS
	if strings.Contains(strings.ToLower(cssContent), "<script") {
		c.Status(http.StatusBadRequest)
		return
	}

	c.Header("Content-Type", "text/css")
	c.String(http.StatusOK, cssContent)
}

// GetCustomJS serves the global JavaScript content
func GetCustomJS(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	jsContent := common.OptionMap["global_js"]
	common.OptionMapRWMutex.RUnlock()

	if jsContent == "" {
		c.Status(http.StatusNoContent)
		return
	}

	c.Header("Content-Type", "application/javascript")
	c.String(http.StatusOK, jsContent)
}
