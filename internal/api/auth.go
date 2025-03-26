package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/partners"
	"github.com/remitlane/remitlane/internal/store"
)

type partnerKey struct{}

func partnerFrom(ctx context.Context) db.Partner {
	p, _ := ctx.Value(partnerKey{}).(db.Partner)
	return p
}

// authenticate resolves the bearer API key to a partner.
func (a *API) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		key, ok := strings.CutPrefix(auth, "Bearer ")
		if !ok || strings.TrimSpace(key) == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="remitlane"`)
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "send your API key as: Authorization: Bearer <key>")
			return
		}
		k, err := a.store.Q.GetAPIKeyByHash(r.Context(), partners.HashKey(key))
		if store.IsNotFound(err) {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "unknown or revoked API key")
			return
		}
		if err != nil {
			internalError(w, r, a.log, err)
			return
		}
		if k.PartnerStatus != "active" {
			writeError(w, r, http.StatusForbidden, "partner_suspended", "this account is suspended; contact support@remitlane.com")
			return
		}
		p, err := a.store.Q.GetPartner(r.Context(), k.PartnerID)
		if err != nil {
			internalError(w, r, a.log, err)
			return
		}
		_ = a.store.Q.TouchAPIKey(r.Context(), k.ID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), partnerKey{}, p)))
	})
}
