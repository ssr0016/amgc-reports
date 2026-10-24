package config

import (
	"fmt"
	"strings"
)

// StorageConfig is the Cloudflare R2 bucket that holds profile pictures. When
// every field is empty, uploads are off and the avatar route answers 503.
type StorageConfig struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	// PublicURL is the bucket's public base URL (r2.dev or a custom domain).
	PublicURL string
}

// Enabled reports whether R2 is configured.
func (s StorageConfig) Enabled() bool { return s.Bucket != "" }

func (cfg *Config) storageConfig() error {
	s := StorageConfig{
		AccountID:       getEnv("R2_ACCOUNT_ID", ""),
		AccessKeyID:     getEnv("R2_ACCESS_KEY_ID", ""),
		SecretAccessKey: getEnv("R2_SECRET_ACCESS_KEY", ""),
		Bucket:          getEnv("R2_BUCKET", ""),
		PublicURL:       strings.TrimRight(getEnv("R2_PUBLIC_URL", ""), "/"),
	}
	set := 0
	for _, v := range []string{s.AccountID, s.AccessKeyID, s.SecretAccessKey, s.Bucket, s.PublicURL} {
		if v != "" {
			set++
		}
	}
	if set != 0 && set != 5 {
		return fmt.Errorf("R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_BUCKET and R2_PUBLIC_URL must be set together")
	}
	cfg.Storage = s
	return nil
}
