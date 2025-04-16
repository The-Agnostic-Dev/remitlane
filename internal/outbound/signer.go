package outbound

import (
	"fmt"
	"strconv"
	"time"

	"github.com/remitlane/remitlane/internal/sig"
)

// SignatureHeader is the header partners verify.
const SignatureHeader = "Remitlane-Signature"

// Sign returns "t=<unix>,v1=<hex hmac of '<unix>.<body>'>". Partners should
// reject timestamps older than five minutes (docs/api.md, Webhooks).
func Sign(secret string, ts time.Time, body []byte) string {
	t := strconv.FormatInt(ts.UnixMilli(), 10)
	mac := sig.Hex(secret, append([]byte(t+"."), body...))
	return fmt.Sprintf("t=%s,v1=%s", t, mac)
}
