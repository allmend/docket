package middleware

import (
	"net/http"
	"time"
)

// sessionCookieTTL outlives the 15-minute access token on purpose: the refresh
// token in the second cookie is what keeps the session alive, and the middleware
// silently mints a new access token from it.
const sessionCookieTTL = 5 * 24 * time.Hour

// SetSessionCookies writes the session pair. It is the only place session
// cookies are written — there were three copies of this and the one in the
// middleware kept a hardcoded Secure:true through the change that made the flag
// configurable, so an HTTP deployment logged everyone out the moment the
// middleware refreshed their token and the browser discarded the reply.
//
// secure must come from config.CookieSecure. Marking cookies Secure means the
// browser only stores them over HTTPS; leaving it off puts the session on the
// wire in the clear.
func SetSessionCookies(w http.ResponseWriter, accessToken, refreshToken string, secure bool) {
	writeSessionCookie(w, "access_token", accessToken, secure, int(sessionCookieTTL.Seconds()))
	writeSessionCookie(w, "refresh_token", refreshToken, secure, int(sessionCookieTTL.Seconds()))
}

// ClearSessionCookies expires both cookies. The attributes have to match the
// ones used to set them or the browser keeps the originals and logout does
// nothing.
func ClearSessionCookies(w http.ResponseWriter, secure bool) {
	writeSessionCookie(w, "access_token", "", secure, -1)
	writeSessionCookie(w, "refresh_token", "", secure, -1)
}

func writeSessionCookie(w http.ResponseWriter, name, value string, secure bool, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}
