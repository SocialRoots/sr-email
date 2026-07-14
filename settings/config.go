package settings

import (
	"os"
	"time"
)

var Env = os.Getenv("ENV")
var ApiPort = os.Getenv("API_PORT")

var MailgunAPIKey = os.Getenv("MAILGUN_API_KEY")
var ResponsesServiceURL = os.Getenv("ROOTSHOOT_RESPONSES_SERVICE")
var InternalToken = os.Getenv("SR_INTERNAL_TOKEN")

var EmailStoreDir = func() string {
	if d := os.Getenv("EMAIL_STORE_DIR"); d != "" {
		return d
	}
	return "./data/emails"
}()

var CronInterval = func() time.Duration {
	if s := os.Getenv("CRON_INTERVAL"); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			return d
		}
	}
	return 30 * time.Second
}()