package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/google/uuid"
)

// Authenticate validates the JWT from the Authorization header or access_token cookie,
// or an API token (dkt_ prefix) from the Authorization: Bearer header.
// On success it puts org/user/role/scope on context. On failure it redirects to /login
// for browser requests or returns 401 for API token requests.
func Authenticate(authSvc *service.AuthService, tokenSvc *service.TokenService) func(http.Handler) http.Handler {
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
						setSessionCookies(w, pair.AccessToken, pair.RefreshToken)
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

// RequireAdmin returns 403 if the authenticated user is not an org admin.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if service.RoleFromContext(r.Context()) != "admin" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireScope returns 403 if the request's token scope does not permit the
// required scope. JWT users are always granted full access.
func RequireScope(required model.TokenScope) func(http.Handler) http.Handler {
	order := map[model.TokenScope]int{
		model.ScopeMetricsRead: 1,
		model.ScopeAPIRead:     2,
		model.ScopeAPIWrite:    3,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scope := service.ScopeFromContext(r.Context())
			if order[scope] < order[required] {
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

func setSessionCookies(w http.ResponseWriter, accessToken, refreshToken string) {
	ttl := int((5 * 24 * time.Hour).Seconds())
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   ttl,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   ttl,
	})
}
