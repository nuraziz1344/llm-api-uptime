package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"llm-api-uptime/internal/domain"
	"llm-api-uptime/internal/prober"
	"llm-api-uptime/internal/repo"
	"llm-api-uptime/internal/scheduler"
)

type MonitorHandler struct {
	monitors   *repo.MonitorRepo
	heartbeats *repo.HeartbeatRepo
	scheduler  *scheduler.Scheduler
	prober     *prober.Prober
}

func NewMonitorHandler(
	monitors *repo.MonitorRepo,
	heartbeats *repo.HeartbeatRepo,
	sched *scheduler.Scheduler,
	prb *prober.Prober,
) *MonitorHandler {
	return &MonitorHandler{
		monitors:   monitors,
		heartbeats: heartbeats,
		scheduler:  sched,
		prober:     prb,
	}
}

func (h *MonitorHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.monitors.ListAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list monitors")
		return
	}

	since24h := time.Now().UTC().Add(-24 * time.Hour)
	out := make([]domain.MonitorOverview, 0, len(list))
	for _, m := range list {
		recent, err := h.heartbeats.ListRecent(r.Context(), m.ID, 40)
		if err != nil || recent == nil {
			recent = []domain.Heartbeat{}
		}
		var latest *domain.Heartbeat
		if len(recent) > 0 {
			cp := recent[len(recent)-1]
			latest = &cp
		}
		summary, err := h.heartbeats.SummarizeWindow(r.Context(), m.ID, since24h)
		if err != nil {
			summary = repo.WindowSummary{UptimePct: 100}
		}
		out = append(out, domain.MonitorOverview{
			Monitor:          m,
			Latest:           latest,
			RecentHeartbeats: recent,
			Uptime24h:        summary.UptimePct,
			AvgTPS24h:        summary.AvgTPS,
			AvgTTFT24h:       summary.AvgTTFTMs,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *MonitorHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in domain.MonitorInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validateMonitorInput(in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	m, err := h.monitors.Create(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create monitor")
		return
	}
	h.scheduler.Upsert(m)
	writeJSON(w, http.StatusCreated, m)
}

func (h *MonitorHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := h.monitors.GetByID(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load monitor")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *MonitorHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var in domain.MonitorInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validateMonitorInput(in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	m, err := h.monitors.Update(r.Context(), id, in)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update monitor")
		return
	}
	h.scheduler.Upsert(m)
	writeJSON(w, http.StatusOK, m)
}

func (h *MonitorHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.monitors.Delete(r.Context(), id); errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete monitor")
		return
	}
	h.scheduler.Remove(id)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *MonitorHandler) SetPause(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	m, err := h.monitors.SetActive(r.Context(), id, req.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update monitor status")
		return
	}
	h.scheduler.Upsert(m)
	writeJSON(w, http.StatusOK, m)
}

func (h *MonitorHandler) CheckNow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := h.monitors.GetByID(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load monitor")
		return
	}
	hb, err := h.scheduler.TriggerNow(r.Context(), m)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save check")
		return
	}
	writeJSON(w, http.StatusOK, hb)
}

