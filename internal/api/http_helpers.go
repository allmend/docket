package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// pathUUID parses the named chi URL parameter as a UUID. On failure it writes
// a 400 response and returns false, so callers can simply return.
func pathUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		label := strings.TrimSuffix(name, "ID")
		http.Error(w, "invalid "+label+" ID", http.StatusBadRequest)
		return uuid.Nil, false
	}
	return id, true
}

// formDate parses a YYYY-MM-DD form value. Empty or malformed values return nil —
// date fields are always optional.
func formDate(r *http.Request, field string) *time.Time {
	v := r.FormValue(field)
	if v == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return nil
	}
	return &t
}
