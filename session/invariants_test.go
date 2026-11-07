package session

// The receipt: one test per invariant, run against EVERY store (memory + Postgres).
// A test never touches s.mu, the map or SQL. It only calls the 4 methods
// (Open, Heartbeat, Close, ReadCounterpart) and checks what comes back.

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// eachStore runs one test body against every Store implementation.
// Same test, two stores: that's what proves the database keeps the same rules.
func eachStore(t *testing.T, body func(t *testing.T, store Store)) {
	t.Run("memory", func(t *testing.T) { body(t, NewMemoryStore()) })
	t.Run("postgres", func(t *testing.T) {
		dsn := os.Getenv("PICKUP_DSN")
		if dsn == "" {
			t.Skip("PICKUP_DSN not set: run `make db-up` then `make test-pg`")
		}
		ctx := context.Background()
		s, err := NewPostgresStore(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { s.db.Close() })
		if _, err := s.db.ExecContext(ctx, "TRUNCATE location_session CASCADE"); err != nil {
			t.Fatal(err)
		}
		body(t, s)
	})
}

func TestOnlyCounterpartCanRead(t *testing.T) {
	eachStore(t, func(t *testing.T, store Store) {
		// ARRANGE: Open a session, the driver sends a position
		ctx := context.Background()

		if err := store.Open(ctx, "b1", "r1", "d1", time.Minute); err != nil {
			t.Fatal(err)
		}
		if err := store.Heartbeat(ctx, "b1", "r1", Position{Seq: 1}); err != nil {
			t.Fatal(err)
		}

		// ACT + ASSERT: a stranger reads
		pos, err := store.ReadCounterpart(ctx, "b1", "x9")
		if !errors.Is(err, ErrNotParticipant) {
			t.Errorf("A non participant tried to access data form the session, got:%v", err)
		}
		if pos != (Position{}) {
			t.Errorf("posiiton leaked to a non participants :%v", pos)
		}

	})
}

func TestCancelStopsReadsImmediately(t *testing.T) {
	eachStore(t, func(t *testing.T, store Store) {
		// ARRANGE: open a session, the driver sends one position
		ctx := context.Background()
		if err := store.Open(ctx, "b1", "r1", "d1", time.Minute); err != nil {
			t.Fatal(err)
		}
		if err := store.Heartbeat(ctx, "b1", "d1", Position{Seq: 1}); err != nil {
			t.Fatal(err)
		}

		// ACT: the ride is cancelled
		if err := store.Close(ctx, "b1", "cancelled"); err != nil {
			t.Fatal(err)
		}

		// ASSERT: the rider's next read gets the error AND no position
		pos, err := store.ReadCounterpart(ctx, "b1", "r1")
		if !errors.Is(err, ErrSessionClosed) {
			t.Errorf("want ErrSessionClosed, got %v", err)
		}
		if pos != (Position{}) {
			t.Errorf("leaked position after cancel: %+v", pos)
		}
	})
}

func TestSilentPhoneExpires(t *testing.T) {
	eachStore(t, func(t *testing.T, store Store) {
		// ARRANGE: very short lease, the driver sends once
		ctx := context.Background()
		if err := store.Open(ctx, "b1", "r1", "d1", 20*time.Millisecond); err != nil {
			t.Fatal(err)
		}
		if err := store.Heartbeat(ctx, "b1", "d1", Position{Seq: 1}); err != nil {
			t.Fatal(err)
		}

		// ACT: the phone goes silent for longer than the lease
		time.Sleep(30 * time.Millisecond)

		// ASSERT: fail closed, the read returns nothing
		pos, err := store.ReadCounterpart(ctx, "b1", "r1")
		if !errors.Is(err, ErrSessionClosed) {
			t.Errorf("want ErrSessionClosed, got %v", err)
		}
		if pos != (Position{}) {
			t.Errorf("leaked position after expiry: %+v", pos)
		}
	})
}

