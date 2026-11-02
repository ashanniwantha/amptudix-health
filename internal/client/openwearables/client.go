package openwearables

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Client struct {
	baseURL    string
	apiKey     string
	appID      string
	appSecret  string
	httpClient *http.Client
}

// NewClient constructs the Open Wearables API client. baseURL is the host
// only, e.g. http://host.docker.internal:8000.
func NewClient(baseURL, apiKey, appID, appSecret string) *Client {
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiKey:    apiKey,
		appID:     appID,
		appSecret: appSecret,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateUser creates an Open Wearables user and returns its UUID.
// This is Step 1 of the integration flow.
func (c *Client) CreateUser(ctx context.Context, email, firstName, lastName string) (uuid.UUID, error) {
	body := map[string]string{
		"email":      email,
		"first_name": firstName,
		"last_name":  lastName,
	}
	var out User
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/users", nil, body, &out, true); err != nil {
		return uuid.Nil, err
	}
	return out.ID, nil
}

// CreateUserToken exchanges app credentials for a user-scoped SDK token.
// The app_id/app_secret are sent in the body; no API key header is used.
func (c *Client) CreateUserToken(ctx context.Context, owUserID uuid.UUID) (*TokenResponse, error) {
	body := map[string]string{
		"app_id":     c.appID,
		"app_secret": c.appSecret,
	}
	var out TokenResponse
	path := "/api/v1/users/" + owUserID.String() + "/token"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, body, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTimeseries returns heart rate, steps, etc. for one user.
func (c *Client) GetTimeseries(ctx context.Context, owUserID uuid.UUID, p TimeseriesParams) (*Page[TimeseriesSample], error) {
	q := url.Values{}
	if p.StartTime != nil {
		q.Set("start_time", p.StartTime.UTC().Format(time.RFC3339))
	}
	if p.EndTime != nil {
		q.Set("end_time", p.EndTime.UTC().Format(time.RFC3339))
	}
	for _, t := range p.Types {
		q.Add("types", t)
	}
	if p.Resolution != "" {
		q.Set("resolution", p.Resolution)
	}
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}

	var out Page[TimeseriesSample]
	path := "/api/v1/users/" + owUserID.String() + "/timeseries"
	if err := c.doJSON(ctx, http.MethodGet, path, q, nil, &out, true); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetWorkouts returns workout sessions for one user.
func (c *Client) GetWorkouts(ctx context.Context, owUserID uuid.UUID, p EventParams) (*Page[Workout], error) {
	q := eventQuery(p)
	var out Page[Workout]
	path := "/api/v1/users/" + owUserID.String() + "/events/workouts"
	if err := c.doJSON(ctx, http.MethodGet, path, q, nil, &out, true); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetSleep returns sleep sessions (including naps) for one user.
func (c *Client) GetSleep(ctx context.Context, owUserID uuid.UUID, p SleepParams) (*Page[SleepSession], error) {
	q := eventQuery(p.EventParams)
	if p.FilterByPriority {
		q.Set("filter_by_priority", "true")
	}
	var out Page[SleepSession]
	path := "/api/v1/users/" + owUserID.String() + "/events/sleep"
	if err := c.doJSON(ctx, http.MethodGet, path, q, nil, &out, true); err != nil {
		return nil, err
	}
	return &out, nil
}

func eventQuery(p EventParams) url.Values {
	q := url.Values{}
	if p.StartDate != nil {
		q.Set("start_date", p.StartDate.UTC().Format(time.RFC3339))
	}
	if p.EndDate != nil {
		q.Set("end_date", p.EndDate.UTC().Format(time.RFC3339))
	}
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	return q
}

func (c *Client) doJSON(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	out any,
	withAPIKey bool,
) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if withAPIKey {
		req.Header.Set("X-Open-Wearables-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("open wearables request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct {
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(respBody, &e)
		detail := e.Detail
		if detail == "" {
			detail = strings.TrimSpace(string(respBody))
		}
		return &APIError{StatusCode: resp.StatusCode, Detail: detail}
	}

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode open wearables response: %w", err)
	}
	return nil
}
