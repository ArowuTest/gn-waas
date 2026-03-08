package config

import (
	"fmt"
	"os"
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
	Port            int  `mapstructure:"port"`
	ReadTimeoutSec  int  `mapstructure:"read_timeout_sec"`
	WriteTimeoutSec int  `mapstructure:"write_timeout_sec"`
	GracefulStopSec int  `mapstructure:"graceful_stop_sec"`
	DevMode         bool `mapstructure:"dev_mode"`
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
	v.SetDefault("server.dev_mode", false)
	v.SetDefault("database.ssl_mode", "require")
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
		"server.dev_mode":         "DEV_MODE",
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

	// DATABASE_URL support: Render (and other PaaS) provide a single
	// DATABASE_URL env var. Parse it and set individual DB_* vars so the
	// rest of the config system works unchanged.
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		// Format: postgres://user:password@host:port/dbname?sslmode=require
		if parsed, err := parseDBURL(dbURL); err == nil {
			if parsed["host"] != "" {
				_ = v.BindEnv("database.host", "DATABASE_URL")
				v.Set("database.host", parsed["host"])
			}
			if parsed["port"] != "" {
				_ = v.BindEnv("database.port", "DATABASE_URL")
				v.Set("database.port", parsed["port"])
			}
			if parsed["name"] != "" {
				v.Set("database.name", parsed["name"])
			}
			if parsed["user"] != "" {
				v.Set("database.user", parsed["user"])
			}
			if parsed["password"] != "" {
				v.Set("database.password", parsed["password"])
			}
			if parsed["sslmode"] != "" {
				v.Set("database.ssl_mode", parsed["sslmode"])
			}
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// parseDBURL parses a postgres:// or postgresql:// URL into its components.
func parseDBURL(rawURL string) (map[string]string, error) {
	result := make(map[string]string)

	// Strip scheme
	s := rawURL
	for _, prefix := range []string{"postgresql://", "postgres://"} {
		if strings.HasPrefix(s, prefix) {
			s = s[len(prefix):]
			break
		}
	}

	// Split user:pass@host:port/dbname?params
	atIdx := strings.LastIndex(s, "@")
	if atIdx < 0 {
		return nil, fmt.Errorf("invalid DATABASE_URL: missing @")
	}
	userInfo := s[:atIdx]
	hostInfo := s[atIdx+1:]

	// Parse user:password
	if colonIdx := strings.Index(userInfo, ":"); colonIdx >= 0 {
		result["user"] = userInfo[:colonIdx]
		result["password"] = userInfo[colonIdx+1:]
	} else {
		result["user"] = userInfo
	}

	// Parse host:port/dbname?params
	if slashIdx := strings.Index(hostInfo, "/"); slashIdx >= 0 {
		hostPort := hostInfo[:slashIdx]
		rest := hostInfo[slashIdx+1:]

		// Strip query params from dbname
		if qIdx := strings.Index(rest, "?"); qIdx >= 0 {
			params := rest[qIdx+1:]
			rest = rest[:qIdx]
			// Parse sslmode from query params
			for _, param := range strings.Split(params, "&") {
				if strings.HasPrefix(param, "sslmode=") {
					result["sslmode"] = strings.TrimPrefix(param, "sslmode=")
				}
			}
		}
		result["name"] = rest

		if colonIdx := strings.Index(hostPort, ":"); colonIdx >= 0 {
			result["host"] = hostPort[:colonIdx]
			result["port"] = hostPort[colonIdx+1:]
		} else {
			result["host"] = hostPort
		}
	}

	return result, nil
}

// Validate checks that all required configuration fields are present.
// Returns an error listing all missing fields so operators can fix them
// in one pass rather than discovering them one at a time.
func (c *Config) Validate() error {
	var missing []string

	if c.Database.Host == "" {
		missing = append(missing, "DB_HOST")
	}
	if c.Database.Name == "" {
		missing = append(missing, "DB_NAME")
	}
	if c.Database.User == "" {
		missing = append(missing, "DB_USER")
	}
	if c.Database.Password == "" {
		missing = append(missing, "DB_PASSWORD")
	}

	// Keycloak is only required when DEV_MODE=false.
	// In staging/demo deployments (DEV_MODE=true), the DevAuthMiddleware
	// handles authentication without Keycloak, so these vars can be omitted.
	if !c.Server.DevMode {
		if c.Keycloak.URL == "" {
			missing = append(missing, "KEYCLOAK_URL")
		}
		if c.Keycloak.Realm == "" {
			missing = append(missing, "KEYCLOAK_REALM")
		}
		if c.Keycloak.ClientID == "" {
			missing = append(missing, "KEYCLOAK_CLIENT_ID")
		}
	}

	// MinIO is optional in staging (photo uploads gracefully disabled when not configured).
	// In production with APP_ENV=production AND DEV_MODE=false, MinIO is required.
	if c.App.Env == "production" && !c.Server.DevMode {
		if c.MinIO.Endpoint == "" {
			missing = append(missing, "MINIO_ENDPOINT")
		}
		if c.MinIO.AccessKey == "" {
			missing = append(missing, "MINIO_ACCESS_KEY")
		}
		if c.MinIO.SecretKey == "" {
			missing = append(missing, "MINIO_SECRET_KEY")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return nil
}
