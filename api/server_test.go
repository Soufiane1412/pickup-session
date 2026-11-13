package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Soufiane1412/pickup-session/session"
)

// do sends one request to a fresh API and returns the status code.
func do(t *testing.T, h http.Handler, method, path, user, body string) int {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if user != "" {
		req.Header.Set("X-User-ID", user)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func setup(t *testing.T) http.Handler {
	t.Helper()
	h := New(session.NewMemoryStore())
	if got := do(t, h, "POST", "/sessions", "", `{"bookingId":"b1","riderId":"r1","driverId":"d1","ttlSeconds":60}`); got != http.StatusCreated {
		t.Fatalf("open: want 201, got %d", got)
	}
	if got := do(t, h, "POST", "/sessions/b1/heartbeats", "d1", `{"lat":48.8,"lng":2.3,"seq":5}`); got != http.StatusNoContent {
		t.Fatalf("heartbeat: want 204, got %d", got)
	}
	return h
}

func TestContract(t *testing.T) {
	cases := []struct {
		name               string
		method, path, user string
		body               string
		want               int
	}{
		{"rider reads driver", "GET", "/sessions/b1/counterpart", "r1", "", 200},
		{"stranger reads → 404", "GET", "/sessions/b1/counterpart", "x9", "", 404},
		{"stale heartbeat → 409", "POST", "/sessions/b1/heartbeats", "d1", `{"seq":4}`, 409},
		{"stranger heartbeat → 404", "POST", "/sessions/b1/heartbeats", "x9", `{"seq":9}`, 404},
		{"no identity → 401", "GET", "/sessions/b1/counterpart", "", "", 401},
		{"stranger closes → 404", "POST", "/sessions/b1/close", "x9", `{"reason":"cancelled"}`, 404},
		{"unknown booking closes → 404", "POST", "/sessions/nope/close", "r1", `{"reason":"cancelled"}`, 404},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := setup(t)
			if got := do(t, h, c.method, c.path, c.user, c.body); got != c.want {
				t.Errorf("want %d, got %d", c.want, got)
			}
		})
	}
}

func TestClosedSessionIsGone(t *testing.T) {
	h := setup(t)
	if got := do(t, h, "POST", "/sessions/b1/close", "r1", `{"reason":"boarded"}`); got != http.StatusNoContent {
		t.Fatalf("close: want 204, got %d", got)
	}
	if got := do(t, h, "GET", "/sessions/b1/counterpart", "r1", ""); got != http.StatusGone {
		t.Errorf("read after close: want 410, got %d", got)
	}
	if got := do(t, h, "POST", "/sessions/b1/heartbeats", "d1", `{"seq":6}`); got != http.StatusGone {
		t.Errorf("heartbeat after close: want 410, got %d", got)
	}
}
