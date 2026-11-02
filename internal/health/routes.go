package health

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/users/{userID}/health", func(r chi.Router) {
		r.Get("/timeseries", h.Timeseries)
		r.Get("/workouts", h.Workouts)
		r.Get("/sleep", h.Sleep)
	})
}
