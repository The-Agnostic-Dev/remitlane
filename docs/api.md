# Remitlane API

Base URL: `https://api.remitlane.com/v1` (sandbox: `https://sandbox.remitlane.com/v1`, or
`http://localhost:5121/v1` locally).

All requests are JSON and authenticated with your API key:

```
Authorization: Bearer rl_live_...
```

Amounts are **integers in minor units** of the currency (`amount_minor`). NGN, KES, GHS and USD have two decimal
places; UGX has none. `150000` means UGX 150,000 but NGN 1,500.00.

Timestamps are RFC 3339 in UTC, for example `2026-11-06T09:15:00Z`.

## Errors

```json
{
  "error": { "code": "invalid_recipient", "message": "233301234567 is not a GH mobile money number", "field": "recipient.msisdn" },
  "request_id": "req_6c1f0e2a9b7d4410"
}
```

| HTTP | Code | Meaning |
| --- | --- | --- |
| 400 | `validation_error` | malformed request or missing field |
| 401 | `unauthorized` | missing, unknown or revoked API key |
| 404 | `quote_not_found`, `payout_not_found`, `batch_not_found` | |
| 409 | `duplicate_reference` | a payout or batch with this `reference` exists |
| 422 | `quote_expired`, `quote_already_used` | request a new quote |
| 422 | `invalid_recipient` | see `field` |
| 422 | `corridor_not_supported`, `amount_out_of_range` | see `GET /corridors` |
| 422 | `insufficient_funds` | top up your balance |
| 422 | `daily_limit_exceeded` | your daily limit (USD) would be passed |
| 422 | `idempotency_key_reused` | the key was used with a different body |
| 429 | `rate_limited` | slow down; see `Retry-After` |

Quote `request_id` when you contact support.

## Idempotency

Send `Idempotency-Key: <your unique id>` on `POST /payouts` and `POST /payouts/bulk`. Retrying with the same key and
the same body returns the original response with `Idempotent-Replayed: true`, and never creates a second payout.

## Corridors

`GET /corridors` lists what you can send, with the spread (basis points taken off the mid-market rate), the flat fee
(minor units of the source currency) and the destination amount limits.

| From | To | Channel | Spread | Fee |
| --- | --- | --- | --- | --- |
| USD | NGN | bank | 150 bps | 100 (USD 1.00) |
| USD | KES | bank | 120 bps | 150 |
| USD | KES | mobile_money | 100 bps | 75 |
| USD | GHS | bank | 150 bps | 150 |
| USD | GHS | mobile_money | 120 bps | 75 |
| USD | UGX | bank | 200 bps | 200 |
| USD | UGX | mobile_money | 150 bps | 100 |
| NGN | NGN | bank | 0 | 5000 (NGN 50.00) |

## Quotes

`POST /quotes`

```json
{ "source_currency": "USD", "destination_currency": "KES", "channel": "mobile_money", "amount_minor": 2500, "amount_type": "source" }
```

`amount_type` is `source` (you fix what you spend; default) or `destination` (you fix what the recipient gets).

Response `201`:

```json
{
  "id": "qt_4b0d9e2c7a51f3e86d10",
  "object": "quote",
  "source_currency": "USD",
  "destination_currency": "KES",
  "channel": "mobile_money",
  "source_amount_minor": 2500,
  "destination_amount_minor": 320100,
  "fee_minor": 75,
  "total_debit_minor": 2575,
  "rate": "128.076300",
  "mid_rate": "129.370000",
  "spread_bps": 100,
  "expires_at": "2026-11-06T09:30:00Z",
  "created_at": "2026-11-06T09:15:00Z",
  "used": false
}
```

`rate` is `mid_rate` less the corridor spread: 129.37 × (1 − 0.0100) = 128.0763. The fee is charged on top
(`total_debit_minor`). A quote is valid for **15 minutes** and backs one payout. KES mobile money destinations are
whole shillings (M-Pesa does not accept cents), so the destination is rounded down to the shilling.

`GET /quotes/{id}` returns a quote.

## Payouts

`POST /payouts`

```json
{
  "quote_id": "qt_4b0d9e2c7a51f3e86d10",
  "reference": "ORD-88231",
  "narration": "Seller payout",
  "recipient": { "name": "Achieng Otieno", "country": "KE", "msisdn": "+254 712 345 678" }
}
```

