package api

import (
	"fmt"
	"net/http"
	"time"

	"llm-api-uptime/internal/domain"
	"llm-api-uptime/internal/events"
	"llm-api-uptime/internal/repo"
)

type PublicHandler struct {
	monitors   *repo.MonitorRepo
	heartbeats *repo.HeartbeatRepo
	settings   *repo.SettingsRepo
	hub        *events.Hub
}

func NewPublicHandler(
	monitors *repo.MonitorRepo,
	heartbeats *repo.HeartbeatRepo,
	settings *repo.SettingsRepo,
	hub *events.Hub,
) *PublicHandler {
	return &PublicHandler{
		monitors:   monitors,
		heartbeats: heartbeats,
		settings:   settings,
		hub:        hub,
	}
}

func (h *PublicHandler) Status(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.settings.GetAppSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load settings")
		return
	}

	list, err := h.monitors.ListPublic(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load monitors")
		return
	}

	now := time.Now().UTC()
	since24h := now.Add(-24 * time.Hour)
	since30d := now.AddDate(0, 0, -30)

	overall := domain.StatusUp
	pubMonitors := make([]domain.PublicMonitor, 0, len(list))

	for _, m := range list {
		recent, _ := h.heartbeats.ListRecent(r.Context(), m.ID, 48)
		pubRecent := make([]domain.PublicHeartbeat, 0, len(recent))
		for _, hb := range recent {
			pubRecent = append(pubRecent, domain.PublicHeartbeat{
				Status:          hb.Status,
				ModelsLatencyMs: hb.ModelsLatencyMs,
				TTFTMs:          hb.TTFTMs,
				TotalLatencyMs:  hb.TotalLatencyMs,
				TokensPerSec:    hb.TokensPerSec,
				CheckedAt:       hb.CheckedAt,
			})
		}

		var latest *domain.PublicHeartbeat
		if len(pubRecent) > 0 {
			cp := pubRecent[len(pubRecent)-1]
			latest = &cp
			if m.IsActive {
				if cp.Status == domain.StatusDown {
					overall = domain.StatusDown
				} else if cp.Status == domain.StatusDegraded && overall != domain.StatusDown {
					overall = domain.StatusDegraded
				}
			}
		}

		sum24, _ := h.heartbeats.SummarizeWindow(r.Context(), m.ID, since24h)
		sum30, _ := h.heartbeats.SummarizeWindow(r.Context(), m.ID, since30d)

		pubMonitors = append(pubMonitors, domain.PublicMonitor{
			ID:               m.ID,
			Name:             m.Name,
			GroupName:        m.GroupName,
			ProviderType:     m.ProviderType,
			ModelID:          m.ModelID,
			IntervalSec:      m.IntervalSec,
			IsActive:         m.IsActive,
			Latest:           latest,
			RecentHeartbeats: pubRecent,
			Uptime24h:        sum24.UptimePct,
			Uptime30d:        sum30.UptimePct,
			AvgTPS24h:        sum24.AvgTPS,
			AvgTTFT24h:       sum24.AvgTTFTMs,
		})
	}

	writeJSON(w, http.StatusOK, domain.PublicStatusResponse{
		Title:         cfg.PublicPageTitle,
		Description:   cfg.PublicPageDescription,
		OverallStatus: overall,
		Monitors:      pubMonitors,
		GeneratedAt:   now.Format(time.RFC3339),
	})
}

func (h *PublicHandler) Events(w http.ResponseWriter, r *http.Request) {
	ch, unsub := h.hub.SubscribePublic()
	defer unsub()
	streamSSE(w, r, ch)
}

func streamSSE(w http.ResponseWriter, r *http.Request, ch <-chan events.Message) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case msg, open := <-ch:
			if !open {
				return
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", msg.Event, msg.Data)
			flusher.Flush()
		}
	}
}
