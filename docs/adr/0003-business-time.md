# ADR 0003: business time is Africa/Lagos

Date: 2026-03-04. Status: accepted.

## Context

RL-131: Duka Market's daily limit reset at 01:00 Lagos time (02:00 in Nairobi) because the limit window was a UTC
day. The reconciliation report already used Lagos dates, so the two disagreed about which day a late-night payout
belonged to.

## Decision

All "per day" rules use the business time zone, `BUSINESS_TZ` (default `Africa/Lagos`, WAT, UTC+1, no daylight
saving), whatever zone the server, the database session or the partner is in:

- daily payout limits (`GET /v1/limits` returns the window),
- the reconciliation report and its nightly run,
- dates shown in the console.

Code asks `internal/clock` for day boundaries (`DayBounds`, `DateBounds`). Postgres sessions run in UTC
(`store.Open` sets it), so queries that select a business day take their bounds from the clock.

## Consequences

- Kenyan and Ghanaian partners see their limit reset at 01:00 (Accra) or 02:00 (Nairobi) local time. This is
  documented in the API guide and on the limits page.
- The distroless image has no zoneinfo; `cmd/remitlane` embeds `time/tzdata`.
