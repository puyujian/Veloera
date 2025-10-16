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
	"net/http"
	"strconv"
	"strings"
	"time"
	"veloera/common"
	"veloera/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Common HTTP client with timeout
var oauthHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
}

// OAuthProvider represents different OAuth providers
type OAuthProvider string

const (
	ProviderGitHub   OAuthProvider = "GitHub"
	ProviderLinuxDO  OAuthProvider = "Linux DO"
	ProviderIDCFlare OAuthProvider = "IDC Flare"
	ProviderOIDC     OAuthProvider = "OIDC"
	ProviderTelegram OAuthProvider = "Telegram"
)

// OAuthUser represents a generic OAuth user
type OAuthUser struct {
	ID          string
	Username    string
	DisplayName string
	Email       string
	Provider    OAuthProvider
	// Provider-specific fields
	TrustLevel int  // For LinuxDO
	Active     bool // For LinuxDO
	Silenced   bool // For LinuxDO
}

// OAuthConfig holds configuration for OAuth providers
type OAuthConfig struct {
	Enabled      bool
	ClientID     string
	ClientSecret string
	// Provider-specific configs
	MinTrustLevel int // For LinuxDO
}

// validateOAuthState validates the OAuth state parameter
func validateOAuthState(c *gin.Context) bool {
	session := sessions.Default(c)
	state := c.Query("state")

	if state == "" || session.Get("oauth_state") == nil || state != session.Get("oauth_state").(string) {
		respondWithError(c, http.StatusForbidden, "state is empty or not same")
		return false
	}
	return true
}

// respondWithError sends a standardized error response
func respondWithError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"success": false,
		"message": message,
	})
}

// respondWithSuccess sends a standardized success response
func respondWithSuccess(c *gin.Context, message string, data interface{}) {
	response := gin.H{
		"success": true,
		"message": message,
	}
	if data != nil {
		response["data"] = data
	}
	c.JSON(http.StatusOK, response)
}

// checkUserStatus validates if user is enabled
func checkUserStatus(c *gin.Context, user *model.User) bool {
	if user.Id == 0 {
		respondWithError(c, http.StatusOK, "用户已注销")
		return false
	}

	if user.Status != common.UserStatusEnabled {
		respondWithError(c, http.StatusOK, "用户已被封禁")
		return false
	}

	return true
}

// generateUniqueUsername generates a unique username string without inserting anything
func generateUniqueUsername(prefix string) (string, error) {
	baseUserId := model.GetMaxUserId() + 1

	for i := 0; i < 5; i++ {
		username := prefix + "_" + strconv.Itoa(baseUserId+i)
		// Check if username is already taken by querying the database
		exists, err := model.CheckUserExistOrDeleted(username, "")
		if err != nil {
			return "", err
		}
		if !exists {
			return username, nil
		}
	}

	// If all attempts failed, return a custom error
	return "", errors.New("failed to generate unique username after 5 attempts")
}

// handleOAuthLogin handles the common OAuth login flow
func handleOAuthLogin(c *gin.Context, oauthUser *OAuthUser, config *OAuthConfig,
	userExistsFunc func(string) bool,
	fillUserFunc func(*model.User) error,
	createUserFunc func(*model.User, *OAuthUser, int) error) {

	// Validate state
	if !validateOAuthState(c) {
		return
	}

	session := sessions.Default(c)
	username := session.Get("username")
	if username != nil {
		// User is already logged in, redirect to bind
		handleOAuthBind(c, oauthUser, config, userExistsFunc, fillUserFunc)
		return
	}

	if !config.Enabled {
		respondWithError(c, http.StatusOK, "管理员未开启通过 "+string(oauthUser.Provider)+" 登录以及注册")
		return
	}

	user := &model.User{}

	// Check if user exists
	if userExistsFunc(oauthUser.ID) {
		err := fillUserFunc(user)
		if err != nil {
			respondWithError(c, http.StatusOK, err.Error())
			return
		}

		if !checkUserStatus(c, user) {
			return
		}
	} else {
		// Register new user
		if !common.RegisterEnabled {
			respondWithError(c, http.StatusOK, "管理员关闭了新用户注册")
			return
		}

		// Get affiliate code
		affCode := session.Get("aff")
		inviterId := 0
		if affCode != nil {
			inviterId, _ = model.GetUserIdByAffCode(affCode.(string))
		}

		err := createUserFunc(user, oauthUser, inviterId)
		if err != nil {
			respondWithError(c, http.StatusOK, err.Error())
			return
		}

		if !checkUserStatus(c, user) {
			return
		}
	}

	setupLogin(user, c)
}