Bank recipients send `bank_code` and `account_number` instead of `msisdn`:

| Country | `account_number` | `bank_code` |
| --- | --- | --- |
| NG | 10 digit NUBAN | 3 digit CBN code (or 6 digit) |
| KE | 6 to 16 digits | 2 or 3 digits |
| GH | 10 to 16 digits | 6 digit sort code |
| UG | 9 to 14 digits | SWIFT/BIC |

Mobile money numbers may be local (`0712345678`) or international (`+254712345678`); we store them in
international form without the plus.

Response `201` is the payout:

```json
{
  "id": "po_8f1c2a9e0d7b6c5a4e31",
  "object": "payout",
  "reference": "ORD-88231",
  "status": "pending",
  "channel": "mobile_money",
  "quote_id": "qt_4b0d9e2c7a51f3e86d10",
  "source_currency": "USD",
  "source_amount_minor": 2500,
  "fee_minor": 75,
  "destination_currency": "KES",
  "destination_amount_minor": 320100,
  "rate": "128.076300",
  "recipient": { "name": "Achieng Otieno", "country": "KE", "msisdn": "254712345678" },
  "created_at": "2026-11-06T09:15:20Z",
  "updated_at": "2026-11-06T09:15:20Z"
}
```

Statuses: `pending` → `submitted` / `processing` → `completed` | `failed`; `completed` → `reversed` when the
recipient's bank returns the funds (amount and fee are refunded). Failed payouts carry
`"failure": {"code": "...", "message": "..."}` and their funds go back to your available balance.

`GET /payouts/{id}` returns a payout.

`GET /payouts` lists your payouts, newest first. Query parameters: `limit` (1 to 100, default 20), `cursor`
(the `next_cursor` of the previous page), `status`, `reference`, `batch_id`.

```json
{ "object": "list", "data": [ ... ], "has_more": true, "next_cursor": "MjAyNi0xMS0wNlQwOToxNToyMC4xMjM0NTZa" }
```

## Bulk payouts

`POST /payouts/bulk` books up to 500 payouts in one request. Items are priced by **destination amount** at the
current rate (no quote needed). Either every item is accepted or none is; errors name the failing `item` (1-based).

```json
{
  "reference": "SETTLE-2026-11-06",
  "items": [
    {
      "reference": "SETTLE-2026-11-06-01",
      "source_currency": "USD",
      "destination_currency": "GHS",
      "channel": "bank",
      "amount_minor": 125000,
      "recipient": { "name": "Kwame Boateng", "country": "GH", "bank_code": "130100", "account_number": "1441002233019" }
    }
  ]
}
```

Response `201`: `{"id": "bat_...", "object": "batch", "reference", "item_count", "created_at", "payouts": [...]}`.
`GET /batches/{id}` returns it again.

## Balances and limits

`GET /balances`

```json
{ "object": "list", "data": [ { "currency": "USD", "available_minor": 4983200, "held_minor": 2575 } ] }
```

`held_minor` is money reserved for payouts in flight.

`GET /limits`

```json
{
  "object": "limits", "currency": "USD", "daily_limit_minor": 3000000, "used_today_minor": 27383,
  "remaining_today_minor": 2972617, "window_start": "2026-11-05T23:00:00Z", "window_end": "2026-11-06T23:00:00Z",
  "time_zone": "Africa/Lagos"
}
```

The daily limit is in USD and the day runs from midnight to midnight **Lagos time (WAT)**, whatever your own time
zone. Payouts in other currencies count at the mid rate; failed payouts do not count.

## Webhooks

We POST events to your endpoint:

```json
{ "id": "evt_1c9a...", "type": "payout.completed", "created_at": "2026-11-06T09:16:02Z", "data": { ...payout... } }
```

Types: `payout.processing`, `payout.completed`, `payout.failed`, `payout.reversed`.

Headers: `Remitlane-Event-Id`, `Remitlane-Event-Type`, `Remitlane-Delivery-Attempt` and
`Remitlane-Signature: t=<unix>,v1=<hex>`, where `v1` is HMAC-SHA256 with your endpoint secret over
`<t>.<raw body>`. Reject signatures older than five minutes.

Answer with any `2xx` once you have stored the event. Anything else (or no answer within 5 seconds) is retried up to
5 attempts in total with growing delays; `4xx` answers other than 408 and 429 are not retried. Events can arrive more
than once in rare cases: deduplicate on `id`.
