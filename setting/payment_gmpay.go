package setting

// GM Pay is a crypto top-up gateway configured like Epay: an admin-set
// domain, a merchant PID, and a secret key used to HMAC-sign every request
// and to verify the notify_url webhook.
var (
	GmpayEnabled   bool
	GmpayDomain    string
	GmpayPid       string
	GmpaySecret    string
	GmpayCurrency  string = "USD"
	GmpayMinTopUp  int    = 1
	GmpayNotifyUrl string
)
