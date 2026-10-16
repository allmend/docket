package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/google/uuid"
)

func (h *Handler) SettingsPage(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "tokens"
	}

	members, _ := h.tokens.ListMembers(r.Context(), orgID)
	tokenList := h.tokenList(r, orgID)

	h.render(w, "settings.html", h.pageData(r, map[string]any{
		"Members": members,
		"Tokens":  tokenList,
		"Tab":     tab,
		// settings.html is shared with the workspace-settings branch; this is
		// the org-level view, which has no team or tracks.
		"Team":   nil,
		"Tracks": nil,
	}))
}

// tokenList loads the org's API tokens with Revocable stamped for the viewer, so
// the revoke button only appears where the server will honour it.
func (h *Handler) tokenList(r *http.Request, orgID uuid.UUID) []model.APIToken {
	tokens, _ := h.tokens.List(r.Context(), orgID)
	userID := service.UserIDFromContext(r.Context())
	role := service.RoleFromContext(r.Context())
	for i := range tokens {
		tokens[i].Revocable = service.CanRevokeToken(&tokens[i], userID, role)
	}
	return tokens
}

func (h *Handler) CreateToken(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())

	if !parseForm(w, r) {
		return
	}

	name := r.FormValue("name")
	scope := model.TokenScope(r.FormValue("scope"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if scope != model.ScopeMetricsRead && scope != model.ScopeAPIRead && scope != model.ScopeAPIWrite {
		http.Error(w, "invalid scope", http.StatusBadRequest)
		return
	}

	plaintext, token, err := h.tokens.Create(r.Context(), orgID, userID, name, scope)
	if err != nil {
		http.Error(w, "failed to create token", http.StatusInternalServerError)
		return
	}

	tokenList := h.tokenList(r, orgID)
	h.render(w, "settings-tokens-partial.html", map[string]any{
		"Tokens":    tokenList,
		"NewToken":  token,
		"Plaintext": plaintext,
	})
}

func (h *Handler) RevokeToken(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	tokenID, ok := pathUUID(w, r, "tokenID")
	if !ok {
		return
	}

	actorID := service.UserIDFromContext(r.Context())
	if err := h.tokens.Revoke(r.Context(), orgID, tokenID, actorID, service.RoleFromContext(r.Context())); err != nil {
		serviceError(w, err, "failed to revoke token")
		return
	}

	tokenList := h.tokenList(r, orgID)
	h.render(w, "settings-tokens-partial.html", map[string]any{
		"Tokens": tokenList,
		// Only shown once, immediately after creation.
		"Plaintext": "",
	})
}

func (h *Handler) CreateMember(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())

	if !parseForm(w, r) {
		return
	}

	username := r.FormValue("username")
	name := r.FormValue("name")
	email := r.FormValue("email")
	password := r.FormValue("password")
	role := r.FormValue("role")
	if role != "admin" && role != "member" {
		role = "member"
	}

	if username == "" || name == "" || email == "" || password == "" {
		http.Error(w, "all fields are required", http.StatusBadRequest)
		return
	}

	if _, err := h.auth.CreateLocalUser(r.Context(), orgID, username, name, email, password, role); err != nil {
		// A duplicate is the admin mistyping, not a server fault — re-render the
		// panel with the reason on it rather than replacing the page with a bare
		// 500. Anything else is genuinely unexpected and stays a 500 with the
		// detail in the log, never in the response.
		if errors.Is(err, service.ErrUserExists) {
			h.renderMembers(w, r, err.Error())
			return
		}
		slog.Error("create member", "org_id", orgID, "err", err)
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	h.renderMembers(w, r, "")
}

// renderMembers re-renders the members panel. Every render of this partial goes
// through here so the data contract stays total — it runs under
// missingkey=error, where a forgotten key is a 500 rather than a blank.
func (h *Handler) renderMembers(w http.ResponseWriter, r *http.Request, errMsg string) {
	orgID := service.OrgIDFromContext(r.Context())
	members, _ := h.tokens.ListMembers(r.Context(), orgID)
	h.render(w, "settings-members-partial.html", map[string]any{
		"Members":     members,
		"CurrentUser": h.auth.GetCurrentUser(r.Context()),
		"Error":       errMsg,
	})
}

func (h *Handler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())

	targetID, ok := pathUUID(w, r, "userID")
	if !ok {
		return
	}
	if !parseForm(w, r) {
		return
	}

	if err := h.tokens.UpdateRole(r.Context(), orgID, targetID, r.FormValue("role")); err != nil {
		serviceError(w, err, "failed to update role")
		return
	}

	h.renderMembers(w, r, "")
}
