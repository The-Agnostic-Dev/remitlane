// Package momobridge is the client for MoMoBridge, our mobile money rail
// (M-Pesa and Airtel in Kenya, MTN, Telecel and AirtelTigo in Ghana, MTN and
// Airtel in Uganda).
package momobridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/remitlane/remitlane/internal/domain"
	"github.com/remitlane/remitlane/internal/providers"
)

// Client calls the MoMoBridge disbursements API.
type Client struct {
	base string
	key  string
	http *http.Client
}

// New returns a client.
func New(base, key string, hc *http.Client) *Client {
	return &Client{base: strings.TrimRight(base, "/"), key: key, http: hc}
}

// Name implements providers.Provider.
func (c *Client) Name() string { return providers.MoMoBridge }

// Disbursement is MoMoBridge's view of a payout.
type Disbursement struct {
	ID         string `json:"disbursement_id"`
	ExternalID string `json:"external_id"`
	Status     string `json:"status"`
	Amount     string `json:"amount"`
	Currency   string `json:"currency"`
	MSISDN     string `json:"msisdn"`
	PayeeName  string `json:"payee_name"`
	Reason     string `json:"reason,omitempty"`
}

type disburseRequest struct {
	ExternalID string `json:"external_id"`
	MSISDN     string `json:"msisdn"`
	Amount     string `json:"amount"`
	Currency   string `json:"currency"`
	Country    string `json:"country"`
	PayeeName  string `json:"payee_name"`
	Narration  string `json:"narration,omitempty"`
}

// Send creates a disbursement.
func (c *Client) Send(ctx context.Context, r providers.TransferRequest) (providers.Result, error) {
	body, err := json.Marshal(disburseRequest{
		ExternalID: r.PayoutID, MSISDN: r.MSISDN, Amount: r.Amount.Decimal(),
		Currency: string(r.Amount.Currency), Country: r.Country, PayeeName: r.RecipientName, Narration: r.Narration,
	})
	if err != nil {
		return providers.Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v1/disbursements", bytes.NewReader(body))
	if err != nil {
		return providers.Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", r.PayoutID)
	return c.do(req)
}

// Status fetches a disbursement.
func (c *Client) Status(ctx context.Context, providerRef string) (providers.Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/v1/disbursements/"+url.PathEscape(providerRef), nil)
	if err != nil {
		return providers.Result{}, err
	}
	return c.do(req)
}

func (c *Client) do(req *http.Request) (providers.Result, error) {
	req.Header.Set("X-Api-Key", c.key)
	resp, err := c.http.Do(req)
	if err != nil {
		return providers.Result{}, fmt.Errorf("momobridge: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode/100 != 2 {
		var e struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &e)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests && e.Error != "" {
			return providers.Result{}, &providers.RejectedError{Provider: providers.MoMoBridge, Code: e.Error, Message: e.Message}
		}
		return providers.Result{}, fmt.Errorf("momobridge: status %d: %s", resp.StatusCode, raw)
	}
	var d Disbursement
	if err := json.Unmarshal(raw, &d); err != nil {
		return providers.Result{}, fmt.Errorf("momobridge: decode: %w", err)
	}
	res := providers.Result{ProviderRef: d.ID, RawStatus: d.Status, Status: MapStatus(d.Status)}
	if d.Reason != "" {
		res.FailureCode, res.FailureMessage = d.Reason, strings.ReplaceAll(strings.ToLower(d.Reason), "_", " ")
	}
	return res, nil
}

// MapStatus maps a MoMoBridge status to a payout status.
func MapStatus(s string) domain.Status {
	switch strings.ToUpper(s) {
	case "PENDING", "SENT":
		return domain.Processing
	case "SUCCESSFUL":
		return domain.Completed
	case "REVERSED":
		return domain.Reversed
	default:
		return domain.Failed
	}
}
