package config

// ReportsConfig holds the per-session-user rate limits on the reports routes.
// Write is shared by POST, PUT, DELETE and PATCH; Read by the reports GETs,
// stats, tracker, audit logs and the Excel preview.
type ReportsConfig struct {
	WriteRatePerMinute int
	WriteBurst         int
	ReadRatePerMinute  int
	ReadBurst          int
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
	readRate, err := getEnvInt("REPORTS_READ_RATE_LIMIT", 60)
	if err != nil {
		return err
	}
	readBurst, err := getEnvInt("REPORTS_READ_BURST", 20)
	if err != nil {
		return err
	}
	cfg.Reports = ReportsConfig{
		WriteRatePerMinute: writeRate, WriteBurst: writeBurst,
		ReadRatePerMinute: readRate, ReadBurst: readBurst,
	}
	return nil
}
