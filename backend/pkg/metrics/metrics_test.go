package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestHandler_RefreshesPoolStatsOnScrape(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://test:test@localhost:1/test?sslmode=disable&pool_min_conns=0")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	handler := Handler(pool)
	for range 2 {
		DBConnectionsActive.Set(99)
		DBConnectionsIdle.Set(88)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
		for _, metric := range []string{"db_connections_active 0\n", "db_connections_idle 0\n"} {
			if !strings.Contains(rec.Body.String(), metric) {
				t.Errorf("scrape missing %q", metric)
			}
		}
	}
}
