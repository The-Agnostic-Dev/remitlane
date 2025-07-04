package sandbox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/remitlane/remitlane/internal/providers/swiftpay"
	"github.com/remitlane/remitlane/internal/sig"
)

// Scenarios. Account number endings pick one (documented in docs/sandbox.md).
const (
	scenarioComplete = "complete"
	scenarioFail     = "fail"
	scenarioHold     = "hold"
	scenarioReverse  = "reverse"
	scenarioSilent   = "silent"
	scenarioReview   = "review"
)

// Scenarios lists the outcomes the console can force.
func Scenarios() []string {
	return []string{scenarioComplete, scenarioFail, scenarioHold, scenarioReverse, scenarioSilent, scenarioReview}
}

// scenarioFor must be called with s.mu held.
func (s *Server) scenarioFor(t *Transfer) string {
	if s.forced != "" {
		f := s.forced
		s.forced = ""
		return f
	}
	switch {
	case strings.HasSuffix(t.AccountNumber, "0001"):
		return scenarioFail
	case strings.HasSuffix(t.AccountNumber, "0099"):
		return scenarioHold
	case strings.HasSuffix(t.AccountNumber, "0077"):
		return scenarioReverse
	case strings.HasSuffix(t.AccountNumber, "0055"):
		return scenarioSilent
	}
	if t.Currency == "NGN" && s.cfg.ReviewThresholdNGN > 0 {
		amt, ok := new(big.Rat).SetString(t.Amount)
		if ok && amt.Cmp(new(big.Rat).SetInt64(s.cfg.ReviewThresholdNGN)) >= 0 {
			return scenarioReview
		}
	}
	return scenarioComplete
}

// schedule plays a scenario out over time.
func (s *Server) schedule(id, scenario string) {
	d, hold := s.cfg.Delay, s.cfg.HoldDelay
	switch scenario {
	case scenarioFail:
		s.after(d, id, "FAILED", "INVALID_ACCOUNT", true)
	case scenarioHold:
		s.after(d, id, "ON_HOLD", "COMPLIANCE_REVIEW", true)
		s.after(d+hold, id, "COMPLETED", "", true)
	case scenarioReverse:
		s.after(d, id, "COMPLETED", "", true)
		s.after(d+hold, id, "REVERSED", "RETURNED_BY_BANK", true)
	case scenarioSilent:
		s.after(d, id, "COMPLETED", "", false)
	case scenarioReview:
		s.after(s.cfg.ReviewDelay, id, "COMPLETED", "", true)
	default:
		s.after(d, id, "COMPLETED", "", true)
	}
}

func (s *Server) after(d time.Duration, id, status, reason string, notify bool) {
	time.AfterFunc(d, func() {
		s.mu.Lock()
		t, ok := s.transfers[id]
		if !ok {
			s.mu.Unlock()
			return
		}
		t.Status, t.Reason = status, reason
		t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		snap := *t
		s.mu.Unlock()
		if notify {
			s.notify(snap)
		}
	})
}

// notify posts the webhook, retrying a few times like Swiftpay does.
func (s *Server) notify(t Transfer) {
	body, err := s.webhookBody(t)
	if err != nil {
		s.log.Error("build webhook", "err", err)
		return
	}
	signature := "sha256=" + sig.Hex(s.cfg.WebhookSecret, body)
	for attempt := 1; attempt <= 3; attempt++ {
		code, err := s.post(body, signature)
		s.record(t.ID, Callback{Status: t.Status, Code: code, Error: errText(err), At: time.Now()})
		if err == nil && code/100 == 2 {
			return
		}
		time.Sleep(2 * time.Second)
	}
}

func (s *Server) webhookBody(t Transfer) ([]byte, error) {
	if s.cfg.WebhookVersion == "v1" {
		return json.Marshal(swiftpay.WebhookV1{
			Type: "transfer.status", TransferID: t.ID, Ref: t.Reference,
			State: v1State(t.Status), Reason: t.Reason,
		})
	}
	return json.Marshal(swiftpay.WebhookV2{Event: "transfer.updated", Data: t.Transfer})
}

func v1State(v2 string) string {
	switch v2 {
	case "COMPLETED":
		return "successful"
	case "ON_HOLD", "QUEUED":
		return "pending"
	default:
		return strings.ToLower(v2)
	}
}

func (s *Server) post(body []byte, signature string) (int, error) {
	req, err := http.NewRequest(http.MethodPost, s.cfg.CallbackURL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Swiftpay-Webhooks/2.3")
	req.Header.Set(swiftpay.SignatureHeader, signature)
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
	if t, ok := s.transfers[id]; ok {
		t.Callbacks = append(t.Callbacks, c)
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
