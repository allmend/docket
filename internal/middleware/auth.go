package middleware

import (
	"net/http"
	"strings"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/google/uuid"
)

// Authenticate validates the JWT from the Authorization header or access_token cookie,
// or an API token (dkt_ prefix) from the Authorization: Bearer header.
// On success it puts org/user/role/scope on context. On failure it redirects to /login
// for browser requests or returns 401 for API token requests.
// cookieSecure must match the flag the login handler uses. A mismatch is
// invisible until the transparent refresh below fires, at which point the
// browser silently drops the reply's cookies and the session is gone.
func Authenticate(authSvc *service.AuthService, tokenSvc *service.TokenService, cookieSecure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := tokenFromRequest(r)
			if raw == "" {
				redirectOrUnauthorized(w, r)
				return
			}

			// API token path — dkt_ prefix
			if strings.HasPrefix(raw, "dkt_") {
				t, err := tokenSvc.Validate(r.Context(), raw)
				if err != nil {
					http.Error(w, "invalid or revoked token", http.StatusUnauthorized)
					return
				}
				ctx := service.WithIdentity(r.Context(), t.OrgID, t.CreatedBy, "member")
				ctx = service.WithScope(ctx, t.Scope)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// JWT path
			claims, err := authSvc.ValidateAccessToken(raw)
			if err != nil {
				// Access token invalid — attempt transparent refresh.
				if rc, cerr := r.Cookie("refresh_token"); cerr == nil {
					_, pair, rerr := authSvc.Refresh(r.Context(), rc.Value)
					if rerr == nil {
						SetSessionCookies(w, pair.AccessToken, pair.RefreshToken, cookieSecure)
						claims, err = authSvc.ValidateAccessToken(pair.AccessToken)
					}
				}
				if err != nil {
					redirectOrUnauthorized(w, r)
					return
				}
			}
			orgID, err := uuid.Parse(claims.OrgID)
			if err != nil {
				redirectOrUnauthorized(w, r)
				return
			}
			userID, err := uuid.Parse(claims.UserID)
			if err != nil {
				redirectOrUnauthorized(w, r)
				return
			}
			ctx := service.WithIdentity(r.Context(), orgID, userID, claims.Role)
			ctx = service.WithScope(ctx, model.ScopeAPIWrite) // JWT users have full access
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireScope returns 403 if the request's token scope does not permit the
// required scope. JWT users are always granted full access.
func RequireScope(required model.TokenScope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !service.ScopeFromContext(r.Context()).Allows(required) {
				http.Error(w, "insufficient token scope", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func redirectOrUnauthorized(w http.ResponseWriter, r *http.Request) {
	// API token requests use Authorization header — return 401, not a redirect.
	if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	// HTMX follows a 303 transparently and swaps whatever comes back into the
	// element that triggered the request, so a 30s board poll on an expired
	// session painted the login form inside #board-columns with the sidebar and
	// nav still around it. HX-Redirect tells htmx to navigate the whole page
	// instead; it is handled before any swap logic, so the body is never used.
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func tokenFromRequest(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if c, err := r.Cookie("access_token"); err == nil {
		return c.Value
	}
	return ""
}

// Session cookies are written by SetSessionCookies in cookies.go — one copy,
// carrying the configured Secure flag.
