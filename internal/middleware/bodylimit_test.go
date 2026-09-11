package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLimitBodyRejectsOversizedBody(t *testing.T) {
	var readErr error
	h := LimitBody(8)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader("short")))
	if readErr != nil {
		t.Fatalf("body within the limit: got %v", readErr)
	}

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader("well over eight bytes")))
	if readErr == nil {
		t.Fatal("body over the limit was read in full")
	}
}
