package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// refreshTokenBytes is the raw materials per token, before encoding.
// 32 bytes is 256 bits of entrophy.
const refreshTokenBytes = 32

// RefreshToken pairs the plaintext we hand to the client with the hash
// we store in the database. They are produced together but not equal.
// Rule: PlainText leaves this process only in the response body of either
// /auth/login or /auth/refresh. It is never logged, stored or re-sent.
// After issuance the server only knows the Hash.
type RefreshToken struct {
	PlainText string // goes to the client
	Hash      string // goes into DB at user_session.refresh_token_hash
}

// GenerateRefreshToken mints a new refresh token.
// The plaintext is random bytes from crypto/rand,
// base64url-encoded with no padding; which makes it safe in
// URLs, headers, and cookies.
// The hash is the hex-encoded SHA-256 of the plaintext.
func GenerateRefreshToken() (RefreshToken, error) {
	raw := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		// crypto/rand.Read only breaks if the OS entrophy source is broken.
		// Which is not a retry-error.
		return RefreshToken{}, fmt.Errorf("reading random bytes for refresh token: %w", err)
	}

	plainText := base64.RawURLEncoding.EncodeToString(raw)

	return RefreshToken{
		PlainText: plainText,
		Hash:      HashRefreshToken(plainText),
	}, nil
}

// HashRefreshToken computes the storage hash of a refresh token plaintext.
//
// Called in two places:
//
//  1. During issuance (inside GenerateRefreshToken).
//  2. During validation, by the user service, which receives a plaintext
//     from the client and must hash it before looking up the session row.
//
// It is deterministic and unsalted on purpose. Both call sites depend on
// the same input always producing the same output, so a stored hash can
// be found by an indexed equality match.
func HashRefreshToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
