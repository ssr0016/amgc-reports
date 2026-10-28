package statsimpl

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
)

// Synthetic fixtures only: no database, no production rows.

var manila = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		panic(err)
	}
	return loc
}()

// testNow is 28 October 2026 in Manila: September is in its grace period,
// October has not ended.
var testNow = time.Date(2026, 10, 28, 10, 0, 0, 0, manila)

func f64(v float64) *float64 { return &v }

func month(y, m int) *time.Time {
	t := time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
	return &t
}

func churchRec(id int64, start, end *time.Time, status string) stats.ChurchRecord {
	return stats.ChurchRecord{ID: id, Name: fmt.Sprintf("Church %02d", id), StartMonth: start, EndMonth: end,
		Accounts: []stats.ChurchAccountInfo{{UserID: id + 100, Name: fmt.Sprintf("Worker %02d", id), Status: status}}}
}

// weeks builds a metric's five slots; nil is a blank week.
func weeks(v ...*float64) *[5]*float64 {
	var out [5]*float64
	copy(out[:], v)
	return &out
}

func rep(id, church int64, year, m int, status string, metrics map[string]*[5]*float64) stats.ReportData {
	created := time.Date(year, time.Month(m)+1, 5, 9, 0, 0, 0, manila) // on time
	return stats.ReportData{ID: id, ChurchID: church, ChurchName: fmt.Sprintf("Church %02d", church), Worker: "W", Year: year, Month: m,
		Status: status, CreatedAt: created, UpdatedAt: created, Metrics: metrics}
}

func query(metric string) stats.AttendanceQuery {
	return stats.AttendanceQuery{Year: 2026, Metric: metric, Status: stats.AttendanceStatusApproved, Period: stats.AttendancePeriodFullYear}
}

func metricSummary(t *testing.T, a stats.AttendanceAnalytics, key string) stats.MetricSummary {
	t.Helper()
	for _, m := range a.Metrics {
		if m.Metric == key {
			return m
		}
	}
	t.Fatalf("no metric %s", key)
	return stats.MetricSummary{}
}

func cellOf(t *testing.T, a stats.AttendanceAnalytics, church int64, m int) stats.ComplianceCell {
	t.Helper()
	for _, r := range a.Compliance {
		if r.ChurchID == church {
			return r.Cells[m-1]
		}
	}
	t.Fatalf("no compliance row for church %d", church)
	return stats.ComplianceCell{}
}

func churchValue(t *testing.T, m stats.MetricSummary, church int64) stats.ChurchValue {
	t.Helper()
	for _, c := range m.Churches {
		if c.ChurchID == church {
			return c
		}
	}
	t.Fatalf("no value for church %d", church)
	return stats.ChurchValue{}
}

// complete is 24 churches reporting January-September: worship 50 on weeks 1-4
// (four services), prayer meeting 10 on weeks 1-5 (five).
func complete() ([]stats.ChurchRecord, []stats.ReportData) {
	var churches []stats.ChurchRecord
	var reports []stats.ReportData
	id := int64(0)
	for c := int64(1); c <= 24; c++ {
		churches = append(churches, churchRec(c, month(2026, 1), nil, "active"))
		for m := 1; m <= 9; m++ {
			id++
			reports = append(reports, rep(id, c, 2026, m, "approved", map[string]*[5]*float64{
				"worship_service": weeks(f64(50), f64(50), f64(50), f64(50)),
				"prayer_meeting":  weeks(f64(10), f64(10), f64(10), f64(10), f64(10)),
				"tithes_offering": weeks(f64(100.25)),
			}))
		}
	}
	return churches, reports
}

