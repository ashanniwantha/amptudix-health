package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorResponse is the single error envelope every endpoint returns.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteJSON encodes v and writes it with the given status code.
// If encoding fails, it falls back to a 500 because the status code
// has already been written and cannot be changed.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("httpx: encode response", "error", err)
	}
}

// WriteError writes the standard error envelope.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorResponse{Code: code, Message: message})
}
