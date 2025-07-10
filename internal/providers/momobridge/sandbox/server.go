// Package sandbox is a local stand-in for MoMoBridge, selected with
// MOMOBRIDGE_MODE=sandbox. Like the real service it answers 201 with a
// PENDING disbursement and posts a signed callback when it settles.
package sandbox

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/remitlane/remitlane/internal/providers/momobridge"
	"github.com/remitlane/remitlane/internal/sig"
)

// Config tunes the sandbox.
type Config struct {
	APIKey        string
	WebhookSecret string
	CallbackURL   string
	Delay         time.Duration
}

// Disbursement is the sandbox's record.
type Disbursement struct {
	momobridge.Disbursement
	Country   string
	Scenario  string
	CreatedAt time.Time
	Callbacks []Callback
}

// Callback is one callback attempt.
type Callback struct {
	Status string
	Code   int
	Error  string
	At     time.Time
}

// Server is the fake MoMoBridge.
type Server struct {
	cfg  Config
	log  *slog.Logger
	http *http.Client

	mu     sync.Mutex
	items  map[string]*Disbursement
	byKey  map[string]string
	forced string
}

// New returns a sandbox server.
func New(cfg Config, log *slog.Logger) *Server {
	return &Server{
		cfg: cfg, log: log.With("component", "momobridge-sandbox"),
		http:  &http.Client{Timeout: 5 * time.Second},
		items: map[string]*Disbursement{}, byKey: map[string]string{},
	}
}

// Scenarios lists the outcomes the console can force.
func Scenarios() []string { return []string{"successful", "failed", "silent"} }

// Routes mounts the fake API.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Api-Key") != s.cfg.APIKey {
				writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "bad api key")
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	r.Post("/v1/disbursements", s.create)
	r.Get("/v1/disbursements/{id}", s.get)
	return r
}

type createBody struct {
	ExternalID string `json:"external_id"`
	MSISDN     string `json:"msisdn"`
	Amount     string `json:"amount"`
	Currency   string `json:"currency"`
	Country    string `json:"country"`
	PayeeName  string `json:"payee_name"`
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var b createBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if msg := validate(b); msg != "" {
		code := "INVALID_REQUEST"
		if strings.Contains(msg, "whole") {
			code = "INVALID_AMOUNT"
		}
		writeErr(w, http.StatusUnprocessableEntity, code, msg)
		return
	}
	key := r.Header.Get("Idempotency-Key")
	s.mu.Lock()
	if id, ok := s.byKey[key]; ok && key != "" {
		d := *s.items[id]
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, d.Disbursement)
		return
	}
	d := &Disbursement{
		Disbursement: momobridge.Disbursement{
			ID: newID(), ExternalID: b.ExternalID, Status: "PENDING", Amount: b.Amount,
			Currency: strings.ToUpper(b.Currency), MSISDN: b.MSISDN, PayeeName: b.PayeeName,
		},
		Country: b.Country, CreatedAt: time.Now().UTC(),
	}
	d.Scenario = s.scenarioFor(d)
	s.items[d.ID] = d
	if key != "" {
		s.byKey[key] = d.ID
	}
	snap := *d
	s.mu.Unlock()

	s.log.Info("disbursement created", "id", d.ID, "external_id", d.ExternalID, "scenario", d.Scenario)
	time.AfterFunc(s.cfg.Delay, func() { s.settle(snap.ID) })
	writeJSON(w, http.StatusCreated, snap.Disbursement)
}

// validate mirrors MoMoBridge's request checks. M-Pesa and MTN Uganda take
// whole units only.
func validate(b createBody) string {
	if b.ExternalID == "" || b.MSISDN == "" || b.Amount == "" {
		return "external_id, msisdn and amount are required"
	}
	amt, ok := new(big.Rat).SetString(b.Amount)
	if !ok || amt.Sign() <= 0 {
		return "amount must be a positive decimal"
	}
	switch strings.ToUpper(b.Currency) {
	case "KES", "UGX":
		if !amt.IsInt() {
			return fmt.Sprintf("%s amounts must be whole shillings", strings.ToUpper(b.Currency))
		}
	case "GHS":
	default:
		return "currency must be KES, GHS or UGX"
	}
	return ""
}

func (s *Server) scenarioFor(d *Disbursement) string {
	if s.forced != "" {
		f := s.forced
		s.forced = ""
		return f
	}
	switch {
	case strings.HasSuffix(d.MSISDN, "0000"):
		return "failed"
	case strings.HasSuffix(d.MSISDN, "5555"):
		return "silent"
	}
	return "successful"
}

func (s *Server) settle(id string) {
	s.mu.Lock()
	d, ok := s.items[id]
	if !ok {
		s.mu.Unlock()
		return
	}
	switch d.Scenario {
	case "failed":
		d.Status, d.Reason = "FAILED", "SUBSCRIBER_NOT_FOUND"
	default:
		d.Status = "SUCCESSFUL"
	}
	snap := *d
	s.mu.Unlock()
	if snap.Scenario == "silent" {
		return
	}
	s.callback(snap)
}

func (s *Server) callback(d Disbursement) {
	cb := momobridge.Callback{
		DisbursementID: d.ID, ExternalID: d.ExternalID, Status: d.Status, Amount: d.Amount,
		Currency: d.Currency, MSISDN: d.MSISDN, PayeeName: d.PayeeName, Reason: d.Reason,
		CompletedAt: time.Now().UTC().Format(time.RFC3339),
	}
	// MoMoBridge sends raw UTF-8 and does not escape HTML characters.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(cb); err != nil {
		return
	}
	body := bytes.TrimRight(buf.Bytes(), "\n")
	signature := sig.Base64(s.cfg.WebhookSecret, body)
	for attempt := 1; attempt <= 3; attempt++ {
		code, err := s.post(body, signature)
		s.record(d.ID, Callback{Status: d.Status, Code: code, Error: errText(err), At: time.Now()})
		if err == nil {
			return
		}
		time.Sleep(2 * time.Second)
	}
}

func (s *Server) post(body []byte, signature string) (int, error) {
	req, err := http.NewRequest(http.MethodPost, s.cfg.CallbackURL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set(momobridge.SignatureHeader, signature)
	resp, err := s.http.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return resp.StatusCode, fmt.Errorf("callback answered %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

func (s *Server) record(id string, c Callback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.items[id]; ok {
		d.Callbacks = append(d.Callbacks, c)
	}
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	d, ok := s.items[chi.URLParam(r, "id")]
	var snap Disbursement
	if ok {
		snap = *d
	}
	s.mu.Unlock()
	if !ok {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "no such disbursement")
		return
	}
	writeJSON(w, http.StatusOK, snap.Disbursement)
}

// Disbursements lists disbursements, newest first, optionally for one
// external id (our payout id).
func (s *Server) Disbursements(externalID string) []Disbursement {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Disbursement, 0, len(s.items))
	for _, d := range s.items {
		if externalID == "" || d.ExternalID == externalID {
			c := *d
			c.Callbacks = append([]Callback(nil), d.Callbacks...)
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// Force makes the next disbursement follow a scenario.
func (s *Server) Force(scenario string) {
	s.mu.Lock()
	s.forced = scenario
	s.mu.Unlock()
}

// Forced is the scenario waiting for the next disbursement, if any.
func (s *Server) Forced() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.forced
}

func newID() string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	return "MMB-" + strings.ToUpper(hex.EncodeToString(b))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": code, "message": msg})
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
