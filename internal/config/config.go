package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds service configuration.
type Config struct {
	DatabaseURL         string
	ServerAddr          string
	SessionTTL          time.Duration
	SessionCookieName   string
	SessionCookieSecure bool
}

// Load reads configuration from environment.
func Load() (*Config, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		user := getenv("POSTGRES_USER", "exec_hub")
		pass := getenv("POSTGRES_PASSWORD", "exec_hub_pass")
		db := getenv("POSTGRES_DB", "exec_hub")
		host := getenv("POSTGRES_HOST", "localhost")
		port := getenv("POSTGRES_PORT", "5432")
		sslmode := getenv("DATABASE_SSLMODE", "disable")
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, db, sslmode)
	}
	addr := getenv("SERVER_ADDR", "0.0.0.0:8080")
	ttl := parseDuration(getenv("SESSION_TTL", "24h"), 24*time.Hour)
	cookieName := getenv("SESSION_COOKIE_NAME", "exec_hub_session")
	cookieSecure := parseBool(getenv("SESSION_COOKIE_SECURE", "false"), false)

	return &Config{
		DatabaseURL:         dsn,
		ServerAddr:          addr,
		SessionTTL:          ttl,
		SessionCookieName:   cookieName,
		SessionCookieSecure: cookieSecure,
	}, nil
}

func getenv(key, def string) string {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	return val
}

func parseDuration(val string, def time.Duration) time.Duration {
	if val == "" {
		return def
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return def
	}
	return d
}

func parseBool(val string, def bool) bool {
	if val == "" {
		return def
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return def
	}
	return b
}
