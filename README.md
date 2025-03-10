# remitlane

Payouts API for African fintechs. Partners send USD and we pay out NGN, KES, GHS and UGX to banks and mobile money.

## Run

```
createdb remitlane
go run ./cmd/remitlane
```

Needs Postgres. `DATABASE_URL` defaults to `postgres://postgres@localhost:55421/remitlane`.

## Notes

- amounts are minor units everywhere in the API
- ledger is double entry, see internal/ledger
- TODO: real README
