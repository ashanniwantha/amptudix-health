package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// DecodeJSON reads the request body, enforces a 1 MB size limit, rejects
// unknown fields, and decodes the JSON payload into dst. On failure it
// writes a 400 response and returns false — the caller must return
// immediately when DecodeJSON returns false.
//
// The DisallowUnknownFields call is what catches typos like "passwrod"
// instead of "password": without it, the decoder silently ignores unknown
// keys and the handler proceeds with a zero-value field.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		var maxBytesErr *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxErr):
			WriteError(w, http.StatusBadRequest, "malformed_json",
				"request body contains invalid JSON")
		case errors.As(err, &typeErr):
			WriteError(w, http.StatusBadRequest, "invalid_field_type",
				fmt.Sprintf("field %q has the wrong type", typeErr.Field))
		case errors.As(err, &maxBytesErr):
			WriteError(w, http.StatusRequestEntityTooLarge, "body_too_large",
				"request body exceeds 1 MB")
		case errors.Is(err, io.EOF):
			WriteError(w, http.StatusBadRequest, "empty_body",
				"request body is required")
		default:
			WriteError(w, http.StatusBadRequest, "invalid_body",
				"could not parse request body")
		}
		return false
	}
	return true
}

// ParseUUIDParam reads a chi URL parameter and parses it as a UUID.
// On failure it writes a 400 response and returns false.
func ParseUUIDParam(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	raw := chi.URLParam(r, key)
	id, err := uuid.Parse(raw)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_uuid",
			fmt.Sprintf("%s must be a UUID", key))
		return uuid.Nil, false
	}
	return id, true
}