func TestAnalyze_CompleteNetwork(t *testing.T) {
	churches, reports := complete()
	a := analyze(analysis{q: query("worship_service"), now: testNow, churches: churches, reports: reports})

	assert.Equal(t, 9, a.CutoffMonth, "October has not ended")
	assert.Len(t, a.Churches, 24, "read from the data, not a constant")
	assert.Equal(t, stats.Coverage{Months: 9, ReportsInScope: 216, ExpectedReported: 216, ChurchesReported: 24, Expected: 216, Churches: 24}, a.Coverage)

	w := metricSummary(t, a, "worship_service")
	require.NotNil(t, w.Total)
	assert.InDelta(t, 24*4*50*9, *w.Total, 1e-9, "attendance entries")
	assert.Equal(t, 24*4*9, w.Observations, "recorded church-services")
	assert.InDelta(t, 50, *w.AvgPerChurchService, 1e-9, "per church-service")
	assert.InDelta(t, 1200, *w.NetworkWeekly, 1e-9, "network weekly: 24 × 50")
	assert.Equal(t, 36, w.NetworkSlots, "4 slots × 9 months; week 5 had no worship")
	assert.Equal(t, 36, w.NetworkSlotsFull)

	p := metricSummary(t, a, "prayer_meeting")
	assert.Equal(t, 24*5*9, p.Observations, "five prayer meetings a month, its own denominator")
	assert.InDelta(t, 10, *p.AvgPerChurchService, 1e-9)
	assert.InDelta(t, 240, *p.NetworkWeekly, 1e-9)

	tithes := metricSummary(t, a, "tithes_offering")
	assert.InDelta(t, 24*9*100.25, *tithes.Total, 1e-6)
	assert.Nil(t, tithes.AvgPerChurchService, "no per-service average outside attendance")

	oct := a.Trend[9]
	assert.False(t, oct.Counted)
	assert.Nil(t, oct.Value, "a month not yet ended is no data, not 0")
	assert.Equal(t, stats.CellNotYetOpen, cellOf(t, a, 1, 10).State)
	sep := cellOf(t, a, 1, 9)
	assert.Equal(t, stats.CellSubmitted, sep.State)
	assert.Equal(t, stats.TimelyOnTime, sep.Timeliness)
	assert.Equal(t, 216, a.ComplianceTotals.Expected)
	assert.Equal(t, 216, a.ComplianceTotals.Approved)
	require.Len(t, a.ChurchRows, 24)
	assert.InDelta(t, 1, *a.ChurchRows[0].Completeness, 1e-9)
	assert.Len(t, a.Slots, 45)
}

func TestAnalyze_ZeroBlankAndMissing(t *testing.T) {
	churches := []stats.ChurchRecord{churchRec(1, month(2026, 1), nil, "active"), churchRec(2, month(2026, 1), nil, "active"), churchRec(3, month(2026, 1), nil, "active")}
	reports := []stats.ReportData{
		// Church 1: an explicit zero and 100: two services, average 50.
		rep(1, 1, 2026, 1, "approved", map[string]*[5]*float64{"worship_service": weeks(f64(0), f64(100))}),
		// Church 2 reported, but left worship blank.
		rep(2, 2, 2026, 1, "approved", map[string]*[5]*float64{"tithes_offering": weeks(f64(0))}),
		// Church 3 has no report.
	}
	a := analyze(analysis{q: query("worship_service"), now: testNow, churches: churches, reports: reports})
	w := metricSummary(t, a, "worship_service")
	assert.InDelta(t, 100, *w.Total, 1e-9)
	assert.Equal(t, 2, w.Observations, "the zero is a service; the blank is not")
	assert.Equal(t, 1, w.ZeroObservations)
	assert.InDelta(t, 50, *w.AvgPerChurchService, 1e-9)

	assert.Equal(t, "value", churchValue(t, w, 1).State)
	assert.Equal(t, "blank", churchValue(t, w, 2).State)
	assert.Nil(t, churchValue(t, w, 2).Value, "a blank is not 0")
	assert.Equal(t, "no_report", churchValue(t, w, 3).State)

	jan, feb := a.Trend[0], a.Trend[1]
	assert.InDelta(t, 100, *jan.Value, 1e-9)
	assert.Equal(t, 2, jan.ChurchesReported)
	assert.Equal(t, 3, jan.ChurchesExpected)
	assert.True(t, jan.Partial)
	assert.Nil(t, feb.Value, "nobody reported February: a gap, not 0")
	assert.True(t, feb.Counted)

	tithes := metricSummary(t, a, "tithes_offering")
	assert.Equal(t, 1, tithes.ChurchesWithValue, "an offering of 0 is recorded")
	assert.InDelta(t, 0, *tithes.Total, 1e-9)

	assert.Equal(t, stats.CellNotFiled, cellOf(t, a, 3, 1).State)
	assert.Equal(t, stats.CellOverdue, cellOf(t, a, 3, 9).State, "September is in its grace period on 28 Oct")
	assert.Equal(t, stats.CellNotYetOpen, cellOf(t, a, 3, 10).State)
}

