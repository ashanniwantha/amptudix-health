package user

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/users", h.Create)
}

func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	r.Post("/users/{userID}/sdk-token", h.CreateSDKToken)
}
