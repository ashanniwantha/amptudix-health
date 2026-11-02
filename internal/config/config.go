package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config is the typed, validated representation of every environment variable
// the application needs. Construct it once in main() and pass the relevant
// sub-struct to the packages that need it. No package below main() should
// ever call os.Getenv directly.
type Config struct {
	App           App
	DB            DB
	Redis         Redis
	Auth          Auth
	OpenWearables OpenWearables
}

type App struct {
	Env  string
	Port int
}

// Addr returns a value suitable for http.Server.Addr (":8080").
func (a App) Addr() string {
	return fmt.Sprintf(":%d", a.Port)
}

type DB struct {
	Host              string
	Port              int
	User              string
	Password          string
	Name              string
	MaxConns          int32
	MinConns          int32
	ConnectTimeout    time.Duration
	PingTimeout       time.Duration
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// DSN builds the connection string pgxpool.ParseConfig expects.
// sslmode=disable is correct for the local compose stack (traffic never
// leaves the Docker bridge). Change to "require" or "verify-full" when
// connecting to a managed Postgres over a network.
func (d DB) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		url.QueryEscape(d.User),
		url.QueryEscape(d.Password),
		d.Host,
		d.Port,
		d.Name,
	)
}

type Redis struct {
	Host        string
	Port        int
	Password    string
	DB          int
	PingTimeout time.Duration
}

// Addr returns "host:port" for go-redis.
func (r Redis) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// Auth carries the RSA key paths and access-token lifetime.
// The access token is signed with RS256 (asymmetric); no string secret is
// ever loaded here. The refresh token uses crypto/rand and a SHA-256 hash
// (see internal/auth/refresh.go) and needs no config either.
type Auth struct {
	PrivateKeyPath string
	PublicKeyPath  string
	AccessTTL      time.Duration
}

// OpenWearables groups everything needed to talk to the Open Wearables
// backend. AppID/AppSecret are only required once the SDK token-minting
// endpoint is implemented; the user-creation flow needs only APIKey.
type OpenWearables struct {
	APIKey    string
	APIURL    string
	AppID     string
	AppSecret string
}

// LoadConfig reads the environment (loading .env first if present), parses
// every value into the typed struct, and validates required variables.
// It returns a single error listing every missing variable at once, so
// developers do not fix them one by one.
func LoadConfig() (*Config, error) {
	// godotenv.Load() is a no-op when there is no .env, so it is safe to
	// call unconditionally. The error is logged but not fatal: running in
	// Docker supplies variables via the compose environment block, not a file.
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from system environment variables")
	}

	cfg := &Config{
		App: App{
			Env:  getEnv("APP_ENV", "development"),
			Port: getEnvInt("APP_PORT", 8080),
		},
		DB: DB{
			Host:              getEnv("DB_HOST", "localhost"),
			Port:              getEnvInt("DB_PORT", 5432),
			User:              getEnv("DB_USER", ""),
			Password:          getEnv("DB_PASSWORD", ""),
			Name:              getEnv("DB_NAME", ""),
			MaxConns:          int32(getEnvInt("DB_MAX_CONNS", 20)),
			MinConns:          int32(getEnvInt("DB_MIN_CONNS", 5)),
			ConnectTimeout:    getEnvDuration("DB_CONNECT_TIMEOUT", 5*time.Second),
			PingTimeout:       getEnvDuration("DB_PING_TIMEOUT", 3*time.Second),
			MaxConnLifetime:   getEnvDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute),
			MaxConnIdleTime:   getEnvDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute),
			HealthCheckPeriod: getEnvDuration("DB_HEALTH_CHECK_PERIOD", 1*time.Minute),
		},
		Redis: Redis{
			Host:        getEnv("REDIS_HOST", "localhost"),
			Port:        getEnvInt("REDIS_PORT", 6379),
			Password:    getEnv("REDIS_PASSWORD", ""),
			DB:          getEnvInt("REDIS_DB", 0),
			PingTimeout: getEnvDuration("REDIS_PING_TIMEOUT", 3*time.Second),
		},
		Auth: Auth{
			PrivateKeyPath: getEnv("JWT_PRIVATE_KEY_PATH", ""),
			PublicKeyPath:  getEnv("JWT_PUBLIC_KEY_PATH", ""),
			AccessTTL:      getEnvDuration("JWT_EXPIRATION", 15*time.Minute),
		},
		OpenWearables: OpenWearables{
			APIKey:    getEnv("OPEN_WEARABLES_API_KEY", ""),
			APIURL:    getEnv("OPEN_WEARABLES_API_URL", "http://localhost:8000"),
			AppID:     getEnv("APP_ID", ""),
			AppSecret: getEnv("APP_SECRET", ""),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// validate lists every required-but-empty variable in one error.
// Update this list whenever a new mandatory variable is added; the
// Compose file uses ${VAR:?…} for the same set, so the two should stay
// in sync.
func (c *Config) validate() error {
	var missing []string

	if c.DB.User == "" {
		missing = append(missing, "DB_USER (POSTGRES_USER)")
	}
	if c.DB.Password == "" {
		missing = append(missing, "DB_PASSWORD (POSTGRES_PASSWORD)")
	}
	if c.DB.Name == "" {
		missing = append(missing, "DB_NAME (POSTGRES_DB)")
	}
	if c.Auth.PrivateKeyPath == "" {
		missing = append(missing, "JWT_PRIVATE_KEY_PATH")
	}
	if c.Auth.PublicKeyPath == "" {
		missing = append(missing, "JWT_PUBLIC_KEY_PATH")
	}
	if c.OpenWearables.APIKey == "" {
		missing = append(missing, "OPEN_WEARABLES_API_KEY")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missing)
	}
	return nil
}

// --- helpers ---

// getEnv returns the value of key or fallback when unset or empty.
// Empty-string values in .env are treated as "not set" so the fallback
// applies; that matches Docker Compose's ${VAR:-default} semantics.
func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("config: %s=%q is not an integer, using %d", key, raw, fallback)
		return fallback
	}
	return v
}

// getEnvDuration parses durations like "5s", "1m", "30m".
// time.ParseDuration understands those forms natively.
func getEnvDuration(key string, fallback time.Duration) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		log.Printf("config: %s=%q is not a duration, using %s", key, raw, fallback)
		return fallback
	}
	return v
}
