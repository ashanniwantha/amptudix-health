package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// Respond serializes data as JSON payload and writes it to the response writer
// with the provided HTTP status code. If serialization fails, the error is logged.
func Respond(w http.ResponseWriter, logger *slog.Logger, status int, data any) {
	w.Header().Set("content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			logger.Error("failed to encode respond payload", "err", err)
		}
	}
}

// RespondError wraps a string error message into a standardized JSON object
// and writes it to the response writer using Respond helper.
func RespondError(w http.ResponseWriter, logger *slog.Logger, status int, msg string) {
	Respond(w, logger, status, map[string]string{"error": msg})
}

// DecodeJSON reads the request body, enforces a size limit, disallow unknown fields,
// and decodes the JSON payload into a destination object. It automatically handles and
// responds to malformed requests, return true on success and false if decoding fails.
func DecodeJSON(w http.ResponseWriter, r *http.Request, logger *slog.Logger, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			RespondError(w, logger, http.StatusBadRequest, "malformed json syntax")
		case errors.As(err, &unmarshalTypeError):
			RespondError(w, logger, http.StatusBadRequest, "invalid field type for field: "+unmarshalTypeError.Field)
		case errors.Is(err, io.EOF):
			RespondError(w, logger, http.StatusBadRequest, "request body cannot be empty")
		default:
			RespondError(w, logger, http.StatusBadRequest, "invalid request configuration")
		}
		return false
	}
	return true
}

// ParseIDParam extracts a URL routing parameter by it's key and attemp to parse it
// into a positive 64-bit integer. It returns an error if the key is missing,
// poorly formatted or less than or equals to zero.
func ParseIDParam(r *http.Request, key string) (int64, error) {
	idStr := chi.URLParam(r, key)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid identifier format")
	}
	return id, nil
}
