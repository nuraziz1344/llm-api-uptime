package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"llm-api-uptime/internal/auth"
	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/crypto"
	"llm-api-uptime/internal/db"
	"llm-api-uptime/internal/domain"
	"llm-api-uptime/internal/events"
	"llm-api-uptime/internal/prober"
	"llm-api-uptime/internal/repo"
	"llm-api-uptime/internal/scheduler"
)

func setupTestServer(t *testing.T) (http.Handler, func()) {
	t.Helper()
	dir := t.TempDir()
	vault, err := crypto.NewVault("", dir)
	if err != nil {
		t.Fatalf("vault: %v", err)
	}
	store, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	settingsRepo := repo.NewSettingsRepo(store.Writer, store.Reader)
	monitorRepo := repo.NewMonitorRepo(store.Writer, store.Reader, vault)
	heartbeatRepo := repo.NewHeartbeatRepo(store.Writer, store.Reader)

	authSvc := auth.NewService(settingsRepo)
	prb := prober.New()
	hub := events.NewHub()
	sched := scheduler.New(monitorRepo, heartbeatRepo, settingsRepo, prb, hub)

	router := NewRouter(Deps{
		Config:   config.Config{DataDir: dir},
		Auth:     authSvc,
		AuthH:    NewAuthHandler(settingsRepo, authSvc),
		MonitorH: NewMonitorHandler(monitorRepo, heartbeatRepo, sched, prb),
		PublicH:  NewPublicHandler(monitorRepo, heartbeatRepo, settingsRepo, hub),
		SettingH: NewSettingsHandler(settingsRepo, hub),
	})

	return router, func() {
		sched.Stop()
		_ = store.Close()
	}
}

func TestAuthAndPublicStatusIsolation(t *testing.T) {
	router, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Setup admin
	setupBody := `{"username":"admin","password":"supersecret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(setupBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 on setup, got %d: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected session cookie after setup")
	}

	// 2. Unauthenticated monitor creation must fail
	reqUnauth := httptest.NewRequest(http.MethodPost, "/api/monitors", strings.NewReader(`{}`))
	recUnauth := httptest.NewRecorder()
	router.ServeHTTP(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without cookie, got %d", recUnauth.Code)
	}

	// 3. Create one public and one private monitor
	createMonitor := func(name string, isPublic bool, baseURL, apiKey string) {
		payload, _ := json.Marshal(map[string]any{
			"name":          name,
			"provider_type": "openai",
			"base_url":      baseURL,
			"api_key":       apiKey,
			"model_id":      "gpt-4o-mini",
			"is_public":     isPublic,
		})
		r := httptest.NewRequest(http.MethodPost, "/api/monitors", bytes.NewReader(payload))
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(cookies[0])
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 creating %s, got %d: %s", name, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), apiKey) {
			t.Fatalf("response leaked raw api key")
		}
	}

	createMonitor("Public OpenAI", true, "https://internal-secret-gw.example.com/v1", "sk-live-secret-123456")
	createMonitor("Private Endpoint", false, "https://private-only.example.com/v1", "sk-private-987654")

	// 4. Public status page must only expose public monitor and never leak base_url or api_key
	pubReq := httptest.NewRequest(http.MethodGet, "/api/public/status", nil)
	pubRec := httptest.NewRecorder()
	router.ServeHTTP(pubRec, pubReq)
	if pubRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on public status, got %d", pubRec.Code)
	}
	body := pubRec.Body.String()
	if strings.Contains(body, "Private Endpoint") {
		t.Fatalf("public status leaked private monitor")
	}
	if strings.Contains(body, "internal-secret-gw.example.com") || strings.Contains(body, "sk-live-secret") {
		t.Fatalf("public status leaked internal base_url or api key")
	}

	var pubResp domain.PublicStatusResponse
	if err := json.Unmarshal(pubRec.Body.Bytes(), &pubResp); err != nil {
		t.Fatalf("decode public status: %v", err)
	}
	if len(pubResp.Monitors) != 1 || pubResp.Monitors[0].Name != "Public OpenAI" {
		t.Fatalf("unexpected public monitors: %+v", pubResp.Monitors)
	}
}
