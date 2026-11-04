package session

import (
	"context"
	"sync"
	"time"
)

type entry struct {
	riderID, driverID string
	state             State
	expiresAt         time.Time
	last              map[string]Position
	ttl               time.Duration
	reason            string
}

type MemoryStore struct {
	mu      sync.Mutex
	session map[string]*entry // bookingID -> session
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{session: make(map[string]*entry)}
}

func (s *MemoryStore) Open(ctx context.Context, bookingID, riderID, driverID string, ttl time.Duration) error {

	s.mu.Lock()
	defer s.mu.Unlock()
	s.session[bookingID] = &entry{
		riderID:   riderID,
		driverID:  driverID,
		state:     Open,
		expiresAt: time.Now().Add(ttl),
		last:      make(map[string]Position),
		ttl:       ttl,
	}
	return nil
}

func (s *MemoryStore) Heartbeat(ctx context.Context, bookingID, userID string, p Position) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.session[bookingID]
	if !ok {
		return ErrSessionClosed
	}
	if e.state != Open || time.Now().After(e.expiresAt) {
		return ErrSessionClosed
	}
	if e.riderID != userID && e.driverID != userID {
		return ErrNotParticipant
	}
	if p.Seq <= e.last[userID].Seq {
		return ErrStale
	}
	e.last[userID] = p
	e.expiresAt = time.Now().Add(e.ttl)
	return nil

}

func (s *MemoryStore) ReadCounterpart(ctx context.Context, bookingID, callerID string) (Position, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.session[bookingID]
	if !ok {
		return Position{}, ErrSessionClosed
	}

	if e.driverID != callerID && e.riderID != callerID {
		return Position{}, ErrNotParticipant
	}
	if e.state != Open || time.Now().After(e.expiresAt) {
		return Position{}, ErrSessionClosed
	}
	if callerID == e.riderID {
		return e.last[e.driverID], nil
	}
	return e.last[e.riderID], nil
}

func (s *MemoryStore) Close(ctx context.Context, bookingID, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.session[bookingID]
	if !ok {
		return ErrSessionClosed
	}

	if e.state != Open {
		return nil
	}
	e.last = nil
	e.state = Closed
	e.reason = reason
	return nil
	//Don't know where to transport the reason payload?
}
