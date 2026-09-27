package model

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// oldUserForCompatTest replicates the User struct as it existed before the
// telegram_channel_verified_at column was added, so the upgrade test can
// simulate a real production database created by the previous release.
type oldUserForCompatTest struct {
	Id                   int                        `json:"id"`
	Username             string                     `json:"username" gorm:"unique;index" validate:"max=20"`
	Password             string                     `json:"password" gorm:"not null;" validate:"min=8,max=128"`
	HasPassword          bool                       `json:"-" gorm:"-:all"`
	OriginalPassword     string                     `json:"original_password" gorm:"-:all"`
	DisplayName          string                     `json:"display_name" gorm:"index" validate:"max=20"`
	Role                 int                        `json:"role" gorm:"type:int;default:1"`
	Status               int                        `json:"status" gorm:"type:int;default:1"`
	Email                string                     `json:"email" gorm:"index" validate:"max=50"`
	GitHubId             string                     `json:"github_id" gorm:"column:github_id;index"`
	DiscordId            string                     `json:"discord_id" gorm:"column:discord_id;index"`
	OidcId               string                     `json:"oidc_id" gorm:"column:oidc_id;index"`
	WeChatId             string                     `json:"wechat_id" gorm:"column:wechat_id;index"`
	TelegramId           string                     `json:"telegram_id" gorm:"column:telegram_id;index"`
	VerificationCode     string                     `json:"verification_code" gorm:"-:all"`
	AccessToken          *string                    `json:"-" gorm:"type:char(32);column:access_token;uniqueIndex"`
	AccessTokenCreatedAt *int64                     `json:"-" gorm:"type:bigint;column:access_token_created_at"`
	Quota                int                        `json:"quota" gorm:"type:int;default:0"`
	UsedQuota            int                        `json:"used_quota" gorm:"type:int;default:0;column:used_quota"`
	RequestCount         int                        `json:"request_count" gorm:"type:int;default:0;"`
	Group                string                     `json:"group" gorm:"type:varchar(64);default:'default'"`
	AffCode              string                     `json:"aff_code" gorm:"type:varchar(32);column:aff_code;uniqueIndex"`
	AffCount             int                        `json:"aff_count" gorm:"type:int;default:0;column:aff_count"`
	AffQuota             int                        `json:"aff_quota" gorm:"type:int;default:0;column:aff_quota"`
	AffHistoryQuota      int                        `json:"aff_history_quota" gorm:"type:int;default:0;column:aff_history"`
	InviterId            int                        `json:"inviter_id" gorm:"type:int;column:inviter_id;index"`
	DeletedAt            gorm.DeletedAt             `gorm:"index"`
	LinuxDOId            string                     `json:"linux_do_id" gorm:"column:linux_do_id;index"`
	Setting              string                     `json:"setting" gorm:"type:text;column:setting"`
	Remark               string                     `json:"remark,omitempty" gorm:"type:varchar(255)" validate:"max=255"`
	StripeCustomer       string                     `json:"stripe_customer" gorm:"type:varchar(64);column:stripe_customer;index"`
	CreatedAt            int64                      `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	LastLoginAt          int64                      `json:"last_login_at" gorm:"default:0;column:last_login_at"`
	AuthVersion          int64                      `json:"-" gorm:"type:bigint;not null;default:1;column:auth_version"`
	AdminPermissions     map[string]map[string]bool `json:"admin_permissions,omitempty" gorm:"-:all"`
}

func (oldUserForCompatTest) TableName() string { return "users" }

// TestMySQLTelegramChannelColumnCompat verifies the telegram_channel_verified_at
// migration against a real MySQL instance, per AGENTS.md's three-database
// compatibility rule: it must succeed on a fresh database, be idempotent, and
// preserve existing data/constraints when upgrading a database created by the
// previous schema. Skipped unless MYSQL_COMPAT_DSN points at a real server.
func TestMySQLTelegramChannelColumnCompat(t *testing.T) {
	dsn := os.Getenv("MYSQL_COMPAT_DSN")
	if dsn == "" {
		t.Skip("MYSQL_COMPAT_DSN not set; skipping real-MySQL compatibility check")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	previousDB, previousRedis := DB, common.RedisEnabled
	DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		DB, common.RedisEnabled = previousDB, previousRedis
		_ = sqlDB.Close()
	})

	t.Run("fresh database migration is idempotent and adds the column", func(t *testing.T) {
		require.NoError(t, db.Migrator().DropTable("users"))
		require.NoError(t, db.AutoMigrate(&User{}), "first AutoMigrate on a fresh database")
		require.NoError(t, db.AutoMigrate(&User{}), "second AutoMigrate must be a no-op, not an error")

		var dataType, isNullable, columnDefault string
		row := db.Raw(`SELECT DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'telegram_channel_verified_at'`).Row()
		require.NoError(t, row.Scan(&dataType, &isNullable, &columnDefault))
		assert.Equal(t, "bigint", dataType)
		assert.Equal(t, "0", columnDefault)
	})

	t.Run("upgrading an existing database preserves data and constraints", func(t *testing.T) {
		require.NoError(t, db.Migrator().DropTable("users"))
		// Simulate a production database created by the previous release,
		// before this column existed.
		require.NoError(t, db.AutoMigrate(&oldUserForCompatTest{}))

		preExisting := &oldUserForCompatTest{
			Username: "pre-upgrade-user", Password: "hash", Role: common.RoleCommonUser,
			Status: 1, Group: "default", AffCode: "compat-check-aff", AuthVersion: 1,
			TelegramId: "999999",
		}
		require.NoError(t, db.Create(preExisting).Error)

		// Run the upgrade twice to prove idempotency on a non-empty table too.
		require.NoError(t, db.AutoMigrate(&User{}), "upgrade migration")
		require.NoError(t, db.AutoMigrate(&User{}), "upgrade migration must be idempotent")

		var upgraded User
		require.NoError(t, db.First(&upgraded, preExisting.Id).Error)
		assert.Equal(t, "pre-upgrade-user", upgraded.Username)
		assert.Equal(t, "999999", upgraded.TelegramId)
		assert.Equal(t, "compat-check-aff", upgraded.AffCode)
		assert.Equal(t, int64(0), upgraded.TelegramChannelVerifiedAt)

		// The unique index on aff_code must survive the migration.
		duplicate := &User{Username: "second-user", Password: "hash", Group: "default", AffCode: "compat-check-aff", AuthVersion: 1}
		require.Error(t, db.Create(duplicate).Error, "aff_code uniqueness must still be enforced after migration")

		require.NoError(t, MarkTelegramChannelVerified(preExisting.Id))
		var verified User
		require.NoError(t, db.First(&verified, preExisting.Id).Error)
		assert.Greater(t, verified.TelegramChannelVerifiedAt, int64(0))
	})
}
