// Package api exposes the session store over HTTP (REST + JSON).
// Contract (designed 8 Nov):
//
//	POST /sessions                      201
//	POST /sessions/{id}/heartbeats      204 | 410 closed | 404 stranger | 409 stale
//	GET  /sessions/{id}/counterpart     200 | 410 closed | 404 stranger
//	POST /sessions/{id}/close           204 | 404 unknown or stranger
//
// Identity: DEMO ONLY. The caller is read from the X-User-ID header.
// A real deployment verifies a signed token (JWT) instead.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Soufiane1412/pickup-session/session"
)

type Store interface {
	session.Store
	session.ParticipantLookup
}

type Server struct {
	store Store
}

func New(store Store) http.Handler {
	s := &Server{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /sessions", s.open)
	mux.HandleFunc("POST /sessions/{id}/heartbeats", s.heartbeat)
	mux.HandleFunc("GET /sessions/{id}/counterpart", s.counterpart)
	mux.HandleFunc("POST /sessions/{id}/close", s.close)
	return withCORS(mux)
}

// 🔴 YOU: map each domain error to the status in the contract above.
// Use errors.Is(err, session.ErrX). Anything unknown → 500.

func statusFor(err error) int {
	switch {
	case errors.Is(err, session.ErrNotParticipant):
		return 404
	case errors.Is(err, session.ErrSessionClosed):
		return 410
	case errors.Is(err, session.ErrStale):
		return 409
	default:
		return 500
	}
}

type openReq struct {
	BookingID  string `json:"bookingId"`
	RiderID    string `json:"riderId"`
	DriverID   string `json:"driverId"`
	TTLSeconds int    `json:"ttlSeconds"`
}

func (s *Server) open(w http.ResponseWriter, r *http.Request) {
	var req openReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BookingID == "" || req.TTLSeconds <= 0 {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	ttl := time.Duration(req.TTLSeconds) * time.Second
	if err := s.store.Open(r.Context(), req.BookingID, req.RiderID, req.DriverID, ttl); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

type positionJSON struct {
	Lat      float64   `json:"lat"`
	Lng      float64   `json:"lng"`
	Seq      int64     `json:"seq"`
	ClientAt time.Time `json:"clientAt"`
}

func (s *Server) heartbeat(w http.ResponseWriter, r *http.Request) {
	caller, ok := callerID(w, r)
	if !ok {
		return
	}
	var p positionJSON
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	pos := session.Position{Lat: p.Lat, Lng: p.Lng, Seq: p.Seq, ClientAt: p.ClientAt}
	if err := s.store.Heartbeat(r.Context(), r.PathValue("id"), caller, pos); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) counterpart(w http.ResponseWriter, r *http.Request) {
	caller, ok := callerID(w, r)
	if !ok {
		return
	}
	pos, err := s.store.ReadCounterpart(r.Context(), r.PathValue("id"), caller)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(positionJSON{Lat: pos.Lat, Lng: pos.Lng, Seq: pos.Seq, ClientAt: pos.ClientAt})
}

type closeReq struct {
	Reason string `json:"reason"` // "boarded" | "cancelled"
}

func (s *Server) close(w http.ResponseWriter, r *http.Request) {
	caller, ok := callerID(w, r)
	if !ok {
		return
	}
	bookingID := r.PathValue("id")

	// 🔴 YOU: authorise. Only the rider or the driver of THIS booking may close it.
	// Use s.store.Participants(...). Unknown booking or stranger → 404 (never reveal which).
	riderID, driverID, err := s.store.Participants(r.Context(), bookingID)
	if err != nil || (caller != riderID && driverID != caller) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}

	var req closeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Reason == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := s.store.Close(r.Context(), bookingID, req.Reason); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func callerID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.Header.Get("X-User-ID")
	if id == "" {
		http.Error(w, "missing X-User-ID", http.StatusUnauthorized)
		return "", false
	}
	return id, true
}

func writeErr(w http.ResponseWriter, err error) {
	status := statusFor(err)
	msg := http.StatusText(status)
	if status == http.StatusInternalServerError {
		msg = "internal error" // never leak internals
	}
	http.Error(w, msg, status)
}

// withCORS lets the web build (another domain) call the API.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-ID")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
