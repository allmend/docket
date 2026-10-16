package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// walkTiers lists every route in both route sets with the tier it declared.
// A route registered without the routes helper has no tier and is reported
// as TierUnset.
func walkTiers(t *testing.T) map[string]model.Tier {
	t.Helper()
	h := &Handler{}
	got := map[string]model.Tier{}
	for prefix, mount := range map[string]func(chi.Router){"": h.Routes, "/api/v1": h.V1Routes} {
		r := chi.NewRouter()
		mount(r)
		err := chi.Walk(r, func(method, route string, handler http.Handler, _ ...func(http.Handler) http.Handler) error {
			tier := model.TierUnset
			if th, ok := handler.(tierHandler); ok {
				tier = th.tier
			}
			got[method+" "+prefix+route] = tier
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return got
}

// TestEveryRouteDeclaresTier is the seam's guarantee: a route added with
// r.Get/r.Post instead of the routes helper, or with TierUnset, fails here.
// (At runtime CanDo denies TierUnset, so such a route would be closed.)
func TestEveryRouteDeclaresTier(t *testing.T) {
	routes := walkTiers(t)
	if len(routes) < 100 {
		t.Fatalf("walked only %d routes; is the walk broken?", len(routes))
	}
	for route, tier := range routes {
		if tier == model.TierUnset {
			t.Errorf("%s has no tier: register it through routes{} with model.TierRead/Write/Admin", route)
		}
	}
}

// TestAdminRoutes pins which routes need an org admin, so widening or
// narrowing that set is a deliberate change to this list.
func TestAdminRoutes(t *testing.T) {
	want := []string{
		"POST /settings/members",
		"PUT /settings/members/{userID}/role",
	}
	var got []string
	for route, tier := range walkTiers(t) {
		if tier == model.TierAdmin {
			got = append(got, route)
		}
	}
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("admin routes changed:\n got: %v\nwant: %v", got, want)
	}
}

// TestReadRoutesAreGETs keeps writes from hiding behind TierRead: a read
// token must never reach a mutating verb.
func TestReadRoutesAreGETs(t *testing.T) {
	for route, tier := range walkTiers(t) {
		if (tier == model.TierRead || tier == model.TierMetrics) && !strings.HasPrefix(route, "GET ") {
			t.Errorf("%s is %s-tier but not a GET", route, tier)
		}
	}
}

func TestTierHandlerDenies(t *testing.T) {
	ran := false
	next := func(w http.ResponseWriter, r *http.Request) { ran = true }

	cases := []struct {
		name     string
		role     string
		tier     model.Tier
		api      bool
		wantCode int
		wantJSON bool
	}{
		{"member on admin route", "member", model.TierAdmin, false, http.StatusForbidden, false},
		{"member on admin API route gets JSON", "member", model.TierAdmin, true, http.StatusForbidden, true},
		{"unset tier is closed", "admin", model.TierUnset, false, http.StatusForbidden, false},
		{"admin on admin route", "admin", model.TierAdmin, false, http.StatusOK, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ran = false
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			ctx := service.WithIdentity(req.Context(), uuid.New(), uuid.New(), tc.role)
			ctx = service.WithScope(ctx, model.ScopeAPIWrite)
			ctx = context.WithValue(ctx, chi.RouteCtxKey, chi.NewRouteContext())
			rec := httptest.NewRecorder()
			tierHandler{tier: tc.tier, api: tc.api, next: next}.ServeHTTP(rec, req.WithContext(ctx))

			if rec.Code != tc.wantCode {
				t.Fatalf("got %d, want %d", rec.Code, tc.wantCode)
			}
			if ran != (tc.wantCode == http.StatusOK) {
				t.Fatalf("handler ran=%v with status %d", ran, rec.Code)
			}
			if isJSON := strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json"); isJSON != tc.wantJSON {
				t.Fatalf("JSON response=%v, want %v", isJSON, tc.wantJSON)
			}
		})
	}
}