func TestHeartbeatRenewsLease(t *testing.T) {
	eachStore(t, func(t *testing.T, store Store) {
		// ARRANGE: 20 ms lease
		ctx := context.Background()
		if err := store.Open(ctx, "b1", "r1", "d1", 20*time.Millisecond); err != nil {
			t.Fatal(err)
		}

		// ACT: 3 heartbeats, each inside the lease → total 30 ms, longer than one lease
		for seq := int64(1); seq <= 3; seq++ {
			time.Sleep(10 * time.Millisecond)
			if err := store.Heartbeat(ctx, "b1", "d1", Position{Seq: seq}); err != nil {
				t.Fatalf("heartbeat %d: %v", seq, err)
			}
		}

		// ASSERT: still readable, latest position
		pos, err := store.ReadCounterpart(ctx, "b1", "r1")
		if err != nil {
			t.Fatalf("lease should still be alive, got %v", err)
		}
		if pos.Seq != 3 {
			t.Errorf("want seq 3, got %d", pos.Seq)
		}
	})
}

func TestOlderUpdateIsRejected(t *testing.T) {
	eachStore(t, func(t *testing.T, store Store) {
		// ARRANGE: the driver sends seq 5
		ctx := context.Background()
		if err := store.Open(ctx, "b1", "r1", "d1", time.Minute); err != nil {
			t.Fatal(err)
		}
		if err := store.Heartbeat(ctx, "b1", "d1", Position{Seq: 5}); err != nil {
			t.Fatal(err)
		}

		// ACT: an older update (seq 4) arrives late
		err := store.Heartbeat(ctx, "b1", "d1", Position{Seq: 4})

		// ASSERT: rejected, and the last-known stays seq 5 (latest-wins)
		if !errors.Is(err, ErrStale) {
			t.Errorf("want ErrStale, got %v", err)
		}
		pos, _ := store.ReadCounterpart(ctx, "b1", "r1")
		if pos.Seq != 5 {
			t.Errorf("want seq 5, got %d", pos.Seq)
		}
	})
}

func TestBoardedEndsSharing(t *testing.T) {
	eachStore(t, func(t *testing.T, store Store) {
		// ARRANGE: open, the driver sends
		ctx := context.Background()
		if err := store.Open(ctx, "b1", "r1", "d1", time.Minute); err != nil {
			t.Fatal(err)
		}
		if err := store.Heartbeat(ctx, "b1", "d1", Position{Seq: 1}); err != nil {
			t.Fatal(err)
		}

		// ACT: the rider boards, then the phone keeps sending
		if err := store.Close(ctx, "b1", "boarded"); err != nil {
			t.Fatal(err)
		}
		err := store.Heartbeat(ctx, "b1", "d1", Position{Seq: 2})

		// ASSERT: the late heartbeat is refused and nothing is readable
		if !errors.Is(err, ErrSessionClosed) {
			t.Errorf("want ErrSessionClosed on heartbeat after boarding, got %v", err)
		}
		pos, _ := store.ReadCounterpart(ctx, "b1", "r1")
		if pos != (Position{}) {
			t.Errorf("leaked position after boarding: %+v", pos)
		}
	})
}

func TestRebookingGetsNewSession(t *testing.T) {
	eachStore(t, func(t *testing.T, store Store) {
		// ARRANGE: booking b1 (r1, d1) is cancelled, then the same driver
		// opens b2 with another rider and sends a position
		ctx := context.Background()
		if err := store.Open(ctx, "b1", "r1", "d1", time.Minute); err != nil {
			t.Fatal(err)
		}
		if err := store.Close(ctx, "b1", "cancelled"); err != nil {
			t.Fatal(err)
		}
		if err := store.Open(ctx, "b2", "r2", "d1", time.Minute); err != nil {
			t.Fatal(err)
		}
		if err := store.Heartbeat(ctx, "b2", "d1", Position{Seq: 1}); err != nil {
			t.Fatal(err)
		}

		// ACT: the cancelled rider tries to read the driver's new booking
		pos, err := store.ReadCounterpart(ctx, "b2", "r1")

		// ASSERT: refused, and no position
		if !errors.Is(err, ErrNotParticipant) {
			t.Errorf("want ErrNotParticipant, got %v", err)
		}
		if pos != (Position{}) {
			t.Errorf("leaked the driver's new position to the old rider: %+v", pos)
		}
	})
}

func TestReadsAreLoggedWithoutCoordinates(t *testing.T) {
	t.Skip("each read writes an access-log row with no lat/lng")
}
