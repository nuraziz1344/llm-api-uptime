package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"llm-api-uptime/internal/auth"
	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/webui"
)

type Deps struct {
	Config   config.Config
	Auth     *auth.Service
	AuthH    *AuthHandler
	MonitorH *MonitorHandler
	PublicH  *PublicHandler
	SettingH *SettingsHandler
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	if d.Config.DevMode {
		r.Use(corsDevMiddleware(d.Config.AllowedOrigin))
	}

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api", func(api chi.Router) {
		// Public auth endpoints
		api.Get("/auth/status", d.AuthH.Status)
		api.Post("/auth/setup", d.AuthH.Setup)
		api.Post("/auth/login", d.AuthH.Login)

		// Public status page endpoints
		api.Get("/public/status", d.PublicH.Status)
		api.Get("/public/events", d.PublicH.Events)

		// Admin-protected endpoints
		api.Group(func(priv chi.Router) {
			priv.Use(d.Auth.RequireAdmin)

			priv.Post("/auth/logout", d.AuthH.Logout)
			priv.Put("/auth/password", d.AuthH.ChangePassword)

			priv.Get("/monitors", d.MonitorH.List)
			priv.Post("/monitors", d.MonitorH.Create)
			priv.Post("/monitors/discover-models", d.MonitorH.DiscoverModels)
			priv.Post("/monitors/test", d.MonitorH.TestProbe)

			priv.Get("/monitors/{id}", d.MonitorH.Get)
			priv.Put("/monitors/{id}", d.MonitorH.Update)
			priv.Delete("/monitors/{id}", d.MonitorH.Delete)
			priv.Post("/monitors/{id}/pause", d.MonitorH.SetPause)
			priv.Post("/monitors/{id}/check", d.MonitorH.CheckNow)
			priv.Get("/monitors/{id}/heartbeats", d.MonitorH.Heartbeats)

			priv.Get("/settings", d.SettingH.Get)
			priv.Put("/settings", d.SettingH.Update)
			priv.Get("/events", d.SettingH.AdminEvents)
		})
	})

	r.Handle("/*", webui.Handler())
	return r
}

func corsDevMiddleware(origin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
