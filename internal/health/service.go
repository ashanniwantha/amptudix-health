package health

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/ashanniwantha/amptudix-health/internal/client/openwearables"
	"github.com/ashanniwantha/amptudix-health/internal/user"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrNotLinked    = errors.New("user is not linked to Open Wearables")
)

// UserLookup is the piece of the user service this package needs.
// The consumer defines the contract.
type UserLookup interface {
	GetOpenWearablesID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error)
}

// DataClient is the piece of the Open Wearables client this package needs.
type DataClient interface {
	GetTimeseries(ctx context.Context, owUserID uuid.UUID, p openwearables.TimeseriesParams) (*openwearables.Page[openwearables.TimeseriesSample], error)
	GetWorkouts(ctx context.Context, owUserID uuid.UUID, p openwearables.EventParams) (*openwearables.Page[openwearables.Workout], error)
	GetSleep(ctx context.Context, owUserID uuid.UUID, p openwearables.SleepParams) (*openwearables.Page[openwearables.SleepSession], error)
}

type Service struct {
	users UserLookup
	ow    DataClient
}

func NewService(users UserLookup, ow DataClient) *Service {
	return &Service{users: users, ow: ow}
}

func (s *Service) Timeseries(ctx context.Context, userID uuid.UUID, p openwearables.TimeseriesParams) (*openwearables.Page[openwearables.TimeseriesSample], error) {
	owID, err := s.resolve(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.ow.GetTimeseries(ctx, owID, p)
}

func (s *Service) Workouts(ctx context.Context, userID uuid.UUID, p openwearables.EventParams) (*openwearables.Page[openwearables.Workout], error) {
	owID, err := s.resolve(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.ow.GetWorkouts(ctx, owID, p)
}

func (s *Service) Sleep(ctx context.Context, userID uuid.UUID, p openwearables.SleepParams) (*openwearables.Page[openwearables.SleepSession], error) {
	owID, err := s.resolve(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.ow.GetSleep(ctx, owID, p)
}

func (s *Service) resolve(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	owID, err := s.users.GetOpenWearablesID(ctx, userID)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrUserNotFound):
			return uuid.Nil, ErrUserNotFound
		case errors.Is(err, user.ErrNotLinked):
			return uuid.Nil, ErrNotLinked
		default:
			return uuid.Nil, err
		}
	}
	return owID, nil
}
