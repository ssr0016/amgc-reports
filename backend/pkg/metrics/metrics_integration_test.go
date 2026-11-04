//go:build integration

package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

func TestHandler_TracksAcquiredAndReleasedConnections(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	handler := Handler(tdb.Pool)
	conn, err := tdb.Pool.Acquire(t.Context())
	require.NoError(t, err)
	t.Cleanup(conn.Release)
	for _, counts := range [][2]int{{1, 0}, {0, 1}} {
		if counts[0] == 0 {
			conn.Release()
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", nil))
		require.Equal(t, http.StatusOK, rec.Code)
		want := []string{"db_connections_active 1\n", "db_connections_idle 0\n"}
		if counts[0] == 0 {
			want = []string{"db_connections_active 0\n", "db_connections_idle 1\n"}
		}
		for _, metric := range want {
			require.True(t, strings.Contains(rec.Body.String(), metric), "scrape missing %q", metric)
		}
	}
}
