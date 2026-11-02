package user

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ashanniwantha/amptudix-health/internal/client/openwearables"
	"github.com/ashanniwantha/amptudix-health/internal/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type createRequest struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type userResponse struct {
	ID        uuid.UUID  `json:"id"`
	OWID      *uuid.UUID `json:"ow_id,omitempty"`
	Username  string     `json:"username"`
	Email     string     `json:"email"`
	FirstName string     `json:"first_name"`
	LastName  string     `json:"last_name"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	u, err := h.svc.Create(r.Context(), CreateInput{
		Username:  req.Username,
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, toResponse(u))
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrEmailTaken):
		httpx.WriteError(w, http.StatusConflict, "email_taken", err.Error())
	case errors.Is(err, ErrUsernameTaken):
		httpx.WriteError(w, http.StatusConflict, "username_taken", err.Error())
	case errors.Is(err, ErrInvalidEmail),
		errors.Is(err, ErrInvalidUsername),
		errors.Is(err, ErrInvalidPassword),
		errors.Is(err, ErrInvalidName),
		errors.Is(err, ErrInvalidDOB):
		httpx.WriteError(w, http.StatusBadRequest, "validation_failed", err.Error())
	case errors.Is(err, ErrOpenWearablesSync):
		httpx.WriteError(w, http.StatusBadGateway, "open_wearables_unavailable", "could not create user in Open Wearables")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "something went wrong")
	}
}

func (h *Handler) CreateSDKToken(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_user_id", "user id must be a UUID")
		return
	}

	res, err := h.svc.CreateSDKToken(r.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			httpx.WriteError(w, http.StatusNotFound, "user_not_found", err.Error())
		case errors.Is(err, ErrNotLinked):
			httpx.WriteError(w, http.StatusConflict, "not_linked", "user has not been linked to Open Wearables")
		default:
			var apiErr *openwearables.APIError
			if errors.As(err, &apiErr) {
				httpx.WriteError(w, http.StatusBadGateway, "open_wearables_error", apiErr.Detail)
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not create SDK token")
		}
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"userId":       res.OpenWearablesUserID,
		"accessToken":  res.Token.AccessToken,
		"refreshToken": res.Token.RefreshToken,
		"tokenType":    res.Token.TokenType,
		"expiresIn":    res.Token.ExpiresIn,
	})
}

func toResponse(u *User) userResponse {
	return userResponse{
		ID:        u.ID,
		OWID:      u.OW_ID,
		Username:  u.Username,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
	}
}

// ensure chi is imported if you add more routes here
var _ = chi.NewRouter
