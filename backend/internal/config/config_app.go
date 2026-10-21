package config

// AppConfig holds general app settings.
type AppConfig struct {
	Env  string
	Port string
	URL  string
}

func (cfg *Config) appConfig() {
	// Port: PORT is injected by platforms such as Render; APP_PORT wins.
	cfg.App = AppConfig{
		Env:  getEnv("APP_ENV", "local"),
		Port: getEnv("APP_PORT", getEnv("PORT", "8080")),
		URL:  getEnv("APP_URL", "http://localhost:8080"),
	}
}

// IsProduction returns true if running in production.
func (c *Config) IsProduction() bool {
	return c.Stage == StageProd
}

// IsDevelopment returns true if running in development.
func (c *Config) IsDevelopment() bool {
	return c.Stage == StageLocal || c.Stage == StageDev
}

// IsDebug returns true if debug mode is enabled.
func (c *Config) IsDebug() bool {
	return c.Stage != StageProd
}
