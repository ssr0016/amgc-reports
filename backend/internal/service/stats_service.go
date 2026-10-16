package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
)

// ErrStatsPeriod is returned when only one of month and year is given, or the
// month is not an English month name. The caller maps it to 422.
var ErrStatsPeriod = errors.New("month and year must be given together, and the month must be a month name")

// StatsService holds the rule for which month the admin cards describe: the
// previous month in Philippine time (D-A7, D-F8) unless the caller names one.
type StatsService struct {
	stats repository.StatsRepository
	now   func() time.Time
	loc   *time.Location // months are counted in Philippine time, whatever the server clock says
}

func NewStatsService(stats repository.StatsRepository) *StatsService {
	loc, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		panic(fmt.Sprintf("load Asia/Manila (is tzdata installed?): %v", err))
	}
	return &StatsService{stats: stats, now: time.Now, loc: loc}
}

// Get returns the counts for the given month and year. Both nil means the
// previous month. The month's canonical name is queried and returned.
func (s *StatsService) Get(ctx context.Context, month *string, year *int) (model.AdminStats, error) {
	if (month == nil) != (year == nil) {
		return model.AdminStats{}, ErrStatsPeriod
	}
	var m string
	var y int
	if month == nil {
		t := s.now().In(s.loc)
		prev := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, s.loc).AddDate(0, -1, 0)
		m, y = model.Months[prev.Month()-1], prev.Year()
	} else {
		canonical, ok := canonicalMonth(*month)
		if !ok {
			return model.AdminStats{}, ErrStatsPeriod
		}
		m, y = canonical, *year
	}
	out, err := s.stats.Counts(ctx, m, y)
	if err != nil {
		return model.AdminStats{}, fmt.Errorf("stats for %s %d: %w", m, y, err)
	}
	return out, nil
}

func canonicalMonth(name string) (string, bool) {
	for _, m := range model.Months {
		if strings.EqualFold(m, name) {
			return m, true
		}
	}
	return "", false
}
