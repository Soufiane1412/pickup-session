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

## Run

    go test ./session/ -v -race   # in-memory store

Next: the same tests against a Postgres store (`make db-up`, `make test-pg`).
