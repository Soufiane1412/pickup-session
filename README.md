# pickup-session

Location sharing between rider and driver, pickup window only. Each test is one invariant from the design.

    make db-up      # Postgres 16 on :5433, runs migrations/
    make test       # in-memory store
    make test-pg    # same tests against Postgres
