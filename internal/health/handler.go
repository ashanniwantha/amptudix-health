package health

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ashanniwantha/amptudix-health/internal/client/openwearables"
	"github.com/ashanniwantha/amptudix-health/internal/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Timeseries(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUserID(w, r)
	if !ok {
		return
	}

	q := r.URL.Query()
	p := openwearables.TimeseriesParams{
		Types:      q["types"],
		Resolution: q.Get("resolution"),
		Cursor:     q.Get("cursor"),
	}
	p.StartTime = parseTime(q.Get("start_time"))
	p.EndTime = parseTime(q.Get("end_time"))
	p.Limit, _ = strconv.Atoi(q.Get("limit"))

	page, err := h.svc.Timeseries(r.Context(), userID, p)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, page)
}

func (h *Handler) Workouts(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUserID(w, r)
	if !ok {
		return
	}

	q := r.URL.Query()
	p := openwearables.EventParams{
		StartDate: parseTime(q.Get("start_date")),
		EndDate:   parseTime(q.Get("end_date")),
		Cursor:    q.Get("cursor"),
	}
	p.Limit, _ = strconv.Atoi(q.Get("limit"))

	page, err := h.svc.Workouts(r.Context(), userID, p)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, page)
}

func (h *Handler) Sleep(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUserID(w, r)
	if !ok {
		return
	}

	q := r.URL.Query()
	p := openwearables.SleepParams{
		EventParams: openwearables.EventParams{
			StartDate: parseTime(q.Get("start_date")),
			EndDate:   parseTime(q.Get("end_date")),
			Cursor:    q.Get("cursor"),
		},
		FilterByPriority: q.Get("filter_by_priority") == "true",
	}
	p.Limit, _ = strconv.Atoi(q.Get("limit"))

	page, err := h.svc.Sleep(r.Context(), userID, p)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, page)
}

func parseUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_user_id", "user id must be a UUID")
		return uuid.Nil, false
	}
	return id, true
}

func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUserNotFound):
		httpx.WriteError(w, http.StatusNotFound, "user_not_found", err.Error())
	case errors.Is(err, ErrNotLinked):
		httpx.WriteError(w, http.StatusConflict, "not_linked", err.Error())
	default:
		var apiErr *openwearables.APIError
		if errors.As(err, &apiErr) {
			status := http.StatusBadGateway
			if apiErr.StatusCode == http.StatusBadRequest || apiErr.StatusCode == http.StatusNotFound {
				status = apiErr.StatusCode
			}
			httpx.WriteError(w, status, "open_wearables_error", apiErr.Detail)
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not fetch health data")
	}
}
