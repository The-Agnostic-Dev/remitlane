package httpx

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// NewClient returns an HTTP client for calling partners and providers. Every
// outbound call must have a timeout: a provider that hangs must not hold a
// worker forever.
func NewClient(timeout time.Duration) *http.Client {
	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	return &http.Client{Timeout: timeout, Transport: tr}
}

// StatusError is a response with an unexpected status code.
type StatusError struct {
	Code int
	Body string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("unexpected status %d: %s", e.Code, e.Body)
}

// Temporary reports whether the call is worth retrying.
func (e *StatusError) Temporary() bool {
	return e.Code >= 500 || e.Code == http.StatusTooManyRequests || e.Code < 400
}

// CheckResponse returns a *StatusError unless the response is a success. The
// body is left unread on success.
func CheckResponse(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return &StatusError{Code: resp.StatusCode, Body: string(b)}
}
