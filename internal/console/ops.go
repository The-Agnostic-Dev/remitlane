package console

import (
	"net/http"
	"slices"
	"time"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/providers/momobridge/sandbox"
	swiftsandbox "github.com/remitlane/remitlane/internal/providers/swiftpay/sandbox"
	"github.com/remitlane/remitlane/internal/store"
)

func (c *Console) ledger(w http.ResponseWriter, r *http.Request) {
	account := r.URL.Query().Get("account")
	txns, err := c.store.RecentLedger(r.Context(), account, 50)
	if err != nil {
		c.fail(w, r, err)
		return
	}
	c.renderTitled(w, r, "ledger", "Ledger", map[string]any{"Txns": txns, "Account": account})
}

type providersPage struct {
	Health []store.ProviderStats
	Events map[string][]db.ProviderEvent
}

func (c *Console) providersPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	health, err := c.store.ProviderHealth(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		c.fail(w, r, err)
		return
	}
	data := providersPage{Health: health, Events: map[string][]db.ProviderEvent{}}
	for _, p := range []string{"swiftpay", "momobridge"} {
		ev, err := c.store.Q.ListProviderEvents(ctx, db.ListProviderEventsParams{Provider: p, Limit: 15})
		if err != nil {
			c.fail(w, r, err)
			return
		}
		data.Events[p] = ev
	}
	c.renderTitled(w, r, "providers", "Providers", data)
}

type sandboxPage struct {
	Enabled           bool
	SwiftpayForced    string
	MoMoForced        string
	SwiftpayScenarios []string
	MoMoScenarios     []string
	Transfers         []swiftsandbox.Transfer
	Disbursements     []sandbox.Disbursement
}

func (c *Console) sandboxPage(w http.ResponseWriter, r *http.Request) {
	data := sandboxPage{Enabled: c.swiftpay != nil && c.momo != nil}
	if data.Enabled {
		data.SwiftpayForced, data.MoMoForced = c.swiftpay.Forced(), c.momo.Forced()
		data.SwiftpayScenarios, data.MoMoScenarios = swiftsandbox.Scenarios(), sandbox.Scenarios()
		data.Transfers = first(c.swiftpay.Transfers(""), 30)
		data.Disbursements = first(c.momo.Disbursements(""), 30)
	}
	c.renderTitled(w, r, "sandbox", "Provider sandbox", data)
}

func (c *Console) sandboxForce(w http.ResponseWriter, r *http.Request) {
	if c.swiftpay == nil || c.momo == nil {
		http.Error(w, "providers are not in sandbox mode", http.StatusConflict)
		return
	}
	provider, scenario := r.FormValue("provider"), r.FormValue("scenario")
	switch {
	case provider == "swiftpay" && (scenario == "" || slices.Contains(swiftsandbox.Scenarios(), scenario)):
		c.swiftpay.Force(scenario)
	case provider == "momobridge" && (scenario == "" || slices.Contains(sandbox.Scenarios(), scenario)):
		c.momo.Force(scenario)
	default:
		setFlash(w, "error", "Unknown provider or scenario.")
		http.Redirect(w, r, "/ops/sandbox", http.StatusSeeOther)
		return
	}
	msg := "The next " + provider + " payout will follow the normal sandbox rules."
	if scenario != "" {
		msg = "The next " + provider + " payout will be forced to: " + scenario + "."
	}
	setFlash(w, "ok", msg)
	http.Redirect(w, r, "/ops/sandbox", http.StatusSeeOther)
}

func first[T any](s []T, n int) []T {
	if len(s) > n {
		return s[:n]
	}
	return s
}
