/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func enableTelegramChannelGateForTest(t *testing.T) {
	t.Helper()
	previous := *system_setting.GetTelegramChannelGateSettings()
	system_setting.GetTelegramChannelGateSettings().Enabled = true
	system_setting.GetTelegramChannelGateSettings().ChatId = "@testchannel"
	system_setting.GetTelegramChannelGateSettings().JoinLink = "https://t.me/testchannel"
	t.Cleanup(func() {
		*system_setting.GetTelegramChannelGateSettings() = previous
	})
}

func newTelegramChannelGateRouter() *gin.Engine {
	router := gin.New()
	protected := func(c *gin.Context) { c.Status(http.StatusOK) }
	router.GET("/protected", UserAuth(), protected)
	router.GET("/api/user/self", UserAuth(), protected)
	return router
}

func requestWithPAT(router *gin.Engine, path, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestTelegramChannelGateAllowsWhenDisabled(t *testing.T) {
	setupDashboardAuthMiddlewareTest(t)
	createMiddlewarePATUser(t, "gate-disabled-user", "gate-disabled-token")
	router := newTelegramChannelGateRouter()

	response := requestWithPAT(router, "/protected", "gate-disabled-token")

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestTelegramChannelGateBlocksUnverifiedCommonUser(t *testing.T) {
	setupDashboardAuthMiddlewareTest(t)
	enableTelegramChannelGateForTest(t)
	createMiddlewarePATUser(t, "gate-unverified-user", "gate-unverified-token")
	router := newTelegramChannelGateRouter()

	response := requestWithPAT(router, "/protected", "gate-unverified-token")

	assert.Equal(t, http.StatusForbidden, response.Code)
	assert.Contains(t, response.Body.String(), "TELEGRAM_CHANNEL_VERIFICATION_REQUIRED")
}

func TestTelegramChannelGateAllowsVerifiedUser(t *testing.T) {
	setupDashboardAuthMiddlewareTest(t)
	enableTelegramChannelGateForTest(t)
	user := createMiddlewarePATUser(t, "gate-verified-user", "gate-verified-token")
	require.NoError(t, model.MarkTelegramChannelVerified(user.Id))
	router := newTelegramChannelGateRouter()

	response := requestWithPAT(router, "/protected", "gate-verified-token")

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestTelegramChannelGateExemptsAdminAndRoot(t *testing.T) {
	setupDashboardAuthMiddlewareTest(t)
	enableTelegramChannelGateForTest(t)
	user := &model.User{
		Username: "gate-admin-user", Password: "password-placeholder", Role: common.RoleAdminUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1,
		AffCode: "gate-admin-aff", AccessToken: ptr("gate-admin-token"),
	}
	require.NoError(t, model.DB.Create(user).Error)
	router := newTelegramChannelGateRouter()

	response := requestWithPAT(router, "/protected", "gate-admin-token")

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestTelegramChannelGateExemptsSelfEndpointForUnverifiedUser(t *testing.T) {
	setupDashboardAuthMiddlewareTest(t)
	enableTelegramChannelGateForTest(t)
	createMiddlewarePATUser(t, "gate-self-user", "gate-self-token")
	router := newTelegramChannelGateRouter()

	response := requestWithPAT(router, "/api/user/self", "gate-self-token")

	assert.Equal(t, http.StatusOK, response.Code)
}

func ptr(s string) *string { return &s }
