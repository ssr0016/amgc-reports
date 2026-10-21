//go:build integration

package database_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

// The Go lists in the model package must equal the CHECK constraints (D-6): a
// metric, month or action added to one side only fails here.
func TestModelConstants_MatchCheckConstraints(t *testing.T) {
	tdb := migratedDB(t)
	quoted := regexp.MustCompile(`'([^']+)'`)

	tests := []struct {
		constraint string
		want       []string
	}{
		{"chk_report_metrics_metric", report.MetricKeys},
		{"chk_reports_month", report.Months},
		{"chk_audit_logs_action", audit.Actions},
		{"chk_users_status", user.UserStatuses},
	}
	for _, tt := range tests {
		t.Run(tt.constraint, func(t *testing.T) {
			var def string
			require.NoError(t, tdb.Pool.QueryRow(context.Background(),
				`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = $1`, tt.constraint).Scan(&def))

			var got []string
			for _, m := range quoted.FindAllStringSubmatch(def, -1) {
				got = append(got, m[1])
			}
			require.ElementsMatch(t, tt.want, got)
		})
	}
}
