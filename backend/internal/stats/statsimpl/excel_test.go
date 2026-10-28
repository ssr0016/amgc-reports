package statsimpl

import (
	"bytes"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

func TestMetricLabels_CoverEveryMetric(t *testing.T) {
	for _, k := range report.MetricKeys {
		if _, ok := metricLabels[k]; !ok {
			t.Errorf("no label for metric %q", k)
		}
	}
	if len(metricLabels) != len(report.MetricKeys) {
		t.Errorf("%d labels for %d metrics", len(metricLabels), len(report.MetricKeys))
	}
}

func TestChange(t *testing.T) {
	tests := []struct {
		cur, prev float64
		want      any
	}{{0, 0, pct(0)}, {5, 0, "New"}, {15, 10, pct(0.5)}, {5, 10, pct(-0.5)}}
	for _, tt := range tests {
		if got := change(tt.cur, tt.prev); got != tt.want {
			t.Errorf("change(%v, %v) = %v, want %v", tt.cur, tt.prev, got, tt.want)
		}
	}
}

func TestBuildAnalyticsWorkbook(t *testing.T) {
	ten := 10.0
	data := stats.AttendanceAnalytics{
		Year: 2026, Metric: "worship_service", Status: stats.AttendanceStatusApproved, Period: "full_year",
		Trend: []stats.AttendanceTrendPoint{
			{Month: "January", Week: 1, Value: 1234.5}, {Month: "January", Week: 2, Value: 0}, {Month: "March", Week: 1, Value: 2000},
		},
		Churches: []stats.AttendanceChurch{{ChurchName: "Bethel", Value: 3234.5}, {ChurchName: "Grace", Value: 0}},
		Summary: []stats.AttendanceMetricSummary{
			{Metric: "worship_service", Total: &ten, PeakMonth: "March", PeakValue: 10, Churches: []stats.AttendanceSummaryChurch{{ChurchName: "Bethel", Value: &ten}, {ChurchName: "Grace"}}},
			{Metric: "sunday_school", Churches: []stats.AttendanceSummaryChurch{{ChurchName: "Bethel"}, {ChurchName: "Grace"}}},
		},
		Comparison: &stats.AttendanceComparison{Year: 2025,
			Trend:    []stats.AttendanceTrendPoint{{Month: "January", Week: 1, Value: 1000}},
			Churches: []stats.AttendanceChurch{{ChurchName: "Zion", Value: 1000}},
		},
	}
	loc, _ := time.LoadLocation("Asia/Manila")
	// Dee's account was made in February, so January is not hers to report.
	dee := stats.TrackerRow{UserID: 4, Name: "Dee", ChurchName: "Hope", UserStatus: user.UserStatusActive, SubmissionStatus: stats.TrackerSubmitted, CreatedAt: time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)}
	deeJan := dee
	deeJan.SubmissionStatus = stats.TrackerNoReport
	compliance := []monthTracker{
		{year: 2026, month: time.January, rows: []stats.TrackerRow{
			{UserID: 1, Name: "Ana", ChurchName: "Bethel", UserStatus: user.UserStatusActive, SubmissionStatus: stats.TrackerSubmitted, ReviewStatus: stats.TrackerApproved},
			{UserID: 2, Name: "Ben", ChurchName: "Grace", UserStatus: user.UserStatusActive, SubmissionStatus: stats.TrackerNoReport},
			{UserID: 3, Name: "Cy", ChurchName: "Zion", UserStatus: user.UserStatusInactive, SubmissionStatus: stats.TrackerNoReport},
			deeJan,
		}},
		{year: 2026, month: time.February, rows: []stats.TrackerRow{
			{UserID: 1, Name: "Ana", ChurchName: "Bethel", UserStatus: user.UserStatusActive, SubmissionStatus: stats.TrackerSubmitted, ReviewStatus: stats.TrackerPending},
			{UserID: 2, Name: "Ben", ChurchName: "Grace", UserStatus: user.UserStatusActive, SubmissionStatus: stats.TrackerSubmitted, ReviewStatus: stats.TrackerNeedsRevision},
			dee,
		}},
	}
	raw, err := buildAnalyticsWorkbook(data, 24, compliance, time.Date(2026, 10, 28, 14, 5, 0, 0, loc))
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	if got, want := f.GetSheetList(), []string{"Summary", "Monthly Totals", "Weekly Trend", "Church Breakdown", "Full Metrics Summary", "Submission Compliance"}; !slices.Equal(got, want) {
		t.Fatalf("sheets = %v, want %v", got, want)
	}
	get := func(sheet, ref string) string {
		t.Helper()
		v, err := f.GetCellValue(sheet, ref)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for ref, want := range map[string]string{
		"A1": "AMGCReports — Analytics Report",
		"A2": "Worship Service · Full Year 2026 · Approved",
		"A3": "Generated 28 Oct 2026, 14:05 (Asia/Manila)",
		"A5": "KPI", "C5": "2025", "D5": "Change",
		"B6": "3,234.50", "C6": "1,000.00", "D6": "+223.5%",
		"B7": "March", "C7": "January",
		"B8": "1 out of 24", "C8": "1",
		// Jan-Sep 2026 are due (39 Sundays); all of 2025 is (52).
		"A9": "Average per Week", "B9": "82.94", "C9": "19.23", "D9": "+331.3%",
		// Active accounts submitted 4 of their 5 due reports; Cy is inactive and
		// Dee's January predates her account.
		"A10": "Submission Rate", "B10": "80.0%", "C10": "", "D10": "",
	} {
		if got := get("Summary", ref); got != want {
			t.Errorf("Summary!%s = %q, want %q", ref, got, want)
		}
	}
	// No change is a stored 0 (Excel shows "0.0%"; excelize's renderer ignores the zero section).
	if v, _ := f.GetCellValue("Summary", "D8", excelize.Options{RawCellValue: true}); v != "0" {
		t.Errorf("Summary!D8 raw = %q, want 0", v)
	}
	if merged, _ := f.GetMergeCells("Summary"); len(merged) != 1 || merged[0].GetStartAxis() != "A1" || merged[0].GetEndAxis() != "F1" {
		t.Errorf("title merge = %v, want A1:F1", merged)
	}

	// The zero week is left out; the weeks with data keep their values.
	if got := []string{get("Weekly Trend", "A6"), get("Weekly Trend", "B6"), get("Weekly Trend", "C6"), get("Weekly Trend", "A7"), get("Weekly Trend", "A8")}; !slices.Equal(got, []string{"January", "1", "1,234.50", "March", ""}) {
		t.Errorf("trend rows = %v", got)
	}
	// A church only in last year is listed, at 0 this year: -100%.
	if got := []string{get("Church Breakdown", "A8"), get("Church Breakdown", "B8"), get("Church Breakdown", "D8")}; !slices.Equal(got, []string{"Zion", "0.00", "-100.0%"}) {
		t.Errorf("Zion row = %v", got)
	}
	// Peak month sits after the total; a church with no value for a metric gets a blank cell, not 0.
	const full = "Full Metrics Summary"
	row := func(sheet string, r int) []string {
		var out []string
		for _, col := range "ABCDEFG" {
			out = append(out, get(sheet, string(col)+strconv.Itoa(r)))
		}
		return out
	}
	for _, c := range []struct {
		sheet string
		row   int
		want  []string
	}{
		{full, 5, []string{"Metric", "Total for Period", "Avg per Week", "Peak Month", "Peak Month Total", "Bethel", "Grace"}},
		{full, 6, []string{"Worship Service", "10.00", "0.26", "March", "10.00", "10.00", ""}},
		// A metric with no data has no average or peak month.
		{full, 7, []string{"Sunday School", "", "", "—", "", "", ""}},
		{"Monthly Totals", 5, []string{"Month", "Weeks", "2026 Total", "2026 Avg per Week", "2025 Total", "2025 Avg per Week", "Change (Avg)"}},
		{"Monthly Totals", 6, []string{"January", "4", "1,234.50", get("Monthly Totals", "D6"), "1,000.00", "250.00", get("Monthly Totals", "G6")}},
		// March 2026 has five Sundays; last year's March had nothing.
		{"Monthly Totals", 7, []string{"March", "5", "2,000.00", "400.00", "0.00", "0.00", "New"}},
		{"Monthly Totals", 8, []string{"", "", "", "", "", "", ""}},
		{"Submission Compliance", 5, []string{"Church", "Worker", "Jan", "Feb", "Submitted", "Approved", "Submission Rate"}},
		{"Submission Compliance", 6, []string{"Bethel", "Ana", "Approved", "Pending", "2", "1", "100.0%"}},
		{"Submission Compliance", 7, []string{"Grace", "Ben", "No Report", "Needs Revision", "1", "0", "50.0%"}},
		// A month before the account existed is a dash, outside the rate.
		{"Submission Compliance", 8, []string{"Hope", "Dee", "—", "Pending", "1", "0", "100.0%"}},
		// Inactive accounts are left out.
		{"Submission Compliance", 9, []string{"", "", "", "", "", "", ""}},
	} {
		if got := row(c.sheet, c.row); !slices.Equal(got, c.want) {
			t.Errorf("%s row %d = %q, want %q", c.sheet, c.row, got, c.want)
		}
	}

	// Status cells are colour-coded; the dash keeps the zebra fill.
	for ref, want := range map[string]string{"C6": "C6EFCE", "D6": "FFEB9C", "C7": "FFC7CE", "D7": "FFEB9C", "C8": ""} {
		id, _ := f.GetCellStyle("Submission Compliance", ref)
		style, err := f.GetStyle(id)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(style.Fill.Color, ""); got != want {
			t.Errorf("Submission Compliance!%s fill = %q, want %q", ref, got, want)
		}
	}

	panes, err := f.GetPanes(full)
	if err != nil || !panes.Freeze || panes.YSplit != headerRow || panes.XSplit != 1 {
		t.Errorf("%s panes = %+v, %v", full, panes, err)
	}
	if panes, _ := f.GetPanes("Weekly Trend"); !panes.Freeze || panes.YSplit != headerRow || panes.XSplit != 0 {
		t.Errorf("Weekly Trend panes = %+v", panes)
	}

	headerStyle, _ := f.GetCellStyle("Summary", "A5")
	st, _ := f.GetStyle(headerStyle)
	if !st.Font.Bold || st.Font.Color != "FFFFFF" || st.Fill.Color[0] != "404040" || st.Alignment.Horizontal != "center" {
		t.Errorf("header style = font %+v fill %+v", st.Font, st.Fill)
	}
	even, _ := f.GetCellStyle("Church Breakdown", "A6")
	odd, _ := f.GetCellStyle("Church Breakdown", "A7")
	if oddStyle, _ := f.GetStyle(odd); even == odd || len(oddStyle.Fill.Color) == 0 || oddStyle.Fill.Color[0] != "F2F2F2" {
		t.Errorf("zebra: even style %d, odd style %d", even, odd)
	}
	if w, _ := f.GetColWidth("Church Breakdown", "A"); w < 10 || w > 50 {
		t.Errorf("column A width = %v", w)
	}
}

func TestBuildAnalyticsWorkbook_EmptyPeriod(t *testing.T) {
	raw, err := buildAnalyticsWorkbook(stats.AttendanceAnalytics{Year: 2026, Metric: "baptism", Status: stats.AttendanceStatusAll, Period: "q3"}, 0, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if v, _ := f.GetCellValue("Weekly Trend", "A6"); v != "No data for this period." {
		t.Errorf("empty trend A6 = %q", v)
	}
	if v, _ := f.GetCellValue("Submission Compliance", "A6"); v != "No month of this period is due yet." {
		t.Errorf("empty compliance A6 = %q", v)
	}
	if v, _ := f.GetCellValue("Summary", "A2"); v != "Baptism · Q3 2026 · All Submitted" {
		t.Errorf("subtitle = %q", v)
	}
}

func TestSundays(t *testing.T) {
	for _, c := range []struct {
		year  int
		month time.Month
		want  int
	}{{2026, time.February, 4}, {2026, time.March, 5}, {2026, time.August, 5}, {2025, time.January, 4}, {2024, time.December, 5}} {
		if got := sundays(c.year, c.month); got != c.want {
			t.Errorf("sundays(%d, %s) = %d, want %d", c.year, c.month, got, c.want)
		}
	}
}

func TestDueMonths(t *testing.T) {
	now := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		year   int
		period string
		want   []time.Month
	}{
		{2026, "q3", []time.Month{time.July, time.August, time.September}},
		{2026, "q4", nil},      // October is still open
		{2026, "october", nil}, // the current month is not due yet
		{2025, "q1", []time.Month{time.January, time.February, time.March}},
		{2027, "january", nil},
	} {
		if got := dueMonths(c.year, c.period, now); !slices.Equal(got, c.want) {
			t.Errorf("dueMonths(%d, %q) = %v, want %v", c.year, c.period, got, c.want)
		}
	}
}
