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
package controller

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// VerifyTelegramChannelMembership re-checks the current user's membership in
// the administrator-configured Telegram channel and, on success, lifts the
// dashboard/API gate enforced by middleware.RequireTelegramChannelJoin.
func VerifyTelegramChannelMembership(c *gin.Context) {
	userId := c.GetInt("id")
	verified, err := service.VerifyTelegramChannelMembership(c.Request.Context(), userId)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTelegramChannelGateDisabled):
			common.ApiSuccess(c, gin.H{"verified": true})
		case errors.Is(err, service.ErrTelegramAccountRequired):
			c.JSON(200, gin.H{"success": false, "code": "TELEGRAM_ACCOUNT_NOT_LINKED", "message": err.Error()})
		case errors.Is(err, service.ErrTelegramChannelGateNotConfigured):
			c.JSON(200, gin.H{"success": false, "code": "TELEGRAM_CHANNEL_NOT_CONFIGURED", "message": err.Error()})
		default:
			common.ApiErrorMsg(c, err.Error())
		}
		return
	}
	recordUserSecurityAudit(c, userId, "user.telegram_channel_verify", map[string]any{"verified": verified})
	if !verified {
		c.JSON(200, gin.H{"success": false, "code": "TELEGRAM_CHANNEL_NOT_JOINED", "message": "You haven't joined the required Telegram channel yet."})
		return
	}
	common.ApiSuccess(c, gin.H{"verified": true})
}
