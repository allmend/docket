package api

import (
	"log/slog"
	"net/http"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
)

func (h *Handler) SettingsPage(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "tokens"
	}

	members, _ := h.tokens.ListMembers(r.Context(), orgID)
	tokenList, _ := h.tokens.List(r.Context(), orgID)

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

	tokenList, _ := h.tokens.List(r.Context(), orgID)
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

	if err := h.tokens.Revoke(r.Context(), orgID, tokenID); err != nil {
		http.Error(w, "failed to revoke token", http.StatusInternalServerError)
		return
	}

	tokenList, _ := h.tokens.List(r.Context(), orgID)
	h.render(w, "settings-tokens-partial.html", map[string]any{
		"Tokens": tokenList,
		// Only shown once, immediately after creation.
		"Plaintext": "",
	})
}

func (h *Handler) CreateMember(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	if service.RoleFromContext(r.Context()) != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

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
		slog.Error("create member", "org_id", orgID, "err", err)
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	members, _ := h.tokens.ListMembers(r.Context(), orgID)
	h.render(w, "settings-members-partial.html", map[string]any{
		"Members":     members,
		"CurrentUser": h.auth.GetCurrentUser(r.Context()),
	})
}

func (h *Handler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	if service.RoleFromContext(r.Context()) != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	targetID, ok := pathUUID(w, r, "userID")
	if !ok {
		return
	}
	if !parseForm(w, r) {
		return
	}

	if err := h.tokens.UpdateRole(r.Context(), orgID, targetID, r.FormValue("role")); err != nil {
		http.Error(w, "failed to update role", http.StatusInternalServerError)
		return
	}

	members, _ := h.tokens.ListMembers(r.Context(), orgID)
	h.render(w, "settings-members-partial.html", map[string]any{
		"Members":     members,
		"CurrentUser": h.auth.GetCurrentUser(r.Context()),
	})
}
