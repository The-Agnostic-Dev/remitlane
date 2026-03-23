package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/httpx"
	"github.com/remitlane/remitlane/internal/store"
)

// IdempotencyHeader lets partners retry a POST safely. A replay returns the
// first response unchanged; reusing a key with a different body is refused.
const IdempotencyHeader = "Idempotency-Key"

type captureWriter struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
}

func (c *captureWriter) WriteHeader(code int) {
	c.status = code
	c.ResponseWriter.WriteHeader(code)
}

func (c *captureWriter) Write(b []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	c.buf.Write(b)
	return c.ResponseWriter.Write(b)
}

func (a *API) idempotent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get(IdempotencyHeader)
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}
		if len(key) > 255 {
			writeError(w, r, http.StatusBadRequest, "validation_error", "Idempotency-Key must be at most 255 characters")
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, httpx.MaxBody))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "validation_error", "cannot read request body")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		sum := sha256.Sum256(append([]byte(r.Method+" "+r.URL.Path+"\n"), body...))
		hash := hex.EncodeToString(sum[:])
		partner := partnerFrom(r.Context())

		prev, err := a.store.Q.GetIdempotency(r.Context(), db.GetIdempotencyParams{PartnerID: partner.ID, Key: key})
		switch {
		case err == nil:
			if prev.RequestHash != hash {
				writeError(w, r, http.StatusUnprocessableEntity, "idempotency_key_reused",
					"this Idempotency-Key was already used with a different request")
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Idempotent-Replayed", "true")
			w.WriteHeader(int(prev.ResponseStatus))
			_, _ = w.Write(prev.ResponseBody)
			return
		case !store.IsNotFound(err):
			internalError(w, r, a.log, err)
			return
		}

		cw := &captureWriter{ResponseWriter: w}
		next.ServeHTTP(cw, r)
		if cw.status >= 500 || cw.status == 0 {
			return
		}
		if err := a.store.Q.SaveIdempotency(r.Context(), db.SaveIdempotencyParams{
			PartnerID: partner.ID, Key: key, RequestHash: hash,
			ResponseStatus: int32(cw.status), ResponseBody: cw.buf.Bytes(),
		}); err != nil {
			a.log.Error("save idempotency key", "partner_id", partner.ID, "err", err)
		}
	})
}
