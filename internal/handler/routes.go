package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Config holds all the dependencies which API handlers need
type Config struct {
	Logger *slog.Logger
}

// RegisterRoutes sets up chi and mounts endpoints
func RegisterRoutes(cfg Config) http.Handler {
	r := chi.NewRouter()

	// Base Health endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		Respond(w, cfg.Logger, http.StatusOK, map[string]string{"status": "ok"})
	})

	return r
}
