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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"veloera/common"
	"veloera/model"

	"github.com/gin-gonic/gin"
)

type IdcflareUser struct {
	Id         int    `json:"id"`
	Username   string `json:"username"`
	Name       string `json:"name"`
	Active     bool   `json:"active"`
	TrustLevel int    `json:"trust_level"`
	Silenced   bool   `json:"silenced"`
}

func IDCFlareBind(c *gin.Context) {
	code := c.Query("code")
	idcflareUser, err := getIdcflareUserInfoByCode(code, c)
	if err != nil {
		respondWithError(c, http.StatusOK, err.Error())
		return
	}

	oauthUser := &OAuthUser{
		ID:          strconv.Itoa(idcflareUser.Id),
		Username:    idcflareUser.Username,
		DisplayName: idcflareUser.Name,
		Provider:    ProviderIDCFlare,
		TrustLevel:  idcflareUser.TrustLevel,
		Active:      idcflareUser.Active,
		Silenced:    idcflareUser.Silenced,
	}

	config := &OAuthConfig{
		Enabled:       common.IDCFlareOAuthEnabled,
		MinTrustLevel: common.IDCFlareMinimumTrustLevel,
	}

	handleOAuthBind(c, oauthUser, config,
		model.IsIDCFlareIdAlreadyTaken,
		func(user *model.User) error {
			user.IDCFlareId = oauthUser.ID
			return user.FillUserByIDCFlareId()
		},
	)
}

func getIdcflareUserInfoByCode(code string, c *gin.Context) (*IdcflareUser, error) {
	if code == "" {
		return nil, errors.New("invalid code")
	}

	// Get access token using Basic auth
	tokenEndpoint := "https://connect.idcflare.com/oauth2/token"
	credentials := common.IDCFlareClientId + ":" + common.IDCFlareClientSecret
	basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials))

	// Get redirect URI from request
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	redirectURI := fmt.Sprintf("%s://%s/api/oauth/idcflare", scheme, c.Request.Host)

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest("POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", basicAuth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, errors.New("failed to connect to IDC Flare server")
	}
	defer res.Body.Close()

	var tokenRes struct {
		AccessToken string `json:"access_token"`
		Message     string `json:"message"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tokenRes); err != nil {
		return nil, err
	}

	if tokenRes.AccessToken == "" {
		return nil, fmt.Errorf("failed to get access token: %s", tokenRes.Message)
	}

	// Get user info
	userEndpoint := "https://connect.idcflare.com/api/user"
	req, err = http.NewRequest("GET", userEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)
	req.Header.Set("Accept", "application/json")

	res2, err := client.Do(req)
	if err != nil {
		return nil, errors.New("failed to get user info from IDC Flare")
	}
	defer res2.Body.Close()

	var idcflareUser IdcflareUser
	if err := json.NewDecoder(res2.Body).Decode(&idcflareUser); err != nil {
		return nil, err
	}

	if idcflareUser.Id == 0 {
		return nil, errors.New("invalid user info returned")
	}

	return &idcflareUser, nil
}

func IdcflareOAuth(c *gin.Context) {
	// Handle error from OAuth provider
	errorCode := c.Query("error")
	if errorCode != "" {
		errorDescription := c.Query("error_description")
		respondWithError(c, http.StatusOK, errorDescription)
		return
	}

	code := c.Query("code")
	idcflareUser, err := getIdcflareUserInfoByCode(code, c)
	if err != nil {
		respondWithError(c, http.StatusOK, err.Error())
		return
	}

	oauthUser := &OAuthUser{
		ID:          strconv.Itoa(idcflareUser.Id),
		Username:    idcflareUser.Username,
		DisplayName: idcflareUser.Name,
		Provider:    ProviderIDCFlare,
		TrustLevel:  idcflareUser.TrustLevel,
		Active:      idcflareUser.Active,
		Silenced:    idcflareUser.Silenced,
	}

	config := &OAuthConfig{
		Enabled:       common.IDCFlareOAuthEnabled,
		MinTrustLevel: common.IDCFlareMinimumTrustLevel,
	}

	handleOAuthLogin(c, oauthUser, config,
		model.IsIDCFlareIdAlreadyTaken,
		func(user *model.User) error {
			user.IDCFlareId = oauthUser.ID
			return user.FillUserByIDCFlareId()
		},
		createIDCFlareUser,
	)
}
