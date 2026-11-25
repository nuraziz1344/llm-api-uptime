package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"llm-api-uptime/internal/api"
	"llm-api-uptime/internal/auth"
	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/crypto"
	"llm-api-uptime/internal/db"
	"llm-api-uptime/internal/events"
	"llm-api-uptime/internal/prober"
	"llm-api-uptime/internal/repo"
	"llm-api-uptime/internal/scheduler"
)

func main() {
	cfg := config.Load()

	vault, err := crypto.NewVault(cfg.EncryptionKeyEnv, cfg.DataDir)
	if err != nil {
		log.Fatalf("init secret vault: %v", err)
	}

	store, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open sqlite db: %v", err)
	}
	defer func() { _ = store.Close() }()

	settingsRepo := repo.NewSettingsRepo(store.Writer, store.Reader)
	monitorRepo := repo.NewMonitorRepo(store.Writer, store.Reader, vault)
	heartbeatRepo := repo.NewHeartbeatRepo(store.Writer, store.Reader)

	authSvc := auth.NewService(settingsRepo)
	prb := prober.New()
	hub := events.NewHub()
	sched := scheduler.New(monitorRepo, heartbeatRepo, settingsRepo, prb, hub)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := sched.Start(ctx); err != nil {
		log.Fatalf("start scheduler: %v", err)
	}

	router := api.NewRouter(api.Deps{
		Config:   cfg,
		Auth:     authSvc,
		AuthH:    api.NewAuthHandler(settingsRepo, authSvc),
		MonitorH: api.NewMonitorHandler(monitorRepo, heartbeatRepo, sched, prb),
		PublicH:  api.NewPublicHandler(monitorRepo, heartbeatRepo, settingsRepo, hub),
		SettingH: api.NewSettingsHandler(settingsRepo, hub),
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("LLM API Uptime listening on :%s (db=%s)", cfg.Port, cfg.DBPath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down gracefully...")
	sched.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
