package config

// CORSConfig holds CORS settings.
type CORSConfig struct {
	AllowedOrigins []string
}

func (cfg *Config) corsConfig() {
	cfg.CORS = CORSConfig{
		AllowedOrigins: getEnvSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
	}
}
