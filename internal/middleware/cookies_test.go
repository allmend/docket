package middleware

import (
	"net/http/httptest"
	"testing"
)

// TestSessionCookiesHonourSecure pins the flag through the only writer of
// session cookies. The bug this replaces: the middleware kept its own copy of
// this code with Secure hardcoded true, so on an HTTP deployment the login
// handler set a cookie the browser accepted and the transparent refresh 15
// minutes later replaced it with one the browser discarded — a session that
// died on a timer with nothing in the logs.
func TestSessionCookiesHonourSecure(t *testing.T) {
	for _, secure := range []bool{true, false} {
		rec := httptest.NewRecorder()
		SetSessionCookies(rec, "access", "refresh", secure)

		cookies := rec.Result().Cookies()
		if len(cookies) != 2 {
			t.Fatalf("secure=%v: got %d cookies, want 2", secure, len(cookies))
		}
		for _, c := range cookies {
			if c.Secure != secure {
				t.Errorf("secure=%v: %s has Secure=%v", secure, c.Name, c.Secure)
			}
			if !c.HttpOnly {
				t.Errorf("secure=%v: %s lost HttpOnly", secure, c.Name)
			}
			if c.MaxAge <= 0 {
				t.Errorf("secure=%v: %s has MaxAge=%d, want positive", secure, c.Name, c.MaxAge)
			}
		}

		// The clear path has to match, or logout leaves the cookie in place.
		rec = httptest.NewRecorder()
		ClearSessionCookies(rec, secure)
		for _, c := range rec.Result().Cookies() {
			if c.Secure != secure {
				t.Errorf("secure=%v: cleared %s has Secure=%v", secure, c.Name, c.Secure)
			}
			if c.MaxAge >= 0 {
				t.Errorf("cleared %s has MaxAge=%d, want negative", c.Name, c.MaxAge)
			}
		}
	}
}
