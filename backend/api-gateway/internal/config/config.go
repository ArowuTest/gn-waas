package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Keycloak KeycloakConfig
	Services ServicesConfig
	MinIO    MinIOConfig
}

type AppConfig struct {
	Env     string `mapstructure:"env"`
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
}

type ServerConfig struct {
	Port            int `mapstructure:"port"`
	ReadTimeoutSec  int `mapstructure:"read_timeout_sec"`
	WriteTimeoutSec int `mapstructure:"write_timeout_sec"`
	GracefulStopSec int `mapstructure:"graceful_stop_sec"`
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
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s pool_max_conns=%d",
		d.Host, d.Port, d.Name, d.User, d.Password, d.SSLMode, d.MaxConns,
	)
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type KeycloakConfig struct {
	URL      string `mapstructure:"url"`
	Realm    string `mapstructure:"realm"`
	ClientID string `mapstructure:"client_id"`
}

type ServicesConfig struct {
	SentinelURL   string `mapstructure:"sentinel_url"`
	TariffURL     string `mapstructure:"tariff_url"`
	GRABridgeURL  string `mapstructure:"gra_bridge_url"`
	OCRServiceURL string `mapstructure:"ocr_service_url"`
	CDCIngestorURL string `mapstructure:"cdc_ingestor_url"`
}

type MinIOConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	UseSSL    bool   `mapstructure:"use_ssl"`
}

func Load() (*Config, error) {
	v := viper.New()

	v.SetDefault("app.env", "development")
	v.SetDefault("app.name", "gnwaas-api-gateway")
	v.SetDefault("app.version", "1.0.0")
	v.SetDefault("server.port", 3000)
	v.SetDefault("server.read_timeout_sec", 30)
	v.SetDefault("server.write_timeout_sec", 30)
	v.SetDefault("server.graceful_stop_sec", 10)
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.max_conns", 20)
	v.SetDefault("database.min_conns", 2)
	v.SetDefault("redis.db", 0)
	v.SetDefault("minio.bucket", "gnwaas-evidence")
	v.SetDefault("minio.use_ssl", false)

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	envBindings := map[string]string{
		"app.env":                "APP_ENV",
		"server.port":            "APP_PORT",
		"database.host":          "DB_HOST",
		"database.port":          "DB_PORT",
		"database.name":          "DB_NAME",
		"database.user":          "DB_USER",
		"database.password":      "DB_PASSWORD",
		"redis.host":             "REDIS_HOST",
		"redis.port":             "REDIS_PORT",
		"redis.password":         "REDIS_PASSWORD",
		"keycloak.url":           "KEYCLOAK_URL",
		"keycloak.realm":         "KEYCLOAK_REALM",
		"keycloak.client_id":     "KEYCLOAK_CLIENT_ID",
		"services.sentinel_url":  "SENTINEL_SERVICE_URL",
		"services.tariff_url":    "TARIFF_SERVICE_URL",
		"services.gra_bridge_url": "GRA_SERVICE_URL",
		"services.ocr_service_url": "OCR_SERVICE_URL",
		"minio.endpoint":         "MINIO_ENDPOINT",
		"minio.access_key":       "MINIO_ACCESS_KEY",
		"minio.secret_key":       "MINIO_SECRET_KEY",
	}

	for key, env := range envBindings {
		_ = v.BindEnv(key, env)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
