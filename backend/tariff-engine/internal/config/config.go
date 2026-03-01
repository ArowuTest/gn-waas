package config

import (
	"fmt"
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
}

func (d DatabaseConfig) DSN() string {
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

// Load reads configuration from environment variables and config files
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

	// Environment variables (APP_PORT → server.port, DB_HOST → database.host, etc.)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Map env vars to config keys
	_ = v.BindEnv("app.env", "APP_ENV")
	_ = v.BindEnv("server.port", "APP_PORT")
	_ = v.BindEnv("database.host", "DB_HOST")
	_ = v.BindEnv("database.port", "DB_PORT")
	_ = v.BindEnv("database.name", "DB_NAME")
	_ = v.BindEnv("database.user", "DB_USER")
	_ = v.BindEnv("database.password", "DB_PASSWORD")
	_ = v.BindEnv("database.ssl_mode", "DB_SSL_MODE")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
