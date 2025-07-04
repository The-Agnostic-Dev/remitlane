// Package sandbox is a local stand-in for the Swiftpay API, selected with
// SWIFTPAY_MODE=sandbox. It keeps transfers in memory and calls our webhook
// endpoint back over HTTP, signed like the real service.
package sandbox

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/remitlane/remitlane/internal/providers/swiftpay"
)

// Config tunes the sandbox.
type Config struct {
	APIKey             string
	WebhookSecret      string
	CallbackURL        string
	WebhookVersion     string
	Delay              time.Duration
	HoldDelay          time.Duration
	ReviewDelay        time.Duration
	ReviewThresholdNGN int64
}

// Transfer is the sandbox's record of a transfer.
type Transfer struct {
	swiftpay.Transfer
	Country       string
	BankCode      string
	AccountNumber string
	AccountName   string
	Scenario      string
	Callbacks     []Callback
}

// Callback is one webhook the sandbox sent.
type Callback struct {
	Status string
	Code   int
	Error  string
	At     time.Time
}

// Server is the fake Swiftpay.
type Server struct {
	cfg  Config
	log  *slog.Logger
	http *http.Client

	mu        sync.Mutex
	transfers map[string]*Transfer
	byKey     map[string]string
	forced    string
}

// New returns a sandbox server.
func New(cfg Config, log *slog.Logger) *Server {
	return &Server{
		cfg: cfg, log: log.With("component", "swiftpay-sandbox"),
		http:      &http.Client{Timeout: 5 * time.Second},
		transfers: map[string]*Transfer{},
		byKey:     map[string]string{},
	}
}

// Routes mounts the fake API.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(s.auth)
	r.Post("/v2/transfers", s.create)
	r.Get("/v2/transfers/{id}", s.get)
	return r
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+s.cfg.APIKey {
			writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid API key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type createBody struct {
	Reference     string `json:"reference"`
	Amount        string `json:"amount"`
	Currency      string `json:"currency"`
	Country       string `json:"country"`
	BankCode      string `json:"bank_code"`
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var b createBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if b.Reference == "" || b.AccountNumber == "" || b.Amount == "" {
		writeErr(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "reference, amount and account_number are required")
		return
	}
	key := r.Header.Get("Idempotency-Key")

	s.mu.Lock()
	if id, ok := s.byKey[key]; ok && key != "" {
		t := *s.transfers[id]
		s.mu.Unlock()
		writeJSON(w, statusFor(t.Status), map[string]any{"transfer": t.Transfer})
		return
	}
	now := time.Now().UTC()
	t := &Transfer{
		Transfer: swiftpay.Transfer{
			ID: newID(), Reference: b.Reference, Amount: b.Amount, Currency: strings.ToUpper(b.Currency),
			Status: "PROCESSING", CreatedAt: now.Format(time.RFC3339), UpdatedAt: now.Format(time.RFC3339),
		},
		Country: b.Country, BankCode: b.BankCode, AccountNumber: b.AccountNumber, AccountName: b.AccountName,
	}
	t.Scenario = s.scenarioFor(t)
	if t.Scenario == scenarioReview {
		t.Status = "QUEUED"
	}
	s.transfers[t.ID] = t
	if key != "" {
		s.byKey[key] = t.ID
	}
	snapshot := *t
	s.mu.Unlock()

	s.log.Info("transfer created", "id", t.ID, "reference", t.Reference, "scenario", t.Scenario)
	s.schedule(t.ID, t.Scenario)
	writeJSON(w, statusFor(snapshot.Status), map[string]any{"transfer": snapshot.Transfer})
}

// statusFor is the HTTP status Swiftpay v2 answers a create with: queued
// transfers (manual review) are 202 Accepted.
func statusFor(status string) int {
	if status == "QUEUED" {
		return http.StatusAccepted
	}
	return http.StatusOK
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	t, ok := s.transfers[chi.URLParam(r, "id")]
	var snap Transfer
	if ok {
		snap = *t
	}
	s.mu.Unlock()
	if !ok {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "no such transfer")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"transfer": snap.Transfer})
}

// Transfers lists transfers, newest first, optionally for one reference.
func (s *Server) Transfers(reference string) []Transfer {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Transfer, 0, len(s.transfers))
	for _, t := range s.transfers {
		if reference == "" || t.Reference == reference {
			c := *t
			c.Callbacks = append([]Callback(nil), t.Callbacks...)
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt > out[j].CreatedAt || (out[i].CreatedAt == out[j].CreatedAt && out[i].ID > out[j].ID)
	})
	return out
}

// Force makes the next transfer follow a scenario regardless of its details.
func (s *Server) Force(scenario string) {
	s.mu.Lock()
	s.forced = scenario
	s.mu.Unlock()
}

// Forced is the scenario waiting for the next transfer, if any.
func (s *Server) Forced() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.forced
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "SWP2_" + strings.ToUpper(hex.EncodeToString(b))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}
