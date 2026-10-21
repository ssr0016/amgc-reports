package config

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string
	Format string
}

func (cfg *Config) loggingConfig() {
	cfg.Logging = LoggingConfig{
		Level:  getEnv("LOG_LEVEL", "info"),
		Format: getEnv("LOG_FORMAT", "text"),
	}
}
