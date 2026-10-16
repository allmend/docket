package api

import (
	"log/slog"
	"net/http"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/go-chi/chi/v5"
)

// routes registers handlers on a chi router, each with the tier it requires.
// Routes and V1Routes add every route through it, so no route can exist
// without a declared tier; TestEveryRouteDeclaresTier enforces that.
type routes struct {
	r   chi.Router
	api bool // JSON API: answer a denial with JSON, not plain text
}

func (rt routes) get(t model.Tier, pattern string, fn http.HandlerFunc) {
	rt.handle(http.MethodGet, t, pattern, fn)
}

func (rt routes) post(t model.Tier, pattern string, fn http.HandlerFunc) {
	rt.handle(http.MethodPost, t, pattern, fn)
}

func (rt routes) put(t model.Tier, pattern string, fn http.HandlerFunc) {
	rt.handle(http.MethodPut, t, pattern, fn)
}

func (rt routes) delete(t model.Tier, pattern string, fn http.HandlerFunc) {
	rt.handle(http.MethodDelete, t, pattern, fn)
}

func (rt routes) handle(method string, t model.Tier, pattern string, fn http.HandlerFunc) {
	rt.r.Method(method, pattern, tierHandler{tier: t, api: rt.api, next: fn})
}

// tierHandler asks service.CanDo before running the handler. Handlers never
// check roles or scopes themselves.
type tierHandler struct {
	tier model.Tier
	api  bool
	next http.HandlerFunc
}

func (th tierHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := service.CanDo(r.Context(), th.tier); err != nil {
		route := ""
		if rc := chi.RouteContext(r.Context()); rc != nil {
			route = rc.RoutePattern()
		}
		slog.Warn("access denied",
			"org_id", service.OrgIDFromContext(r.Context()),
			"user_id", service.UserIDFromContext(r.Context()),
			"tier", th.tier.String(),
			"route", route,
		)
		if th.api {
			apiError(w, http.StatusForbidden, "forbidden")
		} else {
			http.Error(w, "forbidden", http.StatusForbidden)
		}
		return
	}
	th.next(w, r)
}
