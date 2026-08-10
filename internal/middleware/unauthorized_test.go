package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// unauthorizedHandler builds the middleware around a handler that must never
// run. Passing nil services is safe here: a request with no token at all is
// rejected by tokenFromRequest before either service is touched, which is
// exactly the expired-session path being tested.
func unauthorizedHandler(t *testing.T) http.Handler {
	t.Helper()
	return Authenticate(nil, nil, true)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("request without a token reached the wrapped handler")
	}))
}

// TestUnauthorizedHTMXRequestRedirectsWholePage covers DKT-9. A 303 is followed
// by htmx transparently, and the login page it returns gets swapped into
// whatever element triggered the request — the board's 30s poll rendered a login
// form inside #board-columns while the sidebar and nav stayed put. The response
// has to tell htmx to navigate instead of swap.
func TestUnauthorizedHTMXRequestRedirectsWholePage(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/boards/abc/columns", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	unauthorizedHandler(t).ServeHTTP(rec, req)

	res := rec.Result()
	if got := res.Header.Get("HX-Redirect"); got != "/login" {
		t.Errorf("HX-Redirect = %q, want %q", got, "/login")
	}
	// A 3xx would make htmx follow the redirect itself and swap the result.
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		t.Errorf("status = %d; a redirect status makes htmx follow and swap", res.StatusCode)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q, want empty — nothing should be swappable", body)
	}
}

// TestUnauthorizedBrowserRequestStillRedirects pins that a normal navigation is
// untouched: there is no htmx involved, so the server-side redirect is right.
func TestUnauthorizedBrowserRequestStillRedirects(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/boards/abc", nil)
	rec := httptest.NewRecorder()

	unauthorizedHandler(t).ServeHTTP(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", res.StatusCode, http.StatusSeeOther)
	}
	if got := res.Header.Get("Location"); got != "/login" {
		t.Errorf("Location = %q, want %q", got, "/login")
	}
	if got := res.Header.Get("HX-Redirect"); got != "" {
		t.Errorf("HX-Redirect = %q on a non-htmx request, want none", got)
	}
}

// TestUnauthorizedAPITokenRequestGets401 pins that API clients keep getting a
// status they can act on rather than a redirect to an HTML page.
func TestUnauthorizedAPITokenRequestGets401(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", nil)
	// An empty Bearer value keeps this on the no-token path (so the nil services
	// are never touched) while still carrying the header that marks it an API
	// client.
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()

	unauthorizedHandler(t).ServeHTTP(rec, req)

	if res := rec.Result(); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", res.StatusCode, http.StatusUnauthorized)
	}
}
