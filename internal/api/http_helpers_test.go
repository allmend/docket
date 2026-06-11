package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// requestWithParam builds a request carrying a chi URL parameter, the way the
// router would during real dispatch.
func requestWithParam(name, value string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(name, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestPathUUIDValid(t *testing.T) {
	want := uuid.New()
	w := httptest.NewRecorder()
	got, ok := pathUUID(w, requestWithParam("boardID", want.String()), "boardID")
	if !ok {
		t.Fatal("pathUUID rejected a valid UUID")
	}
	if got != want {
		t.Errorf("pathUUID = %s, want %s", got, want)
	}
}

func TestPathUUIDInvalid(t *testing.T) {
	w := httptest.NewRecorder()
	_, ok := pathUUID(w, requestWithParam("boardID", "not-a-uuid"), "boardID")
	if ok {
		t.Fatal("pathUUID accepted garbage")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "invalid board ID") {
		t.Errorf("error body = %q, want it to mention 'invalid board ID'", body)
	}
}

func TestFormDate(t *testing.T) {
	mkReq := func(val string) *http.Request {
		form := url.Values{}
		if val != "" {
			form.Set("start_date", val)
		}
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		_ = r.ParseForm()
		return r
	}

	if got := formDate(mkReq("2026-06-10"), "start_date"); got == nil {
		t.Error("valid date returned nil")
	} else if want := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("formDate = %v, want %v", got, want)
	}

	if got := formDate(mkReq(""), "start_date"); got != nil {
		t.Errorf("missing field should return nil, got %v", got)
	}
	if got := formDate(mkReq("10/06/2026"), "start_date"); got != nil {
		t.Errorf("malformed date should return nil, got %v", got)
	}
}

func TestParseRef(t *testing.T) {
	tests := []struct {
		ref     string
		key     string
		num     int
		wantErr bool
	}{
		{"BE-42", "BE", 42, false},
		{"be-7", "BE", 7, false},       // key is upper-cased
		{"AB-CD-3", "AB-CD", 3, false}, // last dash splits
		{"BE-", "", 0, true},
		{"-42", "", 0, true},
		{"BE-0", "", 0, true},
		{"BE-abc", "", 0, true},
		{"noseparator", "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			key, num, err := parseRef(tt.ref)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseRef(%q) expected error, got %s/%d", tt.ref, key, num)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRef(%q) unexpected error: %v", tt.ref, err)
			}
			if key != tt.key || num != tt.num {
				t.Errorf("parseRef(%q) = %s/%d, want %s/%d", tt.ref, key, num, tt.key, tt.num)
			}
		})
	}
}
