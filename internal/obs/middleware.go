package obs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

type ctxKey struct{}

// RequestID returns the request id stored by the middleware.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "req_" + hex.EncodeToString(b)
}

type recorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *recorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Middleware tags each request with an id, logs it and recovers panics.
func Middleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-Id")
			if id == "" || len(id) > 64 {
				id = newID()
			}
			w.Header().Set("X-Request-Id", id)
			ctx := context.WithValue(r.Context(), ctxKey{}, id)
			rec := &recorder{ResponseWriter: w}
			start := time.Now()
			defer func() {
				if p := recover(); p != nil {
					log.Error("panic", "request_id", id, "panic", p, "stack", string(debug.Stack()))
					if rec.status == 0 {
						http.Error(rec, "internal error", http.StatusInternalServerError)
					}
				}
				if quiet(r.URL.Path) {
					return
				}
				log.Info("http",
					"request_id", id, "method", r.Method, "path", r.URL.Path,
					"status", rec.status, "bytes", rec.bytes, "ms", time.Since(start).Milliseconds())
			}()
			next.ServeHTTP(rec, r.WithContext(ctx))
		})
	}
}

func quiet(path string) bool {
	return path == "/healthz" || strings.HasPrefix(path, "/static/")
}
