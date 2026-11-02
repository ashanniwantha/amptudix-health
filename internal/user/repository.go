package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ashanniwantha/amptudix-health/internal/database"
)

// Repository is the persistence contract the service depends on.
type Repository interface {
	Create(ctx context.Context, db database.DBTX, u *User) (*User, error)
	GetByEmail(ctx context.Context, db database.DBTX, email string) (*User, error)
	GetByID(ctx context.Context, db database.DBTX, id uuid.UUID) (*User, error)
	SetOWID(ctx context.Context, db database.DBTX, id uuid.UUID, owID uuid.UUID) error
}

type postgresRepo struct{}

func NewRepository() Repository { return &postgresRepo{} }

func (r *postgresRepo) Create(ctx context.Context, db database.DBTX, u *User) (*User, error) {
	const q = `
		INSERT INTO users (ow_id, username, email, hashed_password, first_name, last_name, dob)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, ow_id, username, email, hashed_password, first_name, last_name, dob, created_at, updated_at`

	var ow pgtype.UUID
	if u.OW_ID != nil {
		ow = pgtype.UUID{Bytes: *u.OW_ID, Valid: true}
	}

	var out User
	var outOW pgtype.UUID
	var dob pgtype.Date

	err := db.QueryRow(ctx, q,
		ow, u.Username, u.Email, u.HashedPassword,
		u.FirstName, u.LastName, u.DOB,
	).Scan(
		&out.ID, &outOW, &out.Username, &out.Email, &out.HashedPassword,
		&out.FirstName, &out.LastName, &dob, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			switch pgErr.ConstraintName {
			case "uq_users_email_lower":
				return nil, ErrEmailTaken
			case "uq_users_username_lower":
				return nil, ErrUsernameTaken
			}
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}
	if outOW.Valid {
		id := uuid.UUID(outOW.Bytes)
		out.OW_ID = &id
	}
	if dob.Valid {
		out.DOB = &dob.Time
	}
	return &out, nil
}

func (r *postgresRepo) GetByEmail(ctx context.Context, db database.DBTX, email string) (*User, error) {
	const q = `
		SELECT id, ow_id, username, email, hashed_password, first_name, last_name, dob, created_at, updated_at
		FROM users WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL`
	return r.scanOne(ctx, db.QueryRow(ctx, q, email))
}

func (r *postgresRepo) GetByID(ctx context.Context, db database.DBTX, id uuid.UUID) (*User, error) {
	const q = `
		SELECT id, ow_id, username, email, hashed_password, first_name, last_name, dob, created_at, updated_at
		FROM users WHERE id = $1 AND deleted_at IS NULL`
	return r.scanOne(ctx, db.QueryRow(ctx, q, id))
}

func (r *postgresRepo) SetOWID(ctx context.Context, db database.DBTX, id uuid.UUID, owID uuid.UUID) error {
	const q = `UPDATE users SET ow_id = $1, updated_at = NOW() WHERE id = $2`
	_, err := db.Exec(ctx, q, owID, id)
	return err
}

func (r *postgresRepo) scanOne(ctx context.Context, row pgx.Row) (*User, error) {
	var u User
	var ow pgtype.UUID
	var dob pgtype.Date
	err := row.Scan(&u.ID, &ow, &u.Username, &u.Email, &u.HashedPassword,
		&u.FirstName, &u.LastName, &dob, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	if ow.Valid {
		id := uuid.UUID(ow.Bytes)
		u.OW_ID = &id
	}
	if dob.Valid {
		u.DOB = &dob.Time
	}
	return &u, nil
}
