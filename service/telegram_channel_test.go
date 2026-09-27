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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupTelegramChannelTestDB wires an in-memory user database and points the
// service at a mock Telegram Bot API, restoring every package-level knob it
// touches so other tests never observe this test's configuration.
func setupTelegramChannelTestDB(t *testing.T, chatMemberStatus string) *model.User {
	t.Helper()
	previousDB, previousRedis := model.DB, common.RedisEnabled
	previousBotToken := common.TelegramBotToken
	previousBase := telegramBotAPIBase
	previousSettings := *system_setting.GetTelegramChannelGateSettings()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if chatMemberStatus == "" {
			w.Write([]byte(`{"ok":false,"description":"user not found"}`))
			return
		}
		w.Write([]byte(fmt.Sprintf(`{"ok":true,"result":{"status":%q}}`, chatMemberStatus)))
	}))
	t.Cleanup(server.Close)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	model.DB = db
	common.RedisEnabled = false
	common.TelegramBotToken = "test-token"
	telegramBotAPIBase = server.URL + "/bot"
	system_setting.GetTelegramChannelGateSettings().Enabled = true
	system_setting.GetTelegramChannelGateSettings().ChatId = "@testchannel"
	system_setting.GetTelegramChannelGateSettings().JoinLink = "https://t.me/testchannel"

	t.Cleanup(func() {
		model.DB = previousDB
		common.RedisEnabled = previousRedis
		common.TelegramBotToken = previousBotToken
		telegramBotAPIBase = previousBase
		*system_setting.GetTelegramChannelGateSettings() = previousSettings
		_ = sqlDB.Close()
	})

	user := &model.User{
		Username:    "telegram-channel-user",
		Password:    "unused-password-hash",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
		TelegramId:  "123456",
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func TestCheckTelegramChannelMembership(t *testing.T) {
	t.Run("member status counts as joined", func(t *testing.T) {
		setupTelegramChannelTestDB(t, "member")
		joined, err := CheckTelegramChannelMembership(context.Background(), "123456")
		require.NoError(t, err)
		assert.True(t, joined)
	})

	t.Run("left status counts as not joined", func(t *testing.T) {
		setupTelegramChannelTestDB(t, "left")
		joined, err := CheckTelegramChannelMembership(context.Background(), "123456")
		require.NoError(t, err)
		assert.False(t, joined)
	})

	t.Run("telegram ok=false is treated as not joined, not an error", func(t *testing.T) {
		setupTelegramChannelTestDB(t, "")
		joined, err := CheckTelegramChannelMembership(context.Background(), "123456")
		require.NoError(t, err)
		assert.False(t, joined)
	})

	t.Run("missing bot token or chat id reports not configured", func(t *testing.T) {
		setupTelegramChannelTestDB(t, "member")
		common.TelegramBotToken = ""
		_, err := CheckTelegramChannelMembership(context.Background(), "123456")
		assert.ErrorIs(t, err, ErrTelegramChannelGateNotConfigured)
	})
}

func TestVerifyTelegramChannelMembership(t *testing.T) {
	t.Run("gate disabled short-circuits without calling Telegram", func(t *testing.T) {
		user := setupTelegramChannelTestDB(t, "member")
		system_setting.GetTelegramChannelGateSettings().Enabled = false
		verified, err := VerifyTelegramChannelMembership(context.Background(), user.Id)
		assert.False(t, verified)
		assert.ErrorIs(t, err, ErrTelegramChannelGateDisabled)
	})

	t.Run("user without a linked Telegram account cannot verify", func(t *testing.T) {
		setupTelegramChannelTestDB(t, "member")
		user := &model.User{
			Username: "no-telegram-user", Password: "x", Role: common.RoleCommonUser,
			Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1,
			AffCode: "no-telegram-aff",
		}
		require.NoError(t, model.DB.Create(user).Error)
		verified, err := VerifyTelegramChannelMembership(context.Background(), user.Id)
		assert.False(t, verified)
		assert.ErrorIs(t, err, ErrTelegramAccountRequired)
	})

	t.Run("member is verified and durably recorded", func(t *testing.T) {
		user := setupTelegramChannelTestDB(t, "member")
		verified, err := VerifyTelegramChannelMembership(context.Background(), user.Id)
		require.NoError(t, err)
		assert.True(t, verified)

		reloaded, err := model.GetUserById(user.Id, false)
		require.NoError(t, err)
		assert.Greater(t, reloaded.TelegramChannelVerifiedAt, int64(0))
	})

	t.Run("non-member is not recorded as verified", func(t *testing.T) {
		user := setupTelegramChannelTestDB(t, "left")
		verified, err := VerifyTelegramChannelMembership(context.Background(), user.Id)
		require.NoError(t, err)
		assert.False(t, verified)

		reloaded, err := model.GetUserById(user.Id, false)
		require.NoError(t, err)
		assert.Equal(t, int64(0), reloaded.TelegramChannelVerifiedAt)
	})
}
