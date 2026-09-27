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
package system_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// TelegramChannelGateSettings configures the mandatory "join our Telegram
// channel" gate. ChatId is passed to the Telegram Bot API's getChatMember
// call (a numeric chat id or an @channel username); JoinLink is the URL shown
// to the user so they can actually join it. Verification itself reuses the
// bot token already stored under the legacy TelegramBotToken option.
type TelegramChannelGateSettings struct {
	Enabled  bool   `json:"enabled"`
	ChatId   string `json:"chat_id"`
	JoinLink string `json:"join_link"`
}

var telegramChannelGateSettings TelegramChannelGateSettings

func init() {
	config.GlobalConfig.Register("telegram_channel", &telegramChannelGateSettings)
}

func GetTelegramChannelGateSettings() *TelegramChannelGateSettings {
	return &telegramChannelGateSettings
}

func (s *TelegramChannelGateSettings) IsConfigured() bool {
	return strings.TrimSpace(s.ChatId) != "" && strings.TrimSpace(s.JoinLink) != ""
}
