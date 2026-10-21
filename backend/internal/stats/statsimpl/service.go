package statsimpl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
)

// service holds the rule for which month the admin cards describe: the
// previous month in Philippine time (D-A7, D-F8) unless the caller names one.
type service struct {
	store stats.Store
	now   func() time.Time
	loc   *time.Location // months are counted in Philippine time, whatever the server clock says
}

// NewService returns the stats.Service over store.
func NewService(store stats.Store) stats.Service { return newService(store) }

// newService is NewService with the concrete type, so tests can set the clock.
func newService(store stats.Store) *service {
	loc, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		panic(fmt.Sprintf("load Asia/Manila (is tzdata installed?): %v", err))
	}
	return &service{store: store, now: time.Now, loc: loc}
}

// Get returns the counts for the given month and year. Both nil means the
// previous month. The month's canonical name is queried and returned.
func (s *service) Get(ctx context.Context, q stats.GetStatsQuery) (stats.AdminStats, error) {
	month, year := q.Month, q.Year
	if (month == nil) != (year == nil) {
		return stats.AdminStats{}, stats.ErrPeriod
	}
	var m string
	var y int
	if month == nil {
		t := s.now().In(s.loc)
		prev := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, s.loc).AddDate(0, -1, 0)
		m, y = report.Months[prev.Month()-1], prev.Year()
	} else {
		canonical, ok := canonicalMonth(*month)
		if !ok {
			return stats.AdminStats{}, stats.ErrPeriod
		}
		m, y = canonical, *year
	}
	out, err := s.store.Counts(ctx, m, y)
	if err != nil {
		return stats.AdminStats{}, fmt.Errorf("stats for %s %d: %w", m, y, err)
	}
	return out, nil
}

func canonicalMonth(name string) (string, bool) {
	for _, m := range report.Months {
		if strings.EqualFold(m, name) {
			return m, true
		}
	}
	return "", false
}
