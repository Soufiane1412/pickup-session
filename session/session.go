// Package session: the pickup-window location session.
// Types transcribed from 02_design/design.md. YOU write every method body.
package session

import (
	"context"
	"errors"
	"time"
)

type State string

const (
	Open   State = "open"   // inside pickup area, consent given
	Closed State = "closed" // boarded | cancelled | revoked | timed out
)

type Position struct {
	Lat, Lng float64
	Seq      int64 // latest-wins
	ClientAt time.Time
}

var (
	ErrNotParticipant = errors.New("caller is not a participant of this booking")
	ErrSessionClosed  = errors.New("session closed")
	ErrStale          = errors.New("older than last-known position")
)

// Store is the contract both implementations satisfy (memory first, then Postgres).
type Store interface {
	Open(ctx context.Context, bookingID, riderID, driverID string, ttl time.Duration) error
	Heartbeat(ctx context.Context, bookingID, userID string, p Position) error // write + renew lease
	ReadCounterpart(ctx context.Context, bookingID, callerID string) (Position, error)
	Close(ctx context.Context, bookingID, reason string) error // boarded | cancelled | revoked
}
