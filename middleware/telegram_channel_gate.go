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

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

const telegramChannelGateErrorMessage = "Join the required Telegram channel and verify your membership to continue."

// telegramChannelGateExemptPaths are the only dashboard endpoints a user must
// still reach while blocked: reading their own profile (so the frontend can
// render the gate), the account-binding verification ceremony needed to link
// Telegram in the first place, and the verification call that lifts the gate.
// Every other UserAuth/AdminAuth/RootAuth route is blocked while this is on.
var telegramChannelGateExemptPaths = map[string]bool{
	"/api/user/self":                    true,
	"/api/user/oauth/bindings":          true,
	"/api/user/passkey/verify/begin":    true,
	"/api/user/passkey/verify/finish":   true,
	"/api/user/telegram-channel/verify": true,
	"/api/verify/methods":               true,
	"/api/verify":                       true,
}

// telegramChannelGateBlocks reports whether the administrator-configured
// Telegram channel gate should block this user right now. Admin and root
// accounts are always exempt, so a misconfigured channel can never lock the
// operator out of the settings needed to fix it.
func telegramChannelGateBlocks(user *model.UserBase) bool {
	settings := system_setting.GetTelegramChannelGateSettings()
	return settings.Enabled && user.Role < common.RoleAdminUser && !user.IsTelegramChannelVerified()
}

// blockTelegramChannelGate enforces the gate for dashboard (session/PAT)
// requests reached through UserAuth/AdminAuth/RootAuth. It aborts and writes
// the dashboard JSON error shape when blocking, and reports whether it did.
func blockTelegramChannelGate(c *gin.Context, user *model.UserBase) bool {
	if !telegramChannelGateBlocks(user) || telegramChannelGateExemptPaths[c.Request.URL.Path] {
		return false
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"success": false,
		"code":    "TELEGRAM_CHANNEL_VERIFICATION_REQUIRED",
		"message": telegramChannelGateErrorMessage,
	})
	return true
}
