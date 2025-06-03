package app

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/remitlane/remitlane/internal/config"
	"github.com/remitlane/remitlane/internal/httpx"
	"github.com/remitlane/remitlane/internal/providers"
	"github.com/remitlane/remitlane/internal/providers/momobridge"
	momosandbox "github.com/remitlane/remitlane/internal/providers/momobridge/sandbox"
	"github.com/remitlane/remitlane/internal/providers/swiftpay"
	swiftsandbox "github.com/remitlane/remitlane/internal/providers/swiftpay/sandbox"
)

type providerSet struct {
	registry        providers.Registry
	swiftpaySandbox *swiftsandbox.Server
	momoSandbox     *momosandbox.Server
}

// buildProviders creates the provider clients and, in sandbox mode, the fake
// provider servers they talk to (mounted under /_sandbox).
func buildProviders(cfg config.Config, log *slog.Logger) providerSet {
	var ps providerSet
	if cfg.Swiftpay.Mode == "sandbox" {
		ps.swiftpaySandbox = swiftsandbox.New(swiftsandbox.Config{
			APIKey: cfg.Swiftpay.APIKey, WebhookSecret: cfg.Swiftpay.WebhookSecret,
			CallbackURL: cfg.BaseURL + "/webhooks/swiftpay", WebhookVersion: cfg.Swiftpay.APIVersion,
			Delay: cfg.Swiftpay.SandboxDelay, HoldDelay: cfg.Swiftpay.SandboxHold, ReviewDelay: cfg.Swiftpay.ReviewDelay,
			ReviewThresholdNGN: cfg.Swiftpay.ReviewThreshold,
		}, log)
	}
	if cfg.MoMo.Mode == "sandbox" {
		ps.momoSandbox = momosandbox.New(momosandbox.Config{
			APIKey: cfg.MoMo.APIKey, WebhookSecret: cfg.MoMo.WebhookSecret,
			CallbackURL: cfg.BaseURL + "/webhooks/momobridge", Delay: cfg.MoMo.SandboxDelay,
		}, log)
	}
	ps.registry = providers.Registry{
		providers.Swiftpay:   swiftpay.New(cfg.Swiftpay.BaseURL, cfg.Swiftpay.APIKey, newClient(cfg.Swiftpay.Timeout)),
		providers.MoMoBridge: momobridge.New(cfg.MoMo.BaseURL, cfg.MoMo.APIKey, newClient(cfg.MoMo.Timeout)),
	}
	return ps
}

func newClient(timeout time.Duration) *http.Client { return httpx.NewClient(timeout) }
