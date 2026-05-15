package api

import (
	"net/http"
	"time"
)

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "login.html", nil)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.render(w, "login.html", map[string]string{"Error": "Invalid request."})
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	_, pair, err := h.auth.LoginSingleOrg(r.Context(), username, password)
	if err != nil {
		h.render(w, "login.html", map[string]string{"Error": "Invalid username or password."})
		return
	}

	setTokenCookies(w, pair.AccessToken, pair.RefreshToken)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
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
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", MaxAge: -1, Path: "/"})
	}
}
