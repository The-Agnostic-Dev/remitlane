# Sandbox

With `SWIFTPAY_MODE=sandbox` and `MOMOBRIDGE_MODE=sandbox` (the defaults) both providers are in-process fakes
mounted under `/_sandbox`. They keep state in memory (it resets when the app restarts), answer like the real APIs
and call our webhook endpoints back over HTTP, signed with the sandbox secrets from `.env.example`.

The console's **Sandbox** page (`/ops/sandbox`) lists every transfer and callback and can force the next outcome.
`/_dev/sandbox/transfers.json?reference=<payout id>` returns the same as JSON.

## Swiftpay (bank)

API v2: `POST /v2/transfers`, `GET /v2/transfers/{id}`, bearer key auth. Webhooks go to `/webhooks/swiftpay` with
`X-Swiftpay-Signature: sha256=<hex HMAC of the raw body>`. `SWIFTPAY_API_VERSION=v1` makes the sandbox send the
legacy v1 webhook body (`type`/`transfer_id`/`ref`/`state`), which some older Swiftpay accounts still get.

v2 transfer statuses: `QUEUED`, `PROCESSING`, `ON_HOLD`, `COMPLETED`, `FAILED`, `REVERSED`.

| Recipient account number ends in | Outcome |
| --- | --- |
| `0001` | `FAILED` with `INVALID_ACCOUNT` |
| `0099` | `ON_HOLD` (`COMPLIANCE_REVIEW`), then `COMPLETED` about 4 seconds later |
| `0077` | `COMPLETED`, then `REVERSED` (`RETURNED_BY_BANK`) |
| `0055` | `COMPLETED` but no webhook is sent; the poller finds it after `POLL_STUCK_AFTER` |
| anything else | `PROCESSING` on create, `COMPLETED` after `SWIFTPAY_SANDBOX_DELAY` |

NGN transfers of `SWIFTPAY_SANDBOX_REVIEW_THRESHOLD_NGN` (₦1,000,000) or more are queued for manual review, as on the
live service: the create call answers `202 Accepted` with status `QUEUED`, and the transfer completes after
`SWIFTPAY_SANDBOX_REVIEW_DELAY`. Swiftpay honours an `Idempotency-Key` header on create.

Failed webhook deliveries are retried three times, two seconds apart.

