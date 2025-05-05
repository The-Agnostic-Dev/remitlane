# ADR 0002: double-entry ledger in NUMERIC(20,4)

Date: 2025-05-06. Status: accepted.

## Context

Partner balances were a single `balance_minor` column updated in place. Finance could not explain a $412 difference
on Kobopay in April 2025, and refunds of reversed payouts were done by hand in SQL.

## Decision

Every movement of money is a ledger transaction with two or more entries that sum to zero per currency. Accounts are
identified by code:

| Code | Meaning |
| --- | --- |
| `partner:<id>:available:<CUR>` | what the partner can spend |
| `partner:<id>:held:<CUR>` | reserved for payouts in flight |
| `provider:<name>:clearing:<CUR>` | owed to a payout provider |
| `revenue:fees:<CUR>` | our fees |
| `external:funding:<CUR>` | the outside world (partner top-ups) |

Postings: create holds (available to held), complete settles (held to clearing plus fees), fail releases (held back
to available), reversal refunds (clearing and fees back to available). Amounts are stored as NUMERIC(20,4) in major
units so that FX values with more precision than a minor unit can be recorded later; the API converts to and from
minor units at the edge with `internal/money`.

## Consequences

- Balances are `SUM(amount)` per account; the console reads them live.
- Spending locks the partner's available account row (`SELECT ... FOR UPDATE`) so two payouts cannot both pass the
  funds check.
- Never read NUMERIC as float64. Cast to text and parse with `money.DecimalToMinor`.
