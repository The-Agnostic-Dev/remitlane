// Package swiftpay is the client for Swiftpay, our bank transfer rail in
// Nigeria, Kenya, Ghana and Uganda.
package swiftpay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/remitlane/remitlane/internal/httpx"
	"github.com/remitlane/remitlane/internal/providers"
)

// Client calls the Swiftpay transfers API (v2).
type Client struct {
	base string
	key  string
	http *http.Client
}

// New returns a client for the API at base.
func New(base, key string, hc *http.Client) *Client {
	return &Client{base: strings.TrimRight(base, "/"), key: key, http: hc}
}

// Name implements providers.Provider.
func (c *Client) Name() string { return providers.Swiftpay }

type transferRequest struct {
	Reference     string `json:"reference"`
	Amount        string `json:"amount"`
	Currency      string `json:"currency"`
	Country       string `json:"country"`
	BankCode      string `json:"bank_code"`
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	Narration     string `json:"narration,omitempty"`
}

// Transfer is a transfer as Swiftpay returns it.
type Transfer struct {
	ID        string `json:"id"`
	Reference string `json:"reference"`
	Status    string `json:"status"`
	Amount    string `json:"amount"`
	Currency  string `json:"currency"`
	Reason    string `json:"reason,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type envelope struct {
	Transfer Transfer `json:"transfer"`
}

type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Send creates a transfer.
func (c *Client) Send(ctx context.Context, r providers.TransferRequest) (providers.Result, error) {
	body, err := json.Marshal(transferRequest{
		Reference: r.PayoutID, Amount: r.Amount.Decimal(), Currency: string(r.Amount.Currency),
		Country: r.Country, BankCode: r.BankCode, AccountNumber: r.AccountNumber,
		AccountName: r.RecipientName, Narration: r.Narration,
	})
	if err != nil {
		return providers.Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v2/transfers", bytes.NewReader(body))
	if err != nil {
		return providers.Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req)
}

// Status fetches a transfer by Swiftpay id.
func (c *Client) Status(ctx context.Context, providerRef string) (providers.Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/v2/transfers/"+url.PathEscape(providerRef), nil)
	if err != nil {
		return providers.Result{}, err
	}
	return c.do(req)
}

func (c *Client) do(req *http.Request) (providers.Result, error) {
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return providers.Result{}, fmt.Errorf("swiftpay: %s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	if err := httpx.CheckResponse(resp); err != nil {
		return providers.Result{}, c.classify(err)
	}
	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return providers.Result{}, fmt.Errorf("swiftpay: decode response: %w", err)
	}
	return result(env.Transfer), nil
}

// classify turns 4xx answers with a Swiftpay error code into rejections;
// everything else is worth retrying.
func (c *Client) classify(err error) error {
	var se *httpx.StatusError
	if !errors.As(err, &se) || se.Temporary() {
		return fmt.Errorf("swiftpay: %w", err)
	}
	var ae apiError
	if json.Unmarshal([]byte(se.Body), &ae) != nil || ae.Error.Code == "" {
		ae.Error.Code, ae.Error.Message = fmt.Sprintf("HTTP_%d", se.Code), se.Body
	}
	return &providers.RejectedError{Provider: providers.Swiftpay, Code: ae.Error.Code, Message: ae.Error.Message}
}

func result(t Transfer) providers.Result {
	st := MapV2(t.Status)
	res := providers.Result{ProviderRef: t.ID, RawStatus: t.Status, Status: st}
	if t.Reason != "" {
		res.FailureCode, res.FailureMessage = t.Reason, ReasonText(t.Reason)
	}
	return res
}
