package config

import (
	"os"
	"strings"
)

const minMetricsPasswordLen = 12

// ObservabilityConfig controls what the server exposes about itself.
type ObservabilityConfig struct {
	// SwaggerEnabled is whether /swagger/* is registered.
	SwaggerEnabled bool
	// MetricsUser and MetricsPassword are the Basic Auth credentials of
	// /metrics. When both are empty /metrics is open outside production and
	// not registered in production.
	MetricsUser     string
	MetricsPassword string
}

func (cfg *Config) observabilityConfig() error {
	// Swagger is on outside production and off in production unless asked for.
	swagger, err := getEnvBool("SWAGGER_ENABLED", !cfg.IsProduction())
	if err != nil {
		return err
	}
	cfg.Observability = ObservabilityConfig{
		SwaggerEnabled:  swagger,
		MetricsUser:     strings.TrimSpace(os.Getenv("METRICS_USER")),
		MetricsPassword: os.Getenv("METRICS_PASSWORD"),
	}
	return nil
}