func TestAnalyze_Eligibility(t *testing.T) {
	churches := []stats.ChurchRecord{
		churchRec(1, month(2026, 7), nil, "active"),              // starts midyear
		churchRec(2, month(2026, 1), month(2026, 3), "inactive"), // ended in March
		churchRec(3, nil, nil, "active"),                         // not reviewed
	}
	churches[1].Accounts[0].Deleted = true
	reports := []stats.ReportData{
		rep(1, 2, 2026, 1, "approved", map[string]*[5]*float64{"worship_service": weeks(f64(30))}),
		rep(2, 2, 2026, 3, "approved", map[string]*[5]*float64{"worship_service": weeks(f64(40))}),
		rep(3, 3, 2026, 2, "approved", map[string]*[5]*float64{"worship_service": weeks(f64(5))}),
	}
	a := analyze(analysis{q: query("worship_service"), now: testNow, churches: churches, reports: reports})
	require.Len(t, a.Churches, 3, "an inactive church with only a deleted account keeps its history")

	assert.Equal(t, stats.CellNotRequired, cellOf(t, a, 1, 6).State, "before its start month")
	assert.Equal(t, stats.CellNotFiled, cellOf(t, a, 1, 7).State)
	assert.Equal(t, stats.CellNotRequired, cellOf(t, a, 2, 4).State, "after its end month")
	assert.Equal(t, stats.CellNotFiled, cellOf(t, a, 2, 2).State)
	assert.Equal(t, stats.CellSubmitted, cellOf(t, a, 2, 3).State)
	assert.Equal(t, stats.CellNotReviewed, cellOf(t, a, 3, 1).State, "no start month: never missing")
	assert.Equal(t, stats.CellSubmitted, cellOf(t, a, 3, 2).State, "a report is counted even before review")
	assert.True(t, a.Churches[2].NeedsReview)
	assert.Equal(t, 1, a.ComplianceTotals.ChurchesNeedingReview)

	rows := map[int64]stats.ComplianceRow{}
	for _, r := range a.Compliance {
		rows[r.ChurchID] = r
	}
	assert.Equal(t, 3, rows[1].Expected, "July-September")
	assert.Equal(t, 3, rows[2].Expected, "January-March")
	assert.Equal(t, 0, rows[3].Expected, "no start month: nothing is owed")
	assert.Equal(t, 1, rows[3].Outside, "its report is kept, outside the expected months")

	w := metricSummary(t, a, "worship_service")
	assert.InDelta(t, 75, *w.Total, 1e-9, "historical reports of the ended church still count")
}

func TestAnalyze_PendingUnderApprovedFilter(t *testing.T) {
	churches := []stats.ChurchRecord{churchRec(1, month(2026, 1), nil, "active"), churchRec(2, month(2026, 1), nil, "active")}
	reports := []stats.ReportData{
		rep(1, 1, 2026, 9, "approved", map[string]*[5]*float64{"worship_service": weeks(f64(20))}),
		rep(2, 2, 2026, 9, "pending", map[string]*[5]*float64{"worship_service": weeks(f64(70))}),
	}
	a := analyze(analysis{q: query("worship_service"), now: testNow, churches: churches, reports: reports})
	w := metricSummary(t, a, "worship_service")
	assert.InDelta(t, 20, *w.Total, 1e-9)
	assert.Equal(t, "not_approved", churchValue(t, w, 2).State, "pending is not missing")
	sep := cellOf(t, a, 2, 9)
	assert.Equal(t, stats.CellSubmitted, sep.State, "compliance ignores the status filter")
	assert.Equal(t, "pending", sep.ReviewStatus)

	q := query("worship_service")
	q.Status = stats.AttendanceStatusAll
	a = analyze(analysis{q: q, now: testNow, churches: churches, reports: reports})
	assert.InDelta(t, 90, *metricSummary(t, a, "worship_service").Total, 1e-9)
}

