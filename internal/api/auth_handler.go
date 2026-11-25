package api

import (
	"net/http"
	"strings"

	"llm-api-uptime/internal/auth"
	"llm-api-uptime/internal/repo"
)

type AuthHandler struct {
	settings *repo.SettingsRepo
	auth     *auth.Service
}

func NewAuthHandler(settings *repo.SettingsRepo, authSvc *auth.Service) *AuthHandler {
	return &AuthHandler{settings: settings, auth: authSvc}
}

func (h *AuthHandler) Status(w http.ResponseWriter, r *http.Request) {
	completed, adminUser, err := h.settings.IsSetupCompleted(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check setup status")
		return
	}
	user, authenticated := h.auth.ValidateRequest(r)
	if !authenticated {
		user = ""
		if !completed {
			adminUser = ""
		}
	} else {
		adminUser = user
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"setup_completed": completed,
		"authenticated":   authenticated,
		"username":        adminUser,
	})
}

func (h *AuthHandler) Setup(w http.ResponseWriter, r *http.Request) {
	completed, _, err := h.settings.IsSetupCompleted(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check setup")
		return
	}
	if completed {
		writeError(w, http.StatusConflict, "setup already completed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) < 2 {
		writeError(w, http.StatusBadRequest, "username must be at least 2 characters")
		return
	}
	if len(req.Password) < 6 {
		writeError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	secret, err := auth.GenerateJWTSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate jwt secret")
		return
	}
	if err := h.settings.SaveAdminSetup(r.Context(), req.Username, hash, secret); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save admin account")
		return
	}

	token, err := h.auth.IssueToken(r.Context(), req.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue session")
		return
	}
	auth.SetSessionCookie(w, r, token)
	writeJSON(w, http.StatusCreated, map[string]any{
		"username":      req.Username,
		"authenticated": true,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	completed, adminUser, err := h.settings.IsSetupCompleted(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check setup")
		return
	}
	if !completed {
		writeError(w, http.StatusPreconditionRequired, "setup required")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	hash, ok, err := h.settings.Get(r.Context(), "admin_password_hash")
	if err != nil || !ok {
		writeError(w, http.StatusInternalServerError, "admin credentials missing")
		return
	}

	if strings.TrimSpace(req.Username) != adminUser || !auth.CheckPassword(hash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	token, err := h.auth.IssueToken(r.Context(), adminUser)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue session")
		return
	}
	auth.SetSessionCookie(w, r, token)
	writeJSON(w, http.StatusOK, map[string]any{
		"username":      adminUser,
		"authenticated": true,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	auth.ClearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.NewPassword) < 6 {
		writeError(w, http.StatusBadRequest, "new password must be at least 6 characters")
		return
	}

	hash, ok, err := h.settings.Get(r.Context(), "admin_password_hash")
	if err != nil || !ok || !auth.CheckPassword(hash, req.CurrentPassword) {
		writeError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	if err := h.settings.Set(r.Context(), "admin_password_hash", newHash); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