func (h *MonitorHandler) DiscoverModels(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MonitorID        string              `json:"monitor_id"`
		ProviderType     domain.ProviderType `json:"provider_type"`
		BaseURL          string              `json:"base_url"`
		APIKey           string              `json:"api_key"`
		AnthropicVersion string              `json:"anthropic_version"`
		UserAgent        string              `json:"user_agent"`
		CustomHeaders    map[string]string   `json:"custom_headers"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.BaseURL) == "" {
		writeError(w, http.StatusBadRequest, "base_url is required")
		return
	}

	apiKey := strings.TrimSpace(req.APIKey)
	headers := req.CustomHeaders
	if req.MonitorID != "" {
		if existing, err := h.monitors.GetByID(r.Context(), req.MonitorID); err == nil {
			if apiKey == "" {
				apiKey = existing.APIKey
			}
			if len(headers) == 0 {
				headers = existing.CustomHeaders
			}
		}
	}

	models, latMs, err := h.prober.DiscoverModels(
		r.Context(),
		req.ProviderType,
		req.BaseURL,
		apiKey,
		req.AnthropicVersion,
		req.UserAgent,
		headers,
	)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"models":     models,
		"latency_ms": latMs,
	})
}

func (h *MonitorHandler) TestProbe(w http.ResponseWriter, r *http.Request) {
	var in struct {
		MonitorID string `json:"monitor_id"`
		domain.MonitorInput
	}
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(in.BaseURL) == "" || strings.TrimSpace(in.ModelID) == "" {
		writeError(w, http.StatusBadRequest, "base_url and model_id are required")
		return
	}

	m := domain.Monitor{
		ProviderType:      in.ProviderType,
		OpenAIEndpoint:    in.OpenAIEndpoint,
		BaseURL:           strings.TrimRight(strings.TrimSpace(in.BaseURL), "/"),
		ModelID:           strings.TrimSpace(in.ModelID),
		CheckModelsFirst:  true,
		StreamEnabled:     true,
		FallbackNonStream: true,
		Prompt:            strings.TrimSpace(in.Prompt),
		MaxTokens:         in.MaxTokens,
		TimeoutSec:        in.TimeoutSec,
		AnthropicVersion:  in.AnthropicVersion,
		UserAgent:         strings.TrimSpace(in.UserAgent),
		CustomHeaders:     in.CustomHeaders,
	}
	if in.CheckModelsFirst != nil {
		m.CheckModelsFirst = *in.CheckModelsFirst
	}
	if in.StreamEnabled != nil {
		m.StreamEnabled = *in.StreamEnabled
	}
	if in.FallbackNonStream != nil {
		m.FallbackNonStream = *in.FallbackNonStream
	}
	if m.Prompt == "" {
		m.Prompt = "Count from 1 to 15 separated by spaces."
	}
	if m.MaxTokens <= 0 {
		m.MaxTokens = 24
	}
	if m.TimeoutSec <= 0 {
		m.TimeoutSec = 30
	}
	if in.APIKey != nil && strings.TrimSpace(*in.APIKey) != "" {
		m.APIKey = strings.TrimSpace(*in.APIKey)
	} else if in.MonitorID != "" {
		if existing, err := h.monitors.GetByID(r.Context(), in.MonitorID); err == nil {
			m.APIKey = existing.APIKey
			if len(m.CustomHeaders) == 0 {
				m.CustomHeaders = existing.CustomHeaders
			}
		}
	}

	hb := h.prober.Probe(r.Context(), m)
	writeJSON(w, http.StatusOK, hb)
}

func (h *MonitorHandler) Heartbeats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rng := r.URL.Query().Get("range")
	now := time.Now().UTC()

	switch rng {
	case "7d", "30d":
		days := 7
		if rng == "30d" {
			days = 30
		}
		stats, err := h.heartbeats.ListHourlySince(r.Context(), id, now.AddDate(0, 0, -days))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load hourly stats")
			return
		}
		recent, _ := h.heartbeats.ListRecent(r.Context(), id, 60)
		writeJSON(w, http.StatusOK, map[string]any{
			"mode":         "hourly",
			"hourly_stats": stats,
			"heartbeats":   recent,
		})
	default:
		since := now.Add(-24 * time.Hour)
		if rng == "1h" {
			since = now.Add(-1 * time.Hour)
		}
		items, err := h.heartbeats.ListSince(r.Context(), id, since, 1000)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load heartbeats")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"mode":       "raw",
			"heartbeats": items,
		})
	}
}

func validateMonitorInput(in domain.MonitorInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return errors.New("name is required")
	}
	if in.ProviderType != domain.ProviderOpenAI && in.ProviderType != domain.ProviderAnthropic {
		return errors.New("provider_type must be 'openai' or 'anthropic'")
	}
	if in.OpenAIEndpoint != "" && in.OpenAIEndpoint != "chat_completions" && in.OpenAIEndpoint != "responses" {
		return errors.New("openai_endpoint must be 'chat_completions' or 'responses'")
	}
	u := strings.TrimSpace(in.BaseURL)
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return errors.New("base_url must start with http:// or https://")
	}
	if strings.TrimSpace(in.ModelID) == "" {
		return errors.New("model_id is required")
	}
	return nil
}