func TestAnalyze_Comparison(t *testing.T) {
	churches := []stats.ChurchRecord{churchRec(1, month(2025, 1), nil, "active"), churchRec(2, month(2025, 1), nil, "active"), churchRec(3, month(2025, 1), nil, "active")}
	reports := []stats.ReportData{
		rep(1, 1, 2026, 1, "approved", map[string]*[5]*float64{"worship_service": weeks(f64(60), f64(60)), "baptism": weeks(f64(2))}),
		rep(2, 1, 2025, 1, "approved", map[string]*[5]*float64{"worship_service": weeks(f64(40), f64(40)), "baptism": weeks(f64(0))}),
		// Church 2 reported last year only: its drop is missing data, not a decline.
		rep(3, 2, 2025, 1, "approved", map[string]*[5]*float64{"worship_service": weeks(f64(500))}),
		// Church 3 reported this year only.
		rep(4, 3, 2026, 2, "approved", map[string]*[5]*float64{"worship_service": weeks(f64(10))}),
		// Last year's October and later are outside this year's cutoff.
		rep(5, 1, 2025, 11, "approved", map[string]*[5]*float64{"worship_service": weeks(f64(999))}),
	}
	q := query("worship_service")
	q.Compare = true
	a := analyze(analysis{q: q, now: testNow, churches: churches, reports: reports})
	require.NotNil(t, a.Comparison)
	c := a.Comparison
	assert.Equal(t, 2025, c.Year)
	assert.Equal(t, 2, c.Coverage.ReportsInScope, "the same months only: November 2025 is beyond the cutoff")
	pw := metricSummary(t, stats.AttendanceAnalytics{Metrics: c.Metrics}, "worship_service")
	assert.InDelta(t, 580, *pw.Total, 1e-9)
	assert.Nil(t, c.Trend[10].Value, "November is not counted for either year")

	l := c.LikeForLike
	assert.Equal(t, 1, l.Churches)
	assert.InDelta(t, 120, l.CurrentTotal, 1e-9)
	assert.InDelta(t, 80, l.PreviousTotal, 1e-9)
	require.NotNil(t, l.Change)
	assert.InDelta(t, 0.5, *l.Change, 1e-9, "average 60 vs 40")

	rows := map[int64]stats.ChurchMetric{}
	for _, r := range a.ChurchRows {
		rows[r.ChurchID] = r
	}
	assert.Nil(t, rows[2].Change, "no data this year: unavailable, not -100%")
	assert.Nil(t, rows[3].Change, "no data last year: unavailable, not new")

	pb := metricSummary(t, stats.AttendanceAnalytics{Metrics: c.Metrics}, "baptism")
	assert.InDelta(t, 0, *pb.Total, 1e-9)
	assert.Nil(t, change(metricSummary(t, a, "baptism").Total, pb.Total), "from 0: no percentage")
}

