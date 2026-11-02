package user

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// ENTITIES
// ============================================================

// User is the domain representation of a row in the users table.
// DOB is a pointer because it is nullable.
//
// OW_ID is a pointer because the user row is inserted BEFORE we call
// Open Wearables to create the remote user. Between the insert and the
// successful remote call, the local row has no OW_ID. A non-pointer
// uuid.UUID would encode "not yet synced" as the zero UUID, which is
// indistinguishable from a real ID and silently breaks every query
// that filters on OW_ID.
//
// HashedPassword lives here because the login flow needs to read it
// from the same flow as the identity. It is never serialized to a
// client. The handler's response DTO simply has no field for it.
type User struct {
	ID             uuid.UUID
	OW_ID          *uuid.UUID
	Username       string
	Email          string
	HashedPassword string
	FirstName      string
	LastName       string
	DOB            *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// DisplayName returns the user's full name, trimmed so that a missing
// last name does not leave a trailing space. Safe to show in UI.
func (u *User) DisplayName() string {
	return strings.TrimSpace(u.FirstName + " " + u.LastName)
}

// HasOpenWearablesLink reports whether this user has already been
// created in Open Wearables. The service uses this to skip the remote
// call on idempotent re-registrations, and to guard endpoints that
// require a valid OW_ID (token minting, data fetch).
func (u *User) HasOpenWearablesLink() bool {
	return u.OW_ID != nil
}

// ============================================================
// VALIDATION RULES
// ============================================================
//
// These constants express domain invariants. They are the source of
// truth: handlers do not duplicate them, and migrations that add a
// CHECK constraint mirror these values. If you ever bump one, bump
// the SQL constraint in the same commit.

const (
	MinUsernameLength = 3
	MaxUsernameLength = 32

	MinPasswordLength = 8
	MaxPasswordLength = 128

	MinAgeYears = 13  // Below this we cannot legally hold health data in most jurisdictions.
	MaxAgeYears = 120 // Catches typos like 2026-01-01 entered as a birth year.
)

// ============================================================
// DOMAIN ERRORS
// ============================================================
//
// Every error is created with errors.New so it carries no dynamic
// content. Callers use errors.Is to test. If you ever want to attach
// context (which user, which email), wrap the sentinel at the call
// site: fmt.Errorf("%w: %s", ErrUserNotFound, id). Never bake the
// detail into the sentinel itself.

var (
	// --- Lookup / persistence ---

	// ErrUserNotFound is returned when a lookup by ID, email, or
	// username finds no matching row. Handlers translate this to 404
	// for self-service endpoints and to 401 for login (we don't want
	// to reveal which emails are registered).
	ErrUserNotFound = errors.New("user not found")

	// ErrUsernameTaken is returned when an insert or update would
	// violate uq_users_username. The repository detects this by
	// matching the PG error code 23505 and the constraint name, not
	// by doing a "SELECT first, then INSERT" — that pattern races.
	ErrUsernameTaken = errors.New("username is already taken")

	// ErrEmailTaken is the same idea for uq_users_email.
	ErrEmailTaken = errors.New("email is already registered")

	// --- Input validation ---

	// ErrInvalidEmail is returned by ValidateEmail. Handlers map it
	// to 400 with the message surfaced to the client.
	ErrInvalidEmail = errors.New("email address is not valid")

	// ErrInvalidUsername is returned by ValidateUsername. The
	// message deliberately repeats the rule so the client can fix it
	// without reading the docs.
	ErrInvalidUsername = errors.New("username must be 3-32 characters and contain only letters, digits, or underscores")

	// ErrInvalidPassword is returned by ValidatePassword. It is
	// evaluated against the PLAIN password, before hashing. The
	// service never stores plain text.
	ErrInvalidPassword = errors.New("password must be between 8 and 128 characters")

	// ErrInvalidName is returned when first or last name is empty
	// after trimming. We require both because DisplayName and most
	// provider UIs assume they exist.
	ErrInvalidName = errors.New("first name and last name are required")

	// ErrInvalidDOB is returned when DOB is in the future or implies
	// an age outside [MinAgeYears, MaxAgeYears].
	ErrInvalidDOB = errors.New("date of birth is not valid")

	// --- Open Wearables ---

	// ErrOpenWearablesSync is returned by the service when the remote
	// call to Open Wearables fails. The local transaction is rolled
	// back, so the user is not created on either side.
	ErrOpenWearablesSync = errors.New("failed to create user in Open Wearables")

	// ErrAlreadyLinked is returned when the service is asked to link
	// an Open Wearables user to a local user that already has an
	// OW_ID. Callers should treat this as idempotent success, not an
	// error, unless they need to force a re-link.
	ErrAlreadyLinked = errors.New("user is already linked to Open Wearables")
)

// ============================================================
// VALIDATION FUNCTIONS
// ============================================================
//
// Pure functions — no DB, no HTTP, no Open Wearables. The service
// calls these BEFORE opening a transaction so invalid input never
// touches the database. Unit tests for these need zero mocks.

// ValidateEmail checks that s parses as an RFC 5322 address. It does
// not check deliverability — that is the email provider's job.
func ValidateEmail(s string) error {
	if strings.TrimSpace(s) == "" {
		return ErrInvalidEmail
	}
	if _, err := mail.ParseAddress(s); err != nil {
		return ErrInvalidEmail
	}
	return nil
}

// ValidateUsername enforces the shape rules only. Uniqueness is the
// repository's job (ErrUsernameTaken) because only the DB can answer
// that atomically.
func ValidateUsername(s string) error {
	n := len(s)
	if n < MinUsernameLength || n > MaxUsernameLength {
		return ErrInvalidUsername
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_':
		default:
			return ErrInvalidUsername
		}
	}
	return nil
}

// ValidatePassword enforces length only. Complexity rules (require a
// digit, require a symbol, ...) are intentionally NOT enforced —
// NIST SP 800-63B recommends length over composition. Bcrypt will do
// the rest.
func ValidatePassword(plain string) error {
	n := len(plain)
	if n < MinPasswordLength || n > MaxPasswordLength {
		return ErrInvalidPassword
	}
	return nil
}

// ValidateName requires both names to be non-empty after trimming.
func ValidateName(first, last string) error {
	if strings.TrimSpace(first) == "" || strings.TrimSpace(last) == "" {
		return ErrInvalidName
	}
	return nil
}

// ValidateDOB enforces the age window. A nil DOB is legal (the user
// can fill it in later), so this function is only called when a DOB
// is supplied.
func ValidateDOB(dob time.Time, now time.Time) error {
	if dob.After(now) {
		return ErrInvalidDOB
	}
	years := now.Year() - dob.Year()
	// Adjust if the birthday has not yet happened this year.
	if now.Month() < dob.Month() || (now.Month() == dob.Month() && now.Day() < dob.Day()) {
		years--
	}
	if years < MinAgeYears || years > MaxAgeYears {
		return ErrInvalidDOB
	}
	return nil
}

// NormalizeEmail lowercases and trims. Call it in the service before
// both INSERT and lookup so that Alice@Example.com and
// alice@example.com are one identity. The DB should also have a
// unique index on LOWER(email) as a belt-and-braces guard.
func NormalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// NormalizeUsername trims but does not case-fold: usernames are
// commonly allowed to be case-preserving with case-insensitive
// uniqueness enforced by the DB (unique index on LOWER(username)).
func NormalizeUsername(s string) string {
	return strings.TrimSpace(s)
}