// handleOAuthBind handles the common OAuth bind flow
func handleOAuthBind(c *gin.Context, oauthUser *OAuthUser, config *OAuthConfig,
	userExistsFunc func(string) bool,
	fillUserFunc func(*model.User) error) {

	if !config.Enabled {
		respondWithError(c, http.StatusOK, "管理员未开启通过 "+string(oauthUser.Provider)+" 登录以及注册")
		return
	}

	if userExistsFunc(oauthUser.ID) {
		respondWithError(c, http.StatusOK, "该 "+string(oauthUser.Provider)+" 账户已被绑定")
		return
	}

	session := sessions.Default(c)
	id := session.Get("id")
	user := &model.User{Id: id.(int)}

	err := user.FillUserById()
	if err != nil {
		respondWithError(c, http.StatusOK, err.Error())
		return
	}

	// Set the provider-specific ID based on provider type
	switch oauthUser.Provider {
	case ProviderGitHub:
		user.GitHubId = oauthUser.ID
	case ProviderLinuxDO:
		user.LinuxDOId = oauthUser.ID
	case ProviderIDCFlare:
		user.IDCFlareId = oauthUser.ID
	case ProviderOIDC:
		user.OidcId = oauthUser.ID
	case ProviderTelegram:
		user.TelegramId = oauthUser.ID
	}

	err = user.Update(false)
	if err != nil {
		respondWithError(c, http.StatusOK, err.Error())
		return
	}

	respondWithSuccess(c, "bind", nil)
}

// createGitHubUser creates a new user from GitHub OAuth data
func createGitHubUser(user *model.User, oauthUser *OAuthUser, inviterId int) error {
	user.GitHubId = oauthUser.ID
	user.Email = oauthUser.Email
	user.Role = common.RoleCommonUser
	user.Status = common.UserStatusEnabled

	if oauthUser.DisplayName != "" {
		user.DisplayName = oauthUser.DisplayName
	} else {
		user.DisplayName = "GitHub User"
	}

	// Generate unique username
	username, err := generateUniqueUsername("github")
	if err != nil {
		return err
	}

	user.Username = username
	return user.Insert(inviterId)
}

// createLinuxDOUser creates a new user from LinuxDO OAuth data
func createLinuxDOUser(user *model.User, oauthUser *OAuthUser, inviterId int) error {
	// Check trust level requirement
	if oauthUser.TrustLevel < common.LinuxDOMinimumTrustLevel {
		return errors.New("信任等级未达到管理员设置的最低信任等级")
	}

	user.LinuxDOId = oauthUser.ID
	user.DisplayName = oauthUser.DisplayName
	user.Role = common.RoleCommonUser
	user.Status = common.UserStatusEnabled

	// Generate unique username
	username, err := generateUniqueUsername("linuxdo")
	if err != nil {
		return err
	}

	user.Username = username
	return user.Insert(inviterId)
}

// createIDCFlareUser creates a new user from IDC Flare OAuth data
func createIDCFlareUser(user *model.User, oauthUser *OAuthUser, inviterId int) error {
	// Check trust level requirement
	if oauthUser.TrustLevel < common.IDCFlareMinimumTrustLevel {
		return errors.New("信任等级未达到管理员设置的最低信任等级")
	}

	user.IDCFlareId = oauthUser.ID
	user.DisplayName = oauthUser.DisplayName
	user.Role = common.RoleCommonUser
	user.Status = common.UserStatusEnabled

	// Generate unique username
	username, err := generateUniqueUsername("idcflare")
	if err != nil {
		return err
	}

	user.Username = username
	return user.Insert(inviterId)
}

// createOIDCUser creates a new user from OIDC OAuth data
func createOIDCUser(user *model.User, oauthUser *OAuthUser, inviterId int) error {
	user.OidcId = oauthUser.ID
	user.Email = oauthUser.Email
	user.Role = common.RoleCommonUser
	user.Status = common.UserStatusEnabled

	if oauthUser.DisplayName != "" {
		user.DisplayName = oauthUser.DisplayName
	} else {
		user.DisplayName = "OIDC User"
	}

	// Handle username - prefer provided username, fallback to generated
	if oauthUser.Username != "" {
		user.Username = oauthUser.Username
		err := user.Insert(inviterId)
		if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: users.username") {
			// Fallback to generated username
			username, genErr := generateUniqueUsername("oidc")
			if genErr != nil {
				return genErr
			}
			user.Username = username
			return user.Insert(inviterId)
		}
		return err
	} else {
		// Generate unique username
		username, err := generateUniqueUsername("oidc")
		if err != nil {
			return err
		}
		user.Username = username
		return user.Insert(inviterId)
	}
}
