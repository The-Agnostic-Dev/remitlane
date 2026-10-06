# Runbook: payouts stuck in processing

## Symptoms

- A partner says a payout has been `processing` for a long time.
- The console dashboard shows a growing "in flight" count, or the Providers page shows no recent webhooks.

## Check

1. Open the payout in the console. The timeline shows every status change and its source (`api`, `swiftpay`,
   `momobridge`, `poller`). The "Partner webhooks" table shows what we told the partner.
2. Providers page: is the provider sending webhooks at all? Look at **Bad signatures**: a non-zero count means
   webhooks arrive but we refuse them. Check the webhook secret in the secret store matches the provider dashboard.
3. If webhooks stopped entirely, check the provider's status page and our ingress logs for `/webhooks/<provider>`.

## Fix

- The poller (`payout:poll`, every `POLL_INTERVAL`) asks the provider about payouts with no update for
  `POLL_STUCK_AFTER` (30 minutes) and applies what it learns. Most stuck payouts clear on their own within the hour.
- Do not mark a payout failed by hand while the provider may still pay it: that releases the funds to the partner
  and the payout can still complete. Ask the provider first.
- If a provider confirms a transfer failed but never told us, wait for the poller or ask engineering to run it now.

## After

If a partner missed webhooks because their endpoint was down, retry them from **Failed webhooks** once they confirm
it is back.
