package controller

import (
	"maps"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

// TestGetGmpayPayMoney guards against reintroducing the Epay Price ratio
// (local currency per USD) into GM Pay's USD-denominated pricing: GM Pay
// must use its own GmpayUnitPrice, same as Stripe and Waffo use their own
// unit price settings instead of Epay's.
func TestGetGmpayPayMoney(t *testing.T) {
	originalUnitPrice := setting.GmpayUnitPrice
	originalEpayPrice := operation_setting.Price
	originalQuotaDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	originalDiscounts := make(map[int]float64, len(operation_setting.GetPaymentSetting().AmountDiscount))
	maps.Copy(originalDiscounts, operation_setting.GetPaymentSetting().AmountDiscount)
	originalTopupGroupRatio := common.TopupGroupRatio2JSONString()

	t.Cleanup(func() {
		setting.GmpayUnitPrice = originalUnitPrice
		operation_setting.Price = originalEpayPrice
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalQuotaDisplayType
		operation_setting.GetPaymentSetting().AmountDiscount = originalDiscounts
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(originalTopupGroupRatio))
	})

	setting.GmpayUnitPrice = 1.0
	// A local-currency Epay ratio far from 1.0 must not leak into GM Pay's
	// USD pricing; this is the exact bug being regression-tested here.
	operation_setting.Price = 7.3
	operation_setting.GetPaymentSetting().AmountDiscount = map[int]float64{
		10: 0.8,
		20: 0,
	}
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"default":1,"vip":1.2}`))

	testCases := []struct {
		name             string
		amount           float64
		group            string
		quotaDisplayType string
		expected         float64
	}{
		{
			name:             "one dollar top-up charges one dollar, not the Epay local-currency ratio",
			amount:           1,
			group:            "default",
			quotaDisplayType: operation_setting.QuotaDisplayTypeUSD,
			expected:         1,
		},
		{
			name:             "unit price, group ratio, and discount all apply",
			amount:           10,
			group:            "vip",
			quotaDisplayType: operation_setting.QuotaDisplayTypeUSD,
			expected:         9.6, // 10 * 1.0 * 1.2 * 0.8
		},
		{
			name:             "tokens display converts quota to display units before pricing",
			amount:           float64(common.QuotaPerUnit * 3),
			group:            "default",
			quotaDisplayType: operation_setting.QuotaDisplayTypeTokens,
			expected:         3,
		},
		{
			name:             "non-positive discount falls back to no discount",
			amount:           20,
			group:            "default",
			quotaDisplayType: operation_setting.QuotaDisplayTypeUSD,
			expected:         20,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			operation_setting.GetGeneralSetting().QuotaDisplayType = tc.quotaDisplayType
			actual := getGmpayPayMoney(tc.amount, tc.group)
			require.InDelta(t, tc.expected, actual, 0.000001)
		})
	}
}
