package setting

// GM Pay is a crypto top-up gateway configured like Epay: an admin-set
// domain, a merchant PID, and a secret key used to HMAC-sign every request
// and to verify the notify_url webhook.
var (
	GmpayEnabled  bool
	GmpayDomain   string
	GmpayPid      string
	GmpaySecret   string
	GmpayCurrency string = "USD"
	// GmpayUnitPrice is USD-to-USD (1.0 by default), like WaffoUnitPrice:
	// GM Pay settles in USD-denominated stablecoins, not local currency, so
	// this must not reuse the Epay Price ratio (local currency per USD).
	GmpayUnitPrice float64 = 1.0
	GmpayMinTopUp  int     = 1
	GmpayNotifyUrl string
)
