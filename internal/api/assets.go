package api

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StaticDir is where the built assets live; /static/* serves it.
const StaticDir = "static/dist"

// assetURL returns /static/<name>?v=<content hash>. The URL changes whenever
// the file does, so /static can tell browsers to cache it forever. The hash is
// cached per file and recomputed when the size or mtime changes, which keeps
// dev live-reload (esbuild and Tailwind watchers rewriting files) correct.
func assetURL(name string) string {
	return "/static/" + name + "?v=" + assetHashes.hash(filepath.Join(StaticDir, name))
}

var assetHashes = &hashCache{entries: map[string]hashEntry{}}

type hashEntry struct {
	size    int64
	modTime time.Time
	sum     string
}

type hashCache struct {
	mu      sync.Mutex
	entries map[string]hashEntry
}

func (c *hashCache) hash(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return "missing"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[path]; ok && e.size == fi.Size() && e.modTime.Equal(fi.ModTime()) {
		return e.sum
	}
	f, err := os.Open(path)
	if err != nil {
		return "missing"
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "missing"
	}
	sum := hex.EncodeToString(h.Sum(nil))[:12]
	c.entries[path] = hashEntry{size: fi.Size(), modTime: fi.ModTime(), sum: sum}
	return sum
}
