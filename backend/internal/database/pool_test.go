package database

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyPool(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://u:p@localhost:5432/db")
	require.NoError(t, err)
	defMax, defLife := cfg.MaxConns, cfg.MaxConnLifetime

	applyPool(cfg, PoolSettings{})
	assert.Equal(t, defMax, cfg.MaxConns, "zero settings keep the driver defaults")
	assert.Equal(t, defLife, cfg.MaxConnLifetime)

	applyPool(cfg, PoolSettings{MaxConns: 20, MinConns: 2, MaxConnLifetime: time.Hour, MaxConnIdleTime: 10 * time.Minute})
	assert.EqualValues(t, 20, cfg.MaxConns)
	assert.EqualValues(t, 2, cfg.MinConns)
	assert.Equal(t, time.Hour, cfg.MaxConnLifetime)
	assert.Equal(t, 10*time.Minute, cfg.MaxConnIdleTime)
}

func TestPoolerHint(t *testing.T) {
	tests := []struct {
		name, dsn string
		want      bool
	}{
		{"direct endpoint", "postgres://u:p@ep-cool-123.us-east-2.aws.neon.tech/db?sslmode=require", false},
		{"pooled endpoint, default mode", "postgres://u:p@ep-cool-123-pooler.us-east-2.aws.neon.tech/db?sslmode=require", true},
		{"pooled endpoint, simple protocol", "postgres://u:p@ep-cool-123-pooler.us-east-2.aws.neon.tech/db?sslmode=require&default_query_exec_mode=simple_protocol", false},
		{"pooled endpoint, exec mode", "postgres://u:p@ep-cool-123-pooler.us-east-2.aws.neon.tech/db?default_query_exec_mode=exec", false},
		{"local", "postgres://u:p@localhost:5432/db", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := pgxpool.ParseConfig(tt.dsn)
			require.NoError(t, err)
			assert.Equal(t, tt.want, PoolerHint(cfg.ConnConfig))
		})
	}
}
