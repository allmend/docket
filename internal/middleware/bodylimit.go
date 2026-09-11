package middleware

import "net/http"

// LimitBody caps request bodies at n bytes. Without it the JSON API decodes an
// unbounded body, and ParseForm's own ceiling (10 MB for url-encoded forms) is
// far above anything a ticket or comment needs. An oversized body fails the
// handler's parse, which answers 400.
func LimitBody(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, n)
			}
			next.ServeHTTP(w, r)
		})
	}
}
