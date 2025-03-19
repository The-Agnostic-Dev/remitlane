// Package ledger posts double-entry transactions. Each transaction's entries
// must sum to zero in every currency; Post refuses anything else.
package ledger

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/money"
)

// Account kinds.
const (
	KindAvailable = "available"
	KindHeld      = "held"
	KindClearing  = "provider_clearing"
	KindFees      = "fee_revenue"
	KindFunding   = "funding"
)

// Account identifies a ledger account by its code.
type Account struct {
	Code      string
	Kind      string
	PartnerID string
	Currency  money.Currency
}

// Available is a partner's spendable balance.
func Available(partnerID string, c money.Currency) Account {
	return Account{fmt.Sprintf("partner:%s:available:%s", partnerID, c), KindAvailable, partnerID, c}
}

// Held is money reserved for a partner's payouts in flight.
func Held(partnerID string, c money.Currency) Account {
	return Account{fmt.Sprintf("partner:%s:held:%s", partnerID, c), KindHeld, partnerID, c}
}

// Clearing is what Remitlane owes a payout provider.
func Clearing(provider string, c money.Currency) Account {
	return Account{fmt.Sprintf("provider:%s:clearing:%s", provider, c), KindClearing, "", c}
}

// Fees is fee revenue.
func Fees(c money.Currency) Account {
	return Account{fmt.Sprintf("revenue:fees:%s", c), KindFees, "", c}
}

// Funding is the outside world: partner top-ups arrive from here.
func Funding(c money.Currency) Account {
	return Account{fmt.Sprintf("external:funding:%s", c), KindFunding, "", c}
}

// Entry is one leg of a transaction, in minor units (positive increases the
// account's balance).
type Entry struct {
	Account Account
	Minor   int64
}

// ErrUnbalanced is returned when entries do not sum to zero.
var ErrUnbalanced = errors.New("ledger: entries do not balance")

// Post writes a transaction.
func Post(ctx context.Context, q *db.Queries, kind, payoutID, memo string, entries ...Entry) (int64, error) {
	sums := map[money.Currency]int64{}
	for _, e := range entries {
		sums[e.Account.Currency] += e.Minor
	}
	for c, s := range sums {
		if s != 0 {
			return 0, fmt.Errorf("%w: %s off by %d", ErrUnbalanced, c, s)
		}
	}
	txn, err := q.InsertTxn(ctx, db.InsertTxnParams{Kind: kind, PayoutID: text(payoutID), Memo: memo})
	if err != nil {
		return 0, fmt.Errorf("ledger: insert txn: %w", err)
	}
	for _, e := range entries {
		if e.Minor == 0 {
			continue
		}
		acct, err := q.UpsertAccount(ctx, db.UpsertAccountParams{
			Code: e.Account.Code, PartnerID: text(e.Account.PartnerID), Kind: e.Account.Kind,
			Currency: string(e.Account.Currency),
		})
		if err != nil {
			return 0, fmt.Errorf("ledger: account %s: %w", e.Account.Code, err)
		}
		if err := q.InsertEntry(ctx, db.InsertEntryParams{
			TxnID: txn.ID, AccountID: acct.ID, Amount: money.LedgerDecimal(e.Minor, e.Account.Currency),
		}); err != nil {
			return 0, fmt.Errorf("ledger: insert entry: %w", err)
		}
	}
	return txn.ID, nil
}

// LockedBalance locks an account row for the rest of the transaction and
// returns its balance in minor units. Used to serialise spending per partner.
func LockedBalance(ctx context.Context, tx pgx.Tx, a Account) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `SELECT id FROM ledger_accounts WHERE code = $1 FOR UPDATE`, a.Code).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var sum string
	if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(amount), 0)::text FROM ledger_entries WHERE account_id = $1`, id).Scan(&sum); err != nil {
		return 0, err
	}
	return money.DecimalToMinor(sum, a.Currency)
}


func text(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}
