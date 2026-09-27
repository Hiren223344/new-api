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
package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

var (
	ErrTelegramChannelGateNotConfigured = errors.New("Telegram channel verification is not configured. Please contact your administrator.")
	ErrTelegramChannelGateDisabled      = errors.New("Telegram channel verification is not required.")
	ErrTelegramAccountRequired          = errors.New("Link your Telegram account before verifying channel membership.")
	ErrTelegramChannelCheckFailed       = errors.New("Could not verify Telegram channel membership. Please try again.")
)

// telegramBotAPIBase is a var (not const) so tests can point it at a local
// httptest server instead of the real Telegram API.
var telegramBotAPIBase = "https://api.telegram.org/bot"

// telegramChannelHTTPClient is a package-level client so tests can swap its
// Transport without reaching into call sites.
var telegramChannelHTTPClient = &http.Client{Timeout: 10 * time.Second}

// telegramMemberStatuses are the getChatMember statuses that count as "still
// in the channel". "left" and "kicked" are the only statuses Telegram returns
// for someone who is not currently a member.
var telegramMemberStatuses = []string{"creator", "administrator", "member", "restricted"}

type telegramChatMemberResponse struct {
	Ok     bool `json:"ok"`
	Result struct {
		Status string `json:"status"`
	} `json:"result"`
}

// CheckTelegramChannelMembership calls the Telegram Bot API's getChatMember
// method to determine whether telegramUserID currently belongs to the
// administrator-configured channel. The bot must be an administrator of that
// channel for Telegram to report membership for an arbitrary user.
func CheckTelegramChannelMembership(ctx context.Context, telegramUserID string) (bool, error) {
	settings := system_setting.GetTelegramChannelGateSettings()
	botToken := strings.TrimSpace(common.TelegramBotToken)
	chatId := strings.TrimSpace(settings.ChatId)
	if botToken == "" || chatId == "" {
		return false, ErrTelegramChannelGateNotConfigured
	}
	telegramUserID = strings.TrimSpace(telegramUserID)
	if telegramUserID == "" {
		return false, ErrTelegramAccountRequired
	}

	endpoint := telegramBotAPIBase + botToken + "/getChatMember"
	values := url.Values{"chat_id": {chatId}, "user_id": {telegramUserID}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrTelegramChannelCheckFailed, err)
	}
	response, err := telegramChannelHTTPClient.Do(request)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrTelegramChannelCheckFailed, err)
	}
	defer response.Body.Close()

	var parsed telegramChatMemberResponse
	if err := common.DecodeJson(io.LimitReader(response.Body, 1<<20), &parsed); err != nil {
		return false, fmt.Errorf("%w: %v", ErrTelegramChannelCheckFailed, err)
	}
	if !parsed.Ok {
		// Telegram returns ok=false for an unknown chat_id, an unreachable user
		// (never started the bot) or a misconfigured token. Any of those mean
		// membership cannot be confirmed, not that the check itself failed, so
		// callers should not retry endlessly against a bad configuration.
		return false, nil
	}
	return slices.Contains(telegramMemberStatuses, parsed.Result.Status), nil
}

// VerifyTelegramChannelMembership checks and, on success, durably records that
// userId belongs to the configured Telegram channel.
func VerifyTelegramChannelMembership(ctx context.Context, userId int) (bool, error) {
	settings := system_setting.GetTelegramChannelGateSettings()
	if !settings.Enabled {
		return false, ErrTelegramChannelGateDisabled
	}
	user, err := model.GetUserById(userId, false)
	if err != nil {
		return false, err
	}
	if user.TelegramId == "" {
		return false, ErrTelegramAccountRequired
	}
	member, err := CheckTelegramChannelMembership(ctx, user.TelegramId)
	if err != nil {
		return false, err
	}
	if !member {
		return false, nil
	}
	if err := model.MarkTelegramChannelVerified(userId); err != nil {
		return false, err
	}
	return true, nil
}