func TestAnalyze_DuplicatesAndFilter(t *testing.T) {
	churches := []stats.ChurchRecord{churchRec(1, month(2026, 1), nil, "active"), churchRec(2, month(2026, 1), nil, "active")}
	late := time.Date(2026, 3, 1, 0, 0, 0, 0, manila)
	dup := rep(2, 1, 2026, 1, "pending", map[string]*[5]*float64{"worship_service": weeks(f64(999))})
	dup.UpdatedAt = late // updated later, but not approved
	reports := []stats.ReportData{
		rep(1, 1, 2026, 1, "approved", map[string]*[5]*float64{"worship_service": weeks(f64(40))}),
		dup,
		rep(3, 2, 2026, 1, "approved", map[string]*[5]*float64{"worship_service": weeks(f64(7))}),
	}
	q := query("worship_service")
	q.Status = stats.AttendanceStatusAll
	a := analyze(analysis{q: q, now: testNow, churches: churches, reports: reports})
	assert.InDelta(t, 47, *metricSummary(t, a, "worship_service").Total, 1e-9, "one report per church-month")
	assert.Equal(t, 1, cellOf(t, a, 1, 1).Duplicates)
	assert.Equal(t, int64(1), cellOf(t, a, 1, 1).ReportID, "the approved one is counted")

	q.ChurchIDs = []int64{2}
	a = analyze(analysis{q: q, now: testNow, churches: churches, reports: reports})
	assert.Len(t, a.Churches, 1)
	assert.Len(t, a.ChurchOptions, 2, "the filter still offers every church")
	assert.InDelta(t, 7, *metricSummary(t, a, "worship_service").Total, 1e-9)
	assert.Len(t, a.Compliance, 1)
	assert.Len(t, a.Reports, 1)
}

func TestAnalyze_NoDataAndFuture(t *testing.T) {
	churches := []stats.ChurchRecord{churchRec(1, month(2026, 1), nil, "active")}
	q := query("worship_service")
	q.Year = 2027
	a := analyze(analysis{q: q, now: testNow, churches: churches})
	assert.Equal(t, 0, a.CutoffMonth)
	assert.Nil(t, metricSummary(t, a, "worship_service").Total)
	for _, p := range a.Trend {
		assert.False(t, p.Counted)
		assert.Nil(t, p.Value)
	}
	assert.Equal(t, 0, a.ComplianceTotals.Expected, "no month of 2027 is due")

	q.Year, q.Period = 2026, "october"
	a = analyze(analysis{q: q, now: testNow, churches: churches})
	assert.Equal(t, 0, a.CutoffMonth)
	assert.Equal(t, stats.CellNotYetOpen, a.Compliance[0].Cells[0].State)
}

func TestMonthStateAndTimeliness(t *testing.T) {
	at := func(y, m, d, h, min int) time.Time { return time.Date(y, time.Month(m), d, h, min, 0, 0, manila) }
	for _, tt := range []struct {
		now  time.Time
		want string
	}{
		{at(2026, 9, 30, 23, 59), stats.MonthNotYetOpen},
		{at(2026, 10, 1, 0, 0), stats.MonthOpen},
		{at(2026, 10, 15, 23, 59), stats.MonthOpen},
		{at(2026, 10, 16, 0, 0), stats.MonthGrace},
		{at(2026, 10, 31, 23, 59), stats.MonthGrace},
		{at(2026, 11, 1, 0, 0), stats.MonthClosed},
	} {
		assert.Equal(t, tt.want, monthState(2026, time.September, tt.now), tt.now.String())
	}
	// November's grace ends on the 31st of December; February's on 31 March, not "the 31st" of every month.
	assert.Equal(t, stats.MonthGrace, monthState(2026, time.November, at(2026, 12, 31, 12, 0)))
	assert.Equal(t, stats.MonthClosed, monthState(2026, time.March, at(2026, 5, 1, 0, 0)))
	assert.Equal(t, stats.MonthGrace, monthState(2026, time.March, at(2026, 4, 30, 23, 0)))

	utc := func(t time.Time) time.Time { return t.UTC() }
	assert.Equal(t, stats.TimelyOnTime, timeliness(2026, time.September, utc(at(2026, 10, 15, 23, 59)), manila))
	assert.Equal(t, stats.TimelyLate, timeliness(2026, time.September, utc(at(2026, 10, 16, 0, 0)), manila), "Manila time, whatever the stored zone")
	assert.Equal(t, stats.TimelyAfterCutoff, timeliness(2026, time.September, utc(at(2026, 11, 1, 0, 0)), manila))
	assert.Equal(t, stats.TimelyBeforeOpen, timeliness(2026, time.September, utc(at(2026, 9, 20, 0, 0)), manila))
}

