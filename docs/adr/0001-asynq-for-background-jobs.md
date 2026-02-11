# ADR 0001: asynq on Redis for background jobs

Date: 2025-04-14. Status: accepted.

## Context

Payout dispatch, provider polling and partner webhooks all need retries with backoff, and must survive a deploy.
Early on we dispatched from a goroutine in the request handler; two deploys in March 2025 lost payouts that were
mid-dispatch and left their funds held.

## Decision

Use [asynq](https://github.com/hibiken/asynq) with Redis. The worker runs in the same binary and process as the API
for now (one container to operate). Tasks carry ids only, never payout state; handlers reload from Postgres.

- `payout:dispatch` on the `critical` queue, task id `dispatch:<payout id>` so a payout is never queued twice.
- `webhook:deliver` on `default`, retried with linear backoff up to `WEBHOOK_MAX_ATTEMPTS`.
- `recon:daily` and the poller on `low`, registered with the asynq scheduler in the business time zone.

## Consequences

- Redis is now required to run anything, including locally.
- A handler can run more than once (retries, worker restarts), so every handler must be safe to repeat: dispatch
  checks the payout is still `pending`, delivery checks the delivery is not already delivered.
- We can split the worker into its own deployment later without code changes.
