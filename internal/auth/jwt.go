package auth

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Issuer is stamped into every token we sign and verified on every token we
// accept. Every service have it's own issuer so each service can be identified.
const Issuer = "health-service"

// ErrInvalidToken is the sentinal returned by ValidateAccessToken for every
// failure mode: bad signature, wrong algorithm, wrong issuer, expired,
// malformed claims etc. Callers should use errors.Is to distinguish
// different errors from each other.
var ErrInvalidToken = errors.New("invalid access token")

// Claims is the strongly-typed JWT body we sign and verify.
type Claims struct {
	jwt.RegisteredClaims
}

// UserID parses the Subject back into a UUID.
func (c *Claims) UserID() (uuid.UUID, error) {
	return uuid.Parse(c.Subject)
}

// SessionID parses jti claims back into a UUID.
// This is the ID of the user_sessions row that this token belongs to.
func (c *Claims) SessionID() (uuid.UUID, error) {
	return uuid.Parse(c.ID)
}

// TokenGenerator signs and verifies access token with an RSA key pair.
// Construct one at startup and share it accross the app.
// Safe for concurrent use since RSA signing with a *rsa.PrivateKey
// Which is read-only.
type TokenGenerator struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	expiration time.Duration
}

// NewTokenGenerator constructs a generator from already parsed
// RSA key pair and an access-token lifetime.
func NewTokenGenerator(
	privateKey *rsa.PrivateKey,
	publicKey *rsa.PublicKey,
	expiration time.Duration,
) *TokenGenerator {
	return &TokenGenerator{
		privateKey: privateKey,
		publicKey:  publicKey,
		expiration: expiration,
	}
}

// GenerateAccessToken signs a fresh access token for the given user and
// session IDs. The session ID comes from caller (the user service).
func (g *TokenGenerator) GenerateAccessToken(userID, sessionID uuid.UUID) (string, error) {
	now := time.Now()

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Subject:   userID.String(),
			ID:        sessionID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(g.expiration)),
		},
	}

	// RSA256 = RSA + SHA-256.
	// Asymmetric signing means only the auth service needs the private key.
	// All other services can verify the token just using the public key.
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	signed, err := token.SignedString(g.privateKey)
	if err != nil {
		return "", fmt.Errorf("signing access token: %w", err)
	}

	return signed, nil
}

// ValidateAccessToken parses, verifies, and returns the claims of an access token.
// Any failure is wrapped in ErrInvalidToken so the caller has one thing to check:
// The undelying cause is preserved in the message for logging.
func (g *TokenGenerator) ValidateAccessToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}

	_, err := jwt.ParseWithClaims(
		tokenStr,
		claims,
		func(t *jwt.Token) (any, error) {
			// By the time this runs, WithValidateMethod has already rejected
			// anything that isn't RSA256. Returning the public here is sufficient;
			// no need to re-check t.Method.
			return g.publicKey, nil
		},
		// Pin the accepted algorithm list. Without this, jwt v5 package will
		// whatever the "alg" header claims;
		// Which is the classic algorithm-confusion vulnerability.
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),

		// Reject tokens from a different issuer
		jwt.WithIssuer(Issuer),

		// Reject tokens with no expiration.
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	// Make sure subject and jti is usable as UUID
	// before we hand out the claims.
	if _, err := claims.UserID(); err != nil {
		return nil, fmt.Errorf("%w: malformed subject (user ID)", ErrInvalidToken)
	}
	if _, err := claims.SessionID(); err != nil {
		return nil, fmt.Errorf("%w: malformed jti (session ID)", ErrInvalidToken)
	}

	return claims, nil
}