func TestChange(t *testing.T) {
	assert.Nil(t, change(f64(5), f64(0)))
	assert.Nil(t, change(nil, f64(5)))
	assert.Nil(t, change(f64(5), nil))
	assert.InDelta(t, 0.5, *change(f64(15), f64(10)), 1e-9)
	assert.InDelta(t, -0.5, *change(f64(5), f64(10)), 1e-9)
}

func TestAnalyze_OrganizationDefaultStart(t *testing.T) {
	inactive := churchRec(4, nil, nil, "deceased")
	gone := churchRec(5, nil, nil, "inactive") // inactive, no reports, no end month: not listed
	churches := []stats.ChurchRecord{
		churchRec(1, nil, nil, "active"),             // uses the default
		churchRec(2, month(2026, 10), nil, "active"), // joined later: an override
		churchRec(3, nil, month(2026, 9), "active"),  // stops after September
		inactive, gone,
	}
	reports := []stats.ReportData{
		rep(1, 1, 2026, 9, "approved", map[string]*[5]*float64{"worship_service": weeks(f64(50))}),
		rep(2, 4, 2026, 3, "approved", map[string]*[5]*float64{"worship_service": weeks(f64(20))}), // history before the default
	}
	a := analyze(analysis{q: query("worship_service"), now: testNow, churches: churches, reports: reports, defaultStart: month(2026, 9)})

	assert.Equal(t, "2026-09", a.DefaultStartMonth)
	require.Len(t, a.Churches, 4, "an inactive church with no reports and no end month is not listed")
	byID := map[int64]stats.ChurchScope{}
	for _, c := range a.Churches {
		byID[c.ID] = c
	}
	assert.True(t, byID[1].StartIsDefault)
	assert.Equal(t, "2026-09", byID[1].StartMonth)
	assert.False(t, byID[2].StartIsDefault)
	assert.False(t, byID[4].Active)
	assert.False(t, byID[1].NeedsReview, "the default makes every church's eligibility known")
	assert.Equal(t, 0, a.ComplianceTotals.ChurchesNeedingReview)

	assert.Equal(t, stats.CellNotRequired, cellOf(t, a, 1, 8).State, "before the organization start")
	assert.Equal(t, stats.CellSubmitted, cellOf(t, a, 1, 9).State)
	assert.Equal(t, stats.CellNotRequired, cellOf(t, a, 2, 9).State, "before its own start")
	assert.Equal(t, stats.CellOverdue, cellOf(t, a, 3, 9).State)
	assert.Equal(t, stats.CellNotRequired, cellOf(t, a, 3, 10).State, "after its end month")
	assert.Equal(t, stats.CellInactive, cellOf(t, a, 4, 9).State, "no active account, no end month: never Not Filed")
	assert.Equal(t, stats.CellSubmitted, cellOf(t, a, 4, 3).State, "its history stays")

	// September: churches 1 and 3 are expected; church 4 is not.
	sep := a.Trend[8]
	assert.Equal(t, 2, sep.ChurchesExpected)
	assert.Equal(t, 1, sep.ChurchesReported)
	assert.True(t, sep.Partial)
	assert.Equal(t, 0, a.Trend[2].ChurchesExpected, "March is before every church's start: nothing is expected")
	assert.Equal(t, 1, a.Trend[2].ChurchesReported, "the report is still shown")
	assert.False(t, a.Trend[2].Partial)
	assert.Equal(t, 1, a.Coverage.OutsideReports)
	assert.Equal(t, 1, a.Coverage.ExpectedReported)
	assert.Equal(t, 2, a.Coverage.Expected, "September for churches 1 and 3: not inflated by March")
	assert.InDelta(t, 70, *metricSummary(t, a, "worship_service").Total, 1e-9, "reports before the start month still count")
}
