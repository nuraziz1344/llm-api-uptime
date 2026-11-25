package api

import (
	"net/http"

	"llm-api-uptime/internal/domain"
	"llm-api-uptime/internal/events"
	"llm-api-uptime/internal/repo"
)

type SettingsHandler struct {
	settings *repo.SettingsRepo
	hub      *events.Hub
}

func NewSettingsHandler(settings *repo.SettingsRepo, hub *events.Hub) *SettingsHandler {
	return &SettingsHandler{settings: settings, hub: hub}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.settings.GetAppSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load settings")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var in domain.AppSettings
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.settings.UpdateAppSettings(r.Context(), in); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update settings")
		return
	}
	updated, _ := h.settings.GetAppSettings(r.Context())
	writeJSON(w, http.StatusOK, updated)
}

func (h *SettingsHandler) AdminEvents(w http.ResponseWriter, r *http.Request) {
	ch, unsub := h.hub.SubscribeAdmin()
	defer unsub()
	streamSSE(w, r, ch)
}
