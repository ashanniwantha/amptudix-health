package user

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/ashanniwantha/amptudix-health/internal/client/openwearables"
	"github.com/ashanniwantha/amptudix-health/internal/database"
)

// TxRunner is satisfied by *database.TxManager.
type TxRunner interface {
	WithTransaction(ctx context.Context, fn func(context.Context) error) error
}

// OWClient is the Open Wearables surface the user service needs.
type OWClient interface {
	CreateUser(ctx context.Context, email, firstName, lastName string) (uuid.UUID, error)
	CreateUserToken(ctx context.Context, owUserID uuid.UUID) (*openwearables.TokenResponse, error)
}

type SDKToken struct {
	OpenWearablesUserID uuid.UUID
	Token               *openwearables.TokenResponse
}

type Service struct {
	repo Repository
	tx   TxRunner
	db   database.DBTX
	ow   OWClient
}

func NewService(repo Repository, tx TxRunner, db database.DBTX, ow OWClient) *Service {
	return &Service{repo: repo, tx: tx, db: db, ow: ow}
}

// GetOpenWearablesID resolves a local user ID to the stored OW UUID.
func (s *Service) GetOpenWearablesID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	u, err := s.repo.GetByID(ctx, s.db, userID)
	if err != nil {
		return uuid.Nil, err
	}
	if u.OW_ID == nil {
		return uuid.Nil, ErrNotLinked
	}
	return *u.OW_ID, nil
}

// CreateSDKToken mints a short-lived, user-scoped token for the Android SDK.
func (s *Service) CreateSDKToken(ctx context.Context, userID uuid.UUID) (*SDKToken, error) {
	owID, err := s.GetOpenWearablesID(ctx, userID)
	if err != nil {
		return nil, err
	}
	tok, err := s.ow.CreateUserToken(ctx, owID)
	if err != nil {
		return nil, err
	}
	return &SDKToken{OpenWearablesUserID: owID, Token: tok}, nil
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
