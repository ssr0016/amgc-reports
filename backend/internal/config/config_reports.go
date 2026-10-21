package config

// ReportsConfig holds the rate limit shared by the reports write routes
// (POST, PUT, DELETE, PATCH). Reads are not limited. Per session user.
type ReportsConfig struct {
	WriteRatePerMinute int
	WriteBurst         int
}

func (cfg *Config) reportsConfig() error {
	writeRate, err := getEnvInt("REPORTS_WRITE_RATE_LIMIT", 60)
	if err != nil {
		return err
	}
	writeBurst, err := getEnvInt("REPORTS_WRITE_BURST", 10)
	if err != nil {
		return err
	}
	cfg.Reports = ReportsConfig{WriteRatePerMinute: writeRate, WriteBurst: writeBurst}
	return nil
}
