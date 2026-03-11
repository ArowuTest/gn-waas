package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the tariff engine service
type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Server   ServerConfig
}

type AppConfig struct {
	Env     string `mapstructure:"env"`
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Name     string `mapstructure:"name"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	SSLMode  string `mapstructure:"ssl_mode"`
	MaxConns int    `mapstructure:"max_conns"`
	MinConns int    `mapstructure:"min_conns"`
	// Raw DSN — set when DATABASE_URL is provided (Render / Heroku style)
	RawDSN string
}

// DSN returns the pgx-compatible connection string.
// If DATABASE_URL was provided it is used directly (with pool params appended).
// Otherwise the individual host/port/name/user/password fields are used.
func (d DatabaseConfig) DSN() string {
	if d.RawDSN != "" {
		// Append pgx pool params to the URL
		sep := "?"
		if strings.Contains(d.RawDSN, "?") {
			sep = "&"
		}
		return fmt.Sprintf("%spool_max_conns=%d&pool_min_conns=%d",
			d.RawDSN+sep, d.MaxConns, d.MinConns)
	}
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s pool_max_conns=%d pool_min_conns=%d",
		d.Host, d.Port, d.Name, d.User, d.Password, d.SSLMode, d.MaxConns, d.MinConns,
	)
}

type ServerConfig struct {
	Port            int    `mapstructure:"port"`
	ReadTimeoutSec  int    `mapstructure:"read_timeout_sec"`
	WriteTimeoutSec int    `mapstructure:"write_timeout_sec"`
	GracefulStopSec int    `mapstructure:"graceful_stop_sec"`
}

// Load reads configuration from environment variables and config files.
// Supports both DATABASE_URL (Render/Heroku style) and individual DB_* vars.
func Load() (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("app.env", "development")
	v.SetDefault("app.name", "gnwaas-tariff-engine")
	v.SetDefault("app.version", "1.0.0")
	v.SetDefault("server.port", 3003)
	v.SetDefault("server.read_timeout_sec", 30)
	v.SetDefault("server.write_timeout_sec", 30)
	v.SetDefault("server.graceful_stop_sec", 10)
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.max_conns", 10)
	v.SetDefault("database.min_conns", 2)

	// Environment variables
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Map env vars to config keys
	_ = v.BindEnv("app.env", "APP_ENV")
	_ = v.BindEnv("server.port", "APP_PORT", "PORT")
	_ = v.BindEnv("database.host", "DB_HOST")
	_ = v.BindEnv("database.port", "DB_PORT")
	_ = v.BindEnv("database.name", "DB_NAME")
	_ = v.BindEnv("database.user", "DB_USER")
	_ = v.BindEnv("database.password", "DB_PASSWORD")
	_ = v.BindEnv("database.ssl_mode", "DB_SSL_MODE", "DB_SSLMODE")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// ── DATABASE_URL support (Render / Heroku / Railway style) ───────────────
	// If DATABASE_URL is set, parse it and override individual DB fields.
	// This is the standard way Render injects the database connection string.
	if rawURL := os.Getenv("DATABASE_URL"); rawURL != "" {
		cfg.Database.RawDSN = rawURL
		// Also populate individual fields for logging / diagnostics
		if u, err := url.Parse(rawURL); err == nil {
			cfg.Database.Host = u.Hostname()
			if p := u.Port(); p != "" {
				if port, err := strconv.Atoi(p); err == nil {
					cfg.Database.Port = port
				}
			}
			cfg.Database.Name = strings.TrimPrefix(u.Path, "/")
			cfg.Database.User = u.User.Username()
			if pw, ok := u.User.Password(); ok {
				cfg.Database.Password = pw
			}
			// Render uses sslmode=require in the URL query string
			if q := u.Query(); q.Get("sslmode") != "" {
				cfg.Database.SSLMode = q.Get("sslmode")
			} else {
				// Default to require for Render hosted databases
				cfg.Database.SSLMode = "require"
			}
		}
	}

	// ── PORT env var (Render sets PORT, not APP_PORT) ─────────────────────────
	if portStr := os.Getenv("PORT"); portStr != "" && cfg.Server.Port == 3003 {
		if port, err := strconv.Atoi(portStr); err == nil {
			cfg.Server.Port = port
		}
	}

	return &cfg, nil
}
