package openwearables

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// User is the subset of the Open Wearables user shape the backend needs.
type User struct {
	ID        uuid.UUID `json:"id"`
	Email     *string   `json:"email"`
	FirstName *string   `json:"first_name"`
	LastName  *string   `json:"last_name"`
}

// TokenResponse is returned by POST /api/v1/users/{user_id}/token.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// Page is the common envelope for data endpoints (timeseries, events).
type Page[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
	Metadata   Metadata   `json:"metadata"`
}

type Pagination struct {
	NextCursor     *string `json:"next_cursor"`
	PreviousCursor *string `json:"previous_cursor"`
	HasMore        bool    `json:"has_more"`
	TotalCount     *int    `json:"total_count"`
}

type Metadata struct {
	Resolution  *string    `json:"resolution"`
	SampleCount *int       `json:"sample_count"`
	StartTime   *time.Time `json:"start_time"`
	EndTime     *time.Time `json:"end_time"`
}

// TimeseriesSample matches the documented data item shape.
type TimeseriesSample struct {
	Timestamp     time.Time  `json:"timestamp"`
	Type          string     `json:"type"`
	Value         float64    `json:"value"`
	Unit          string     `json:"unit"`
	ZoneOffset    string     `json:"zone_offset"`
	IsDailyTotal  bool       `json:"is_daily_total"`
	WindowSeconds *int       `json:"window_seconds"`
	PeriodEnd     *time.Time `json:"period_end"`
	Source        *Source    `json:"source"`
	Stats         *Stats     `json:"stats"`
}

type Source struct {
	Provider   string `json:"provider"`
	DeviceName string `json:"device_name"`
	Source     string `json:"source"`
	Device     string `json:"device"`
	DeviceType string `json:"device_type"`
	DeviceID   string `json:"device_id"`
}

type Stats struct {
	Count int     `json:"count"`
	Avg   float64 `json:"avg"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Sum   float64 `json:"sum"`
	Last  float64 `json:"last"`
}

// Workout and SleepSession cover the common fields. Extend them as you
// inspect real payloads from your instance's /docs Swagger UI.
type Workout struct {
	ID               uuid.UUID `json:"id"`
	Type             string    `json:"type"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	DurationSeconds  int       `json:"duration_seconds"`
	Calories         *float64  `json:"calories"`
	DistanceMeters   *float64  `json:"distance_meters"`
	AverageHeartRate *float64  `json:"average_heart_rate"`
	MaxHeartRate     *float64  `json:"max_heart_rate"`
	Source           *Source   `json:"source"`
}

type SleepSession struct {
	ID              uuid.UUID       `json:"id"`
	StartTime       time.Time       `json:"start_time"`
	EndTime         time.Time       `json:"end_time"`
	DurationSeconds int             `json:"duration_seconds"`
	Stages          json.RawMessage `json:"stages"`
	Source          *Source         `json:"source"`
}

// TimeseriesParams are the query parameters for GET /users/{id}/timeseries.
type TimeseriesParams struct {
	StartTime  *time.Time
	EndTime    *time.Time
	Types      []string
	Resolution string // raw | hourly | daily
	Limit      int    // max 1000
	Cursor     string
}

// EventParams are the query parameters for workouts and sleep.
type EventParams struct {
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
	Cursor    string
}

// SleepParams adds the sleep-specific priority filter.
type SleepParams struct {
	EventParams
	FilterByPriority bool
}

// APIError is returned for any non-2xx response.
type APIError struct {
	StatusCode int
	Detail     string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("open wearables: HTTP %d: %s", e.StatusCode, e.Detail)
}
