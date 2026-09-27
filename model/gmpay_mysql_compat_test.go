package model

import (
	"os"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestRechargeGmpayMySQLCompat verifies RechargeGmpay's row-locked transaction
// against a real MySQL instance. GM Pay itself added no schema changes (it
// reuses the existing Option table and TopUp.payment_provider column), but
// the transaction + lockForUpdate path is dialect-sensitive and deserves its
// own real-database check. Skipped unless MYSQL_COMPAT_DSN is set.
func TestRechargeGmpayMySQLCompat(t *testing.T) {
	dsn := os.Getenv("MYSQL_COMPAT_DSN")
	if dsn == "" {
		t.Skip("MYSQL_COMPAT_DSN not set; skipping real-MySQL compatibility check")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.Migrator().DropTable(&User{}, &TopUp{}))
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}))

	previousDB, previousRedis, previousType := DB, common.RedisEnabled, common.MainDatabaseType()
	DB = db
	common.RedisEnabled = false
	common.SetMainDatabaseType(common.DatabaseTypeMySQL)
	t.Cleanup(func() {
		DB, common.RedisEnabled = previousDB, previousRedis
		common.SetMainDatabaseType(previousType)
	})

	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	user := &User{Username: "gmpay-mysql-user", Password: "hash", Group: "default", AffCode: "gmpay-mysql-aff", AuthVersion: 1}
	require.NoError(t, db.Create(user).Error)
	topUp := &TopUp{
		UserId: user.Id, Amount: 2, Money: 9.99, TradeNo: "GMPAY-MYSQL-COMPAT",
		PaymentMethod: PaymentMethodGmpay, PaymentProvider: PaymentProviderGmpay, Status: common.TopUpStatusPending,
	}
	require.NoError(t, topUp.Insert())

	err = RechargeGmpay("GMPAY-MYSQL-COMPAT", "127.0.0.1")
	require.NoError(t, err)

	var reloaded User
	require.NoError(t, db.First(&reloaded, user.Id).Error)
	assert.Equal(t, 2*500000, reloaded.Quota)

	// Idempotent under concurrent delivery: MySQL's row lock must serialize
	// these so only one of N concurrent webhook retries credits the quota.
	const concurrentDeliveries = 5
	var wg sync.WaitGroup
	for range concurrentDeliveries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = RechargeGmpay("GMPAY-MYSQL-COMPAT", "127.0.0.1")
		}()
	}
	wg.Wait()

	require.NoError(t, db.First(&reloaded, user.Id).Error)
	assert.Equal(t, 2*500000, reloaded.Quota, "concurrent webhook retries must not double-credit")

	// AutoMigrate on top of an already-migrated table must remain a no-op.
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}))
}
