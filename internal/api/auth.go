package api

import (
	"net/http"
	"time"
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

	setTokenCookies(w, pair.AccessToken, pair.RefreshToken)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) ProfilePage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "profile.html", h.pageData(r, nil))
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("refresh_token"); err == nil {
		h.auth.RevokeRefreshToken(r.Context(), c.Value)
	}
	clearTokenCookies(w)
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
		clearTokenCookies(w)
		http.Error(w, "invalid refresh token", http.StatusUnauthorized)
		return
	}

	setTokenCookies(w, pair.AccessToken, pair.RefreshToken)
	w.WriteHeader(http.StatusNoContent)
}

func setTokenCookies(w http.ResponseWriter, accessToken, refreshToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((5 * 24 * time.Hour).Seconds()),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((5 * 24 * time.Hour).Seconds()),
	})
}

func clearTokenCookies(w http.ResponseWriter) {
	for _, name := range []string{"access_token", "refresh_token"} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			MaxAge:   -1,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})
	}
}
