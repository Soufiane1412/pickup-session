# pickup-session

Live location sharing between a carpool rider and driver, **for the pickup window only**.

Born from a real BlaBlaCar pickup that failed and moved to SMS: the driver had to text a Waze screenshot of where he was. A carpool driver is a private person, so his route is his routine: location must be shared briefly, with the right person, and then disappear.

## Invariants (each one is a test)

| Invariant | Test |
|---|---|
| Only the two participants of a booking can read a position | `TestOnlyCounterpartCanRead` |
| A cancellation stops reads immediately | `TestCancelStopsReadsImmediately` |
| A silent phone's session expires (fail closed) | `TestSilentPhoneExpires` |
| Heartbeats renew the lease | `TestHeartbeatRenewsLease` |
| An older update never overwrites a newer one (latest-wins) | `TestOlderUpdateIsRejected` |
| Boarding ends sharing, even if the phone keeps sending | `TestBoardedEndsSharing` |
| A cancelled rider can't read the driver's next booking | `TestRebookingGetsNewSession` |

## Two stores, one contract

Every invariant runs against **both** implementations of the same `Store` interface:

| Store | How it enforces the rules |
|---|---|
| `MemoryStore` | a mutex around find → guards → write |
| `PostgresStore` | a transaction + `SELECT … FOR UPDATE` row lock; expiry computed with the database clock (`expires_at > now()`), never the app clock |

7 invariants × 2 stores = **14 green**, race-detector clean.

## Run

    make test       # in-memory store
    make db-up      # Postgres 16 in Docker, runs migrations/
    make test-pg    # the same 7 tests against Postgres

Next: an HTTP API and a React Native client for the pickup window.
