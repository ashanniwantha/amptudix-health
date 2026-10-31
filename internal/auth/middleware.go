package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// contextKey is an unexported struct type used as key
// for the values we stash in the request context.
type contextKey struct {
	name string
}

var (
	userIDKey    = contextKey{"userID"}
	sessionIDKey = contextKey{"sessionID"}
)

// UserIDFromContext returns the authenticated user's UUID,
// if the request passes through Authenticate. The bool is false
// when the middleware did not run, which would only happen on routes
// that were explicitly mounted outside the protected group
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}

// SessionIDFromContext returns the UUID of the user_sessions row that this
// access token belongs to. Callers that need to revoke (logout) or re-check
// session-liveness use this to find the right row.
func SessionIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(sessionIDKey).(uuid.UUID)
	return id, ok
}

// Authenticate returns an http middleware that:
//
//  1. Reads the Authorization header and requires "Bearer <jwt>".
//  2. Validates the JWT via TokenGenerator.ValidateAccessToken.
//  3. Injects the user UUID and session UUID into the request context.
//  4. Calls the next handler.
//
// On any failure it writes a JSON error envelope and stops the chain and
// the downstream handler never invoked, which makes it safe to assume that
// if this runs, the context carries a valid user
//
// The logger is used to record the reason a token was rejected, and deatils are
// deliberately not returned to the user (get a flat "unauthorized" message) but
// is stored within logs to later use for debug and identifying patterns.
func Authenticate(gen *TokenGenerator, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				unauthorized(w, "missing_token", "authorized header is required")
				return
			}

			// SplitN with n=2 so that an oddly-spaced token still yields two parts
			// so a token containing spaces wouldn't be an issue at split.
			// Usually, a Bearer token never contains spaces.
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				unauthorized(w, "malformed_header", "authorization header must be 'Bearer <token>'")
				return
			}

			tokenStr := strings.TrimSpace(parts[1])
			if tokenStr == "" {
				unauthorized(w, "missing_token", "bearer token is empty")
				return
			}

			claims, err := gen.ValidateAccessToken(tokenStr)
			if err != nil {
				if errors.Is(err, ErrInvalidToken) {
					logger.Warn("rejected access token", "err", err)
				} else {
					logger.Error("unexpected error validating access token", "err", err)
				}
				unauthorized(w, "invalid_token", "token is invalid or expired")
				return
			}
			userID, err := claims.UserID()
			if err != nil {
				// ValidateAccessToken already checked this; hitting it here
				// means the validation path changed and this is a bug in
				// our own code. Log at Error so it stands out.
				logger.Error("claims passed validation but user id parse failed", "err", err)
				unauthorized(w, "invalid_token", "token is invalid or expired")
				return
			}
			sessionID, err := claims.SessionID()
			if err != nil {
				logger.Error("claims passed validation but session id parse failed", "err", err)
				unauthorized(w, "invalid_token", "token is invalid or expired")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			ctx = context.WithValue(ctx, sessionID, sessionID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// unauthorized writes a 401 Unauthorized with the standard JSON error envelope and the
// WWW-Authenticate header required by RFC 6750.
//
// The `code` is a short machine-readable string the client can branch on
// (e.g. "missing_token" → redirect to login, "invalid_token" → refresh and
// retry). The `message` is human-readable and safe to display.
func unauthorized(w http.ResponseWriter, code, message string) {
	// RFC 6750 §3: a challenge that tells the client which scheme to use.
	// We include error="invalid_token" only when the problem is the token
	// itself; a missing header gets a bare challenge.
	if code == "invalid_token" {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	} else {
		w.Header().Set("WWW-Authenticate", `Bearer`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
