package middleware

import (
	"net/http"
	"strings"

	"github.com/allmend/docket/internal/service"
	"github.com/google/uuid"
)

// Authenticate validates the JWT from the Authorization header or access_token cookie.
// On success it puts org/user/role on context. On failure it redirects to /login.
func Authenticate(authSvc *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := tokenFromRequest(r)
			if token == "" {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			claims, err := authSvc.ValidateAccessToken(token)
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			orgID, err := uuid.Parse(claims.OrgID)
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			userID, err := uuid.Parse(claims.UserID)
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			ctx := service.WithIdentity(r.Context(), orgID, userID, claims.Role)
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

func tokenFromRequest(r *http.Request) string {
	// Prefer Authorization: Bearer <token>
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	// Fall back to httpOnly cookie
	if c, err := r.Cookie("access_token"); err == nil {
		return c.Value
	}
	return ""
}
