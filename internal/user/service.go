package user

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/ashanniwantha/amptudix-health/internal/database"
)

// TxRunner is satisfied by *database.TxManager.
type TxRunner interface {
	WithTransaction(ctx context.Context, fn func(context.Context) error) error
}

// OWUserCreator is the piece of Open Wearables the service needs.
// The consumer defines the contract; the client package satisfies it.
type OWUserCreator interface {
	CreateUser(ctx context.Context, email, firstName, lastName string) (uuid.UUID, error)
}

type Service struct {
	repo Repository
	tx   TxRunner
	ow   OWUserCreator
}

func NewService(repo Repository, tx TxRunner, ow OWUserCreator) *Service {
	return &Service{repo: repo, tx: tx, ow: ow}
}

type CreateInput struct {
	Username  string
	Email     string
	Password  string
	FirstName string
	LastName  string
	DOB       *time.Time
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*User, error) {
	// 1. Pure validation — no DB yet.
	if err := ValidateUsername(in.Username); err != nil {
		return nil, err
	}
	if err := ValidateEmail(in.Email); err != nil {
		return nil, err
	}
	if err := ValidatePassword(in.Password); err != nil {
		return nil, err
	}
	if err := ValidateName(in.FirstName, in.LastName); err != nil {
		return nil, err
	}
	if in.DOB != nil {
		if err := ValidateDOB(*in.DOB, time.Now()); err != nil {
			return nil, err
		}
	}

	// 2. Normalize.
	email := NormalizeEmail(in.Email)
	username := NormalizeUsername(in.Username)

	// 3. Hash.
	hashed, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	var created *User

	err = s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		db := database.TxFromContext(ctx)

		local := &User{
			Username:       username,
			Email:          email,
			HashedPassword: string(hashed),
			FirstName:      in.FirstName,
			LastName:       in.LastName,
			DOB:            in.DOB,
		}
		saved, err := s.repo.Create(ctx, db, local)
		if err != nil {
			return err
		}

		owID, err := s.ow.CreateUser(ctx, email, in.FirstName, in.LastName)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrOpenWearablesSync, err)
		}

		if err := s.repo.SetOWID(ctx, db, saved.ID, owID); err != nil {
			return err
		}
		saved.OW_ID = &owID
		created = saved
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	// For a read, we do not need a transaction. We borrow a connection
	// from the pool directly.
	return nil, fmt.Errorf("not wired yet")
}
