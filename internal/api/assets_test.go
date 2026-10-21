package api

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHashCacheFollowsContent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "app.css")
	if err := os.WriteFile(p, []byte("a{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &hashCache{entries: map[string]hashEntry{}}
	first := c.hash(p)
	if first == "missing" || first != c.hash(p) {
		t.Fatalf("unstable hash %q", first)
	}
	if err := os.WriteFile(p, []byte("b{color:red}"), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Second)
	_ = os.Chtimes(p, later, later)
	if c.hash(p) == first {
		t.Error("hash did not change with the file's content")
	}
	if c.hash(filepath.Join(t.TempDir(), "nope.js")) != "missing" {
		t.Error("a missing file should hash to \"missing\"")
	}
}
