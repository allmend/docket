package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
)

// TestRequireScope checks the scope ladder: metrics:read < api:read < api:write.
// A principal must hold at least the required scope; anything lower is 403.
func TestRequireScope(t *testing.T) {
	cases := []struct {
		name     string
		have     model.TokenScope
		required model.TokenScope
		wantCode int
	}{
		{"write token hits write route", model.ScopeAPIWrite, model.ScopeAPIWrite, http.StatusOK},
		{"write token hits read route", model.ScopeAPIWrite, model.ScopeAPIRead, http.StatusOK},
		{"read token hits read route", model.ScopeAPIRead, model.ScopeAPIRead, http.StatusOK},
		{"read token blocked from write route", model.ScopeAPIRead, model.ScopeAPIWrite, http.StatusForbidden},
		{"metrics token blocked from read route", model.ScopeMetricsRead, model.ScopeAPIRead, http.StatusForbidden},
		{"metrics token blocked from write route", model.ScopeMetricsRead, model.ScopeAPIWrite, http.StatusForbidden},
		{"metrics token hits metrics route", model.ScopeMetricsRead, model.ScopeMetricsRead, http.StatusOK},
		{"empty scope blocked from metrics route", model.TokenScope(""), model.ScopeMetricsRead, http.StatusForbidden},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := RequireScope(tc.required)(next)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = req.WithContext(service.WithScope(req.Context(), tc.have))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.wantCode {
				t.Errorf("have=%q required=%q: got %d, want %d", tc.have, tc.required, rec.Code, tc.wantCode)
			}
		})
	}
}
