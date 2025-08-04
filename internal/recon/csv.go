package recon

import (
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"github.com/remitlane/remitlane/internal/money"
)

var header = []string{
	"payout_id", "partner", "reference", "provider", "provider_reference", "status", "channel",
	"destination_currency", "destination_amount", "source_currency", "source_amount", "fee",
	"usd_value", "created_at_wat", "completed_at_wat",
}

// WriteCSV writes the report as CSV. Amounts are plain decimals in each
// currency's own precision (UGX has none), times are Lagos time.
func WriteCSV(w io.Writer, r Report, loc *time.Location) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, row := range r.Rows {
		completed := ""
		if row.CompletedAt.Valid {
			completed = row.CompletedAt.Time.In(loc).Format("2006-01-02 15:04:05")
		}
		dst, src := money.Currency(row.DestCurrency), money.Currency(row.SourceCur)
		if err := cw.Write([]string{
			row.PayoutID, row.PartnerName, row.Reference, row.Provider, row.ProviderRef, row.Status, row.Channel,
			string(dst), money.MinorToDecimal(row.DestMinor, dst), string(src), money.MinorToDecimal(row.SourceMinor, src),
			money.MinorToDecimal(row.FeeMinor, src), money.MinorToDecimal(row.USDMinor, money.USD),
			row.CreatedAt.In(loc).Format("2006-01-02 15:04:05"), completed,
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// Filename is the download name for a report.
func Filename(date string) string { return "remitlane-reconciliation-" + date + ".csv" }
