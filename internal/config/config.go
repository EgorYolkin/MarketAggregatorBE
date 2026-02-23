// Package config provides environment-based configuration loading for the application.
package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config represents the root configuration loaded from the environment.
type Config struct {
	App    AppConfig
	Redis  RedisConfig
	Worker WorkerConfig
}

// AppConfig contains HTTP server and logging configurations.
type AppConfig struct {
	// Port on which the Fiber HTTP server will listen.
	Port string `envconfig:"PORT" default:"8080"`
	// LogLevel defines the verbosity of the logger (e.g. debug, info, error).
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`
	// LogFormat defines the output format of the logger (json or text).
	LogFormat string `envconfig:"LOG_FORMAT" default:"json"`
}

// RedisConfig contains connection parameters for the Redis store.
type RedisConfig struct {
	// Host is the address of the Redis server.
	Host string `envconfig:"REDIS_HOST" required:"true"`
	// Port is the port of the Redis server.
	Port string `envconfig:"REDIS_PORT" default:"6379"`
	// Password is the optional authentication password.
	Password string `envconfig:"REDIS_PASSWORD"` //nolint:gosec // Not a hardcoded secret, loaded from env.
	// DB is the database index to use.
	DB int `envconfig:"REDIS_DB" default:"0"`
}

// WorkerConfig contains settings for background polling processes.
type WorkerConfig struct {
	// PollInterval defines how frequently the service queries external APIs.
	PollInterval time.Duration `envconfig:"POLL_INTERVAL" default:"30s"`
	// Assets is a comma-separated list of symbols to track (e.g. BTC,ETH,SOL).
	Assets []string `envconfig:"TRACKED_ASSETS" default:"BTC,ETH"`
}

// LoadConfig parses environment variables into the Config struct.
// It fails fast if required variables are missing.
func LoadConfig() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return &cfg, nil
}
