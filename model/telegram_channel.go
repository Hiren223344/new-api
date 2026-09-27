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
package model

import "github.com/QuantumNous/new-api/common"

// MarkTelegramChannelVerified records that userId is currently a member of the
// administrator-configured Telegram channel. It does not change AuthVersion:
// verification is not a credential change and must not revoke other sessions.
func MarkTelegramChannelVerified(userId int) error {
	now := common.GetTimestamp()
	if err := DB.Model(&User{}).Where("id = ?", userId).Update("telegram_channel_verified_at", now).Error; err != nil {
		return err
	}
	return updateUserCacheField(userId, "TelegramChannelVerifiedAt", now)
}
