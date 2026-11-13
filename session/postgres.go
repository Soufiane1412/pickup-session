package session

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/lib/pq" // Postgres driver for database/sql
)

// Compile-time proof that both stores satisfy the same contract.
var (
	_ Store = (*MemoryStore)(nil)
	_ Store = (*PostgresStore)(nil)
)

// SQL. Same rules as memory.go, enforced by the database.
const (
	// Open: insert a new open session (drill 1).
	// $1 booking, $2 rider, $3 driver, $4 TTL in milliseconds.
	sqlOpen = `INSERT INTO location_session (booking_id, rider_id, driver_id, state, expires_at, ttl)
VALUES ($1, $2, $3, 'open', now() + $4 * interval '1 millisecond', $4 * interval '1 millisecond')`

	// FIND + lock the row (the Postgres equivalent of s.mu.Lock()).
	// "live" is computed with the database clock, never the app clock.
	sqlLockSession = `SELECT rider_id, driver_id, (state = 'open' AND expires_at > now())
FROM location_session WHERE booking_id = $1 FOR UPDATE`

	sqlGetSession = `SELECT rider_id, driver_id, (state = 'open' AND expires_at > now())
FROM location_session WHERE booking_id = $1`

	sqlLastSeq = `SELECT seq FROM last_position WHERE booking_id = $1 AND user_id = $2`

	// Upsert: insert the position, or replace it if this user already has one (drill 2).
	sqlUpsertPosition = `INSERT INTO last_position (booking_id, user_id, lat, lng, seq, client_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (booking_id, user_id) DO UPDATE
SET lat = EXCLUDED.lat, lng = EXCLUDED.lng, seq = EXCLUDED.seq, client_at = EXCLUDED.client_at`

	sqlRenewLease = `UPDATE location_session SET expires_at = now() + ttl WHERE booking_id = $1`

	// Close only if still open (idempotent), then delete the positions (drill 3).
	sqlClose           = `UPDATE location_session SET state = 'closed', close_reason = $2 WHERE booking_id = $1 AND state = 'open'`
	sqlDeletePositions = `DELETE FROM last_position WHERE booking_id = $1`

	// Written by Soufiane (7 Nov).
	sqlReadCounterpart = `SELECT p.lat, p.lng, p.seq
FROM last_position p
JOIN location_session s ON s.booking_id = p.booking_id
WHERE s.booking_id = $1
AND p.user_id = $2
AND s.state = 'open'
AND s.expires_at > now()`
)

type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore: same role as NewMemoryStore — hand back a store that is
// ready to use (connection open and checked), or an error.
func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Open(ctx context.Context, bookingID, riderID, driverID string, ttl time.Duration) error {
	_, err := s.db.ExecContext(ctx, sqlOpen, bookingID, riderID, driverID, ttl.Milliseconds())
	return err
}

func (s *PostgresStore) Heartbeat(ctx context.Context, bookingID, userID string, p Position) error {
	tx, err := s.db.BeginTx(ctx, nil) // LOCK: a transaction + FOR UPDATE replaces the mutex
	if err != nil {
		return err
	}
	defer tx.Rollback() // no-op once committed

	// FIND
	var riderID, driverID string
	var live bool
	err = tx.QueryRowContext(ctx, sqlLockSession, bookingID).Scan(&riderID, &driverID, &live)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSessionClosed
	}
	if err != nil {
		return err
	}
	// GUARDS — same order as memory.go
	if !live {
		return ErrSessionClosed
	}
	if userID != riderID && userID != driverID {
		return ErrNotParticipant
	}
	var lastSeq int64
	err = tx.QueryRowContext(ctx, sqlLastSeq, bookingID, userID).Scan(&lastSeq)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if p.Seq <= lastSeq {
		return ErrStale
	}
	// WRITE + RENEW
	if _, err := tx.ExecContext(ctx, sqlUpsertPosition, bookingID, userID, p.Lat, p.Lng, p.Seq, p.ClientAt); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, sqlRenewLease, bookingID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) ReadCounterpart(ctx context.Context, bookingID, callerID string) (Position, error) {
	// FIND
	var riderID, driverID string
	var live bool
	err := s.db.QueryRowContext(ctx, sqlGetSession, bookingID).Scan(&riderID, &driverID, &live)
	if errors.Is(err, sql.ErrNoRows) {
		return Position{}, ErrSessionClosed
	}
	if err != nil {
		return Position{}, err
	}
	// GUARDS: authorise before revealing, then open + not expired
	if callerID != riderID && callerID != driverID {
		return Position{}, ErrNotParticipant
	}
	if !live {
		return Position{}, ErrSessionClosed
	}
	// Whose data am I returning? The OTHER participant's.
	counterpart := riderID
	if callerID == riderID {
		counterpart = driverID
	}
	var p Position
	err = s.db.QueryRowContext(ctx, sqlReadCounterpart, bookingID, counterpart).Scan(&p.Lat, &p.Lng, &p.Seq)
	if errors.Is(err, sql.ErrNoRows) {
		return Position{}, nil // counterpart hasn't sent yet, or the session just closed: nothing leaks
	}
	if err != nil {
		return Position{}, err
	}
	return p, nil
}

func (s *PostgresStore) Close(ctx context.Context, bookingID, reason string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var riderID, driverID string
	var live bool
	err = tx.QueryRowContext(ctx, sqlLockSession, bookingID).Scan(&riderID, &driverID, &live)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSessionClosed
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, sqlClose, bookingID, reason); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, sqlDeletePositions, bookingID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) Participants(ctx context.Context, bookingID string) (string, string, error) {
	var riderID, driverID string
	err := s.db.QueryRowContext(ctx,
		`SELECT rider_id, driver_id FROM location_session WHERE booking_id = $1`, bookingID).
		Scan(&riderID, &driverID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrSessionClosed
	}
	return riderID, driverID, err
}
