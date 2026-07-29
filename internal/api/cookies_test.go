package api

import (
	"net/http/httptest"
	"testing"
)

// TestCookieSecureFlagReachesCookies pins the wiring between config and the
// Set-Cookie header. Getting this wrong is invisible in tests but breaks login
// entirely: a Secure cookie is dropped by the browser over plain HTTP, and an
// insecure one leaks the session on a network anyone can watch.
func TestCookieSecureFlagReachesCookies(t *testing.T) {
	for _, secure := range []bool{true, false} {
		h := &Handler{cookieSecure: secure}

		rec := httptest.NewRecorder()
		h.setTokenCookies(rec, "access", "refresh")
		cookies := rec.Result().Cookies()
		if len(cookies) != 2 {
			t.Fatalf("cookieSecure=%v: got %d cookies, want 2", secure, len(cookies))
		}
		for _, c := range cookies {
			if c.Secure != secure {
				t.Errorf("cookieSecure=%v: %s has Secure=%v", secure, c.Name, c.Secure)
			}
			if !c.HttpOnly {
				t.Errorf("cookieSecure=%v: %s lost HttpOnly", secure, c.Name)
			}
		}

		// The deletion cookies must match, or logging out over plain HTTP leaves
		// the session cookie in place.
		rec = httptest.NewRecorder()
		h.clearTokenCookies(rec)
		for _, c := range rec.Result().Cookies() {
			if c.Secure != secure {
				t.Errorf("cookieSecure=%v: cleared %s has Secure=%v", secure, c.Name, c.Secure)
			}
			if c.MaxAge >= 0 {
				t.Errorf("cleared %s has MaxAge=%d, want negative", c.Name, c.MaxAge)
			}
		}
	}
}
