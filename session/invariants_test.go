package session

// The receipt: one test per invariant.
// A test never touches s.mu or the map. It only calls the 4 methods
// (Open, Heartbeat, Close, ReadCounterpart) and checks what comes back.

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestOnlyCounterpartCanRead(t *testing.T) {
	// ARRANGE: Open a session, the driver sends a position
	ctx := context.Background()
	store := NewMemoryStore()

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

}

func TestCancelStopsReadsImmediately(t *testing.T) {
	// ARRANGE: open a session, the driver sends one position
	ctx := context.Background()
	store := NewMemoryStore()
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
}

func TestSilentPhoneExpires(t *testing.T) {
	// ARRANGE: very short lease, the driver sends once
	ctx := context.Background()
	store := NewMemoryStore()
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
}

func TestHeartbeatRenewsLease(t *testing.T) {
	// ARRANGE: 20 ms lease
	ctx := context.Background()
	store := NewMemoryStore()
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
}

func TestOlderUpdateIsRejected(t *testing.T) {
	// ARRANGE: the driver sends seq 5
	ctx := context.Background()
	store := NewMemoryStore()
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
}

func TestBoardedEndsSharing(t *testing.T) {
	// ARRANGE: open, the driver sends
	ctx := context.Background()
	store := NewMemoryStore()
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
}

func TestRebookingGetsNewSession(t *testing.T) {
	t.Skip("cancelled rider cannot read the driver's next booking")

	ctx := context.Background()
	store := NewMemoryStore()

	if err := store.Open(ctx, "b1", "r1", "d1", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(ctx, "r1", "cancelled"); err != nil {
		t.Fatal(err)
	}
	if err := store.Open(ctx, "b2", "d2", "r2", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.Heartbeat(ctx, "b2", "r2", Position{Seq: 1}); err != nil {
		t.Fatal(err)
	}
	_, err := store.ReadCounterpart(ctx, "r1", "b2")

	if !errors.Is(err, ErrNotParticipant) {
		t.Errorf("Non participant try to read newer session got:%v", err)
	}

}
func TestReadsAreLoggedWithoutCoordinates(t *testing.T) {
	t.Skip("each read writes an access-log row with no lat/lng")
}
