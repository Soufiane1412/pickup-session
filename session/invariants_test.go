package session

// The receipt: one test per invariant. Run every test against BOTH stores.
// YOU write the bodies. Remove t.Skip as each goes green.

import (
	"testing"
)

func TestOnlyCounterpartCanRead(t *testing.T) {
	t.Skip("IDOR: a third user reading this booking gets ErrNotParticipant")
}
func TestCancelStopsReadsImmediately(t *testing.T) {
	//ARRANGE: create a session with booking and cancel it
}
func TestSilentPhoneExpires(t *testing.T) {
	t.Skip("no heartbeat for > ttl → read returns nothing (fail closed)")
}
func TestHeartbeatRenewsLease(t *testing.T) {
	t.Skip("heartbeats inside ttl keep the session readable")
}
func TestOlderUpdateIsRejected(t *testing.T) {
	t.Skip("seq 5 then seq 4 → last-known stays seq 5 (latest-wins)")
}
func TestBoardedEndsSharing(t *testing.T) {
	t.Skip("Close(boarded) → no reads, even if the phone keeps sending")
}
func TestRebookingGetsNewSession(t *testing.T) {
	t.Skip("cancelled rider cannot read the driver's next booking")
}
func TestReadsAreLoggedWithoutCoordinates(t *testing.T) {
	t.Skip("each read writes an access-log row with no lat/lng")
}
