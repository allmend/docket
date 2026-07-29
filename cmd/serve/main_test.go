package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// loginRouter wires the real login limiters in front of a stub handler, so the
// tests below exercise the middleware the server actually uses.
func loginRouter() http.Handler {
	r := chi.NewRouter()
	r.With(loginRateLimiters()...).Post("/login", func(w http.ResponseWriter, r *http.Request) {
		// Parse again, as the real handler does, to prove the key func left the
		// form readable.
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.PostFormValue("username") == "" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	return r
}

func postLogin(t *testing.T, h http.Handler, username string) int {
	t.Helper()
	body := url.Values{"username": {username}, "password": {"wrong"}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "10.0.0.1:1234" // every request shares one address, as behind a proxy
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

// TestLoginRateLimitIsPerAccount is the point of keying on the username: a
// proxy or gateway makes every request share one source address, so an IP-keyed
// limit would let one attacker lock out the whole instance.
func TestLoginRateLimitIsPerAccount(t *testing.T) {
	h := loginRouter()

	for i := 1; i <= 10; i++ {
		if code := postLogin(t, h, "alice"); code != http.StatusOK {
			t.Fatalf("attempt %d for alice = %d, want 200 (limit is 10)", i, code)
		}
	}
	if code := postLogin(t, h, "alice"); code != http.StatusTooManyRequests {
		t.Errorf("11th attempt for alice = %d, want 429", code)
	}

	// The whole point: bob is unaffected by alice being spammed.
	if code := postLogin(t, h, "bob"); code != http.StatusOK {
		t.Errorf("bob = %d, want 200 — one account's limit must not lock out others", code)
	}
}

// TestLoginRateLimitIgnoresForgedForwardedFor guards the reason the RealIP key
// funcs are not used here: they read X-Forwarded-For with no trusted-proxy
// check, so anything able to reach the pod could mint a fresh bucket per request.
func TestLoginRateLimitIgnoresForgedForwardedFor(t *testing.T) {
	h := loginRouter()

	for i := 1; i <= 10; i++ {
		if code := postLogin(t, h, "carol"); code != http.StatusOK {
			t.Fatalf("attempt %d for carol = %d, want 200", i, code)
		}
	}

	body := url.Values{"username": {"carol"}, "password": {"wrong"}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.Header.Set("X-Real-IP", "203.0.113.9")
	req.Header.Set("True-Client-IP", "203.0.113.9")
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("forged client-IP headers got %d, want 429 — the limit must not be bypassable", rec.Code)
	}
}

// TestKeyByLoginUsernameNormalises keeps "Alice", "alice" and " alice " in one
// bucket, so case or whitespace cannot be used to multiply the allowance.
func TestKeyByLoginUsernameNormalises(t *testing.T) {
	for _, raw := range []string{"alice", "Alice", "  ALICE  "} {
		body := url.Values{"username": {raw}}.Encode()
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		key, err := keyByLoginUsername(req)
		if err != nil {
			t.Fatalf("keyByLoginUsername(%q): %v", raw, err)
		}
		if key != "alice" {
			t.Errorf("keyByLoginUsername(%q) = %q, want %q", raw, key, "alice")
		}
	}
}

// TestKeyFuncLeavesFormReadable pins the ParseForm caching the key func relies
// on: reading the body in middleware must not leave the handler with nothing.
func TestKeyFuncLeavesFormReadable(t *testing.T) {
	if code := postLogin(t, loginRouter(), "dave"); code != http.StatusOK {
		t.Fatalf("handler saw no username after the key func parsed the form (got %d)", code)
	}
}
