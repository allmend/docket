package api

import (
	"net/http"

	"github.com/allmend/docket/internal/middleware"
	"github.com/allmend/docket/internal/service"
	"github.com/go-chi/chi/v5"
)

// orgNameForLogin resolves the org name shown on the sign-in page. login.html has
// always read .OrgName, but no handler supplied it, so the "to your <org>
// organization" line never rendered — caught by TestTemplateDataContract.
func (h *Handler) orgNameForLogin(r *http.Request) string {
	return h.auth.SingleOrgName(r.Context())
}

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "login.html", map[string]any{"Error": "", "OrgName": h.orgNameForLogin(r)})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.render(w, "login.html", map[string]any{"Error": "Invalid request.", "OrgName": h.orgNameForLogin(r)})
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	_, pair, err := h.auth.LoginSingleOrg(r.Context(), username, password)
	if err != nil {
		h.render(w, "login.html", map[string]any{"Error": "Invalid username or password.", "OrgName": h.orgNameForLogin(r)})
		return
	}

	h.setTokenCookies(w, pair.AccessToken, pair.RefreshToken)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) ProfilePage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "profile.html", h.pageData(r, map[string]any{
		"Profile": h.auth.GetCurrentUser(r.Context()),
		"IsSelf":  true,
	}))
}

// UserPage shows a member's profile card. It is where @username mentions in
// rendered markdown link to — those pointed at /users/{username} long before
// any route served it, so every mention was a dead link.
func (h *Handler) UserPage(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	u, err := h.tickets.GetUserByUsername(r.Context(), orgID, chi.URLParam(r, "username"))
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	h.render(w, "profile.html", h.pageData(r, map[string]any{
		"Profile": u,
		"IsSelf":  u.ID == service.UserIDFromContext(r.Context()),
	}))
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("refresh_token"); err == nil {
		h.auth.RevokeRefreshToken(r.Context(), c.Value)
	}
	h.clearTokenCookies(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("refresh_token")
	if err != nil {
		http.Error(w, "missing refresh token", http.StatusUnauthorized)
		return
	}

	_, pair, err := h.auth.Refresh(r.Context(), c.Value)
	if err != nil {
		h.clearTokenCookies(w)
		http.Error(w, "invalid refresh token", http.StatusUnauthorized)
		return
	}

	h.setTokenCookies(w, pair.AccessToken, pair.RefreshToken)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setTokenCookies(w http.ResponseWriter, accessToken, refreshToken string) {
	middleware.SetSessionCookies(w, accessToken, refreshToken, h.cookieSecure)
}

func (h *Handler) clearTokenCookies(w http.ResponseWriter) {
	middleware.ClearSessionCookies(w, h.cookieSecure)
}
