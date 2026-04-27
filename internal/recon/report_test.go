package recon

import (
	"bytes"
	"encoding/csv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/remitlane/remitlane/internal/store"
)

func rows() []store.ReconRow {
	at := time.Date(2026, 11, 5, 9, 30, 0, 0, time.UTC)
	return []store.ReconRow{
		{PayoutID: "po_1", PartnerName: "Kobopay", Status: "completed", DestCurrency: "NGN", DestMinor: 15169000, SourceCur: "USD", SourceMinor: 10000, FeeMinor: 100, USDMinor: 10000, CreatedAt: at, CompletedAt: pgtype.Timestamptz{Time: at.Add(2 * time.Minute), Valid: true}},
		{PayoutID: "po_2", PartnerName: "Duka Market", Status: "completed", DestCurrency: "UGX", DestMinor: 1850000, SourceCur: "USD", SourceMinor: 51043, FeeMinor: 200, USDMinor: 51043, CreatedAt: at},
		{PayoutID: "po_3", PartnerName: "Duka Market", Status: "failed", DestCurrency: "KES", DestMinor: 500000, SourceCur: "USD", SourceMinor: 3900, FeeMinor: 75, USDMinor: 3900, CreatedAt: at},
	}
}

func TestSummariseSkipsFailed(t *testing.T) {
	r := summarise("2026-11-05", rows())
	if r.ByStatus["failed"] != 1 || r.ByStatus["completed"] != 2 {
		t.Errorf("by status %v", r.ByStatus)
	}
	if len(r.Totals) != 2 {
		t.Fatalf("totals %v", r.Totals)
	}
	if r.USDMinor != 61043 || r.FeesMinor != 300 {
		t.Errorf("usd %d fees %d", r.USDMinor, r.FeesMinor)
	}
}

func TestWriteCSV(t *testing.T) {
	loc, _ := time.LoadLocation("Africa/Lagos")
	var buf bytes.Buffer
	if err := WriteCSV(&buf, summarise("2026-11-05", rows()), loc); err != nil {
		t.Fatal(err)
	}
	recs, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1+3+2 {
		t.Fatalf("got %d records", len(recs))
	}
	if recs[2][8] != "1850000" {
		t.Errorf("UGX amount rendered as %q", recs[2][8])
	}
	if recs[1][13] != "2026-11-05 10:30:00" {
		t.Errorf("created_at_wat = %q", recs[1][13])
	}
}
