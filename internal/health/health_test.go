package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func probe(t *testing.T, c *Checker, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	c.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Code, rec.Body.String()
}

func ready(c *Checker) {
	c.MarkMigrated()
	c.SetPing(func(context.Context) error { return nil })
}

func TestLivezIgnoresEverything(t *testing.T) {
	c := &Checker{}
	c.SetPing(func(context.Context) error { return errors.New("db down") })
	c.Drain()
	if code, body := probe(t, c, "/livez"); code != http.StatusOK || body != "ok\n" {
		t.Fatalf("livez = %d %q, want 200 ok even with the DB down and draining", code, body)
	}
}

func TestReadyz(t *testing.T) {
	cases := []struct {
		name     string
		setup    func(*Checker)
		wantCode int
		wantFail string
	}{
		{"fresh checker is not ready", func(*Checker) {}, http.StatusServiceUnavailable, "[-]migrations failed"},
		{"migrated but no pool", func(c *Checker) { c.MarkMigrated() }, http.StatusServiceUnavailable, "[-]database failed: not connected"},
		{"database down", func(c *Checker) {
			c.MarkMigrated()
			c.SetPing(func(context.Context) error { return errors.New("connection refused") })
		}, http.StatusServiceUnavailable, "[-]database failed: connection refused"},
		{"draining", func(c *Checker) { ready(c); c.Drain() }, http.StatusServiceUnavailable, "[-]shutdown failed: shutting down"},
		{"ready", ready, http.StatusOK, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Checker{}
			tc.setup(c)
			code, body := probe(t, c, "/readyz")
			if code != tc.wantCode {
				t.Fatalf("code = %d, want %d (body %q)", code, tc.wantCode, body)
			}
			if tc.wantFail != "" && !strings.Contains(body, tc.wantFail) {
				t.Fatalf("body %q does not name the failure %q", body, tc.wantFail)
			}
			if tc.wantFail == "" && body != "ok\n" {
				t.Fatalf("body = %q, want ok", body)
			}
		})
	}
}

func TestReadyzVerbose(t *testing.T) {
	c := &Checker{}
	ready(c)
	_, body := probe(t, c, "/readyz?verbose")
	for _, want := range []string{"[+]migrations ok", "[+]database ok", "[+]shutdown ok", "readyz ok"} {
		if !strings.Contains(body, want) {
			t.Errorf("verbose body %q missing %q", body, want)
		}
	}
}

func TestReadyzPingIsBounded(t *testing.T) {
	c := &Checker{}
	c.MarkMigrated()
	c.SetPing(func(ctx context.Context) error {
		if _, ok := ctx.Deadline(); !ok {
			return errors.New("ping has no deadline")
		}
		return nil
	})
	if code, body := probe(t, c, "/readyz"); code != http.StatusOK {
		t.Fatalf("readyz = %d %q", code, body)
	}
}
