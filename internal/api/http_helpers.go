package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
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

// parseForm parses the request form. On failure it writes a 400 response and
// returns false, so callers can simply return.
func parseForm(w http.ResponseWriter, r *http.Request) bool {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return false
	}
	return true
}

// ticketFromPath parses {ticketID} from the URL and loads the ticket for the
// org on the request context. On failure it writes the error response and
// returns false, so callers can simply return.
func (h *Handler) ticketFromPath(w http.ResponseWriter, r *http.Request) (*model.Ticket, uuid.UUID, bool) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return nil, uuid.Nil, false
	}
	ticket, err := h.tickets.GetTicket(r.Context(), orgID, ticketID)
	if err != nil {
		http.Error(w, "ticket not found", http.StatusNotFound)
		return nil, uuid.Nil, false
	}
	return ticket, orgID, true
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
