package statsimpl

import (
	"bytes"
	"fmt"
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

// Every metric sits under exactly one analytics group.
func TestMetricGroups(t *testing.T) {
	var grouped []string
	for _, g := range stats.MetricGroups {
		grouped = append(grouped, g.Metrics...)
	}
	if got, want := slices.Sorted(slices.Values(grouped)), slices.Sorted(slices.Values(report.MetricKeys)); !slices.Equal(got, want) {
		t.Errorf("grouped metrics = %v, want %v", got, want)
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
			{Metric: "worship_service", Group: "Attendance", Total: &ten, PeakMonth: "March", PeakValue: 10, Churches: []stats.AttendanceSummaryChurch{{ChurchName: "Bethel", Value: &ten}, {ChurchName: "Grace"}}},
			{Metric: "sunday_school", Group: "Attendance", Churches: []stats.AttendanceSummaryChurch{{ChurchName: "Bethel"}, {ChurchName: "Grace"}}},
		},
		Comparison: &stats.AttendanceComparison{Year: 2025,
			Trend:    []stats.AttendanceTrendPoint{{Month: "January", Week: 1, Value: 1000}},
			Churches: []stats.AttendanceChurch{{ChurchName: "Zion", Value: 1000}},
			Detail:   []stats.AttendanceDetail{{Metric: "worship_service", ChurchName: "Zion", Month: 1, Value: 1000}},
			// Last year only January's first week was reported.
			Weeks: 1, WeeksByMonth: map[int]int{1: 1},
		},
		// Two weeks of January and one of March have reports; no other month does.
		Weeks: 3, WeeksByMonth: map[int]int{1: 2, 3: 1},
		Detail:           []stats.AttendanceDetail{{Metric: "worship_service", ChurchName: "Bethel", Month: 3, Value: 10}},
		NoReportChurches: []string{"Hope"},
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

	if got, want := f.GetSheetList(), []string{"Full Metrics Summary", "Summary", "Monthly Totals", "Weekly Trend", "Church Breakdown", "Metric Detail", "Submission Compliance"}; !slices.Equal(got, want) {
		t.Fatalf("sheets = %v, want %v", got, want)
	}
	// AutoFilter is stored as the sheet's hidden _FilterDatabase name.
	var filter string
	for _, n := range f.GetDefinedName() {
		if n.Name == "_xlnm._FilterDatabase" && n.Scope == "Metric Detail" {
			filter = n.RefersTo
		}
	}
	if want := "'Metric Detail'!$A$5:$F$7"; filter != want {
		t.Errorf("Metric Detail filter = %q, want %q", filter, want)
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
		// Averages divide by weeks with reports: 3 this year, 1 last year.
		"A9": "Average per Week", "B9": "1,078.17", "C9": "1,000.00", "D9": "+7.8%",
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
	row := func(sheet string, r, cols int) []string {
		var out []string
		for _, col := range "ABCDEFGHI"[:cols] {
			out = append(out, get(sheet, string(col)+strconv.Itoa(r)))
		}
		return out
	}
	for _, c := range []struct {
		sheet string
		row   int
		want  []string
	}{
		{full, 5, []string{"Metric", "Category", "Total for Period", "Avg per Week", "Peak Month", "Peak Month Total", "Bethel", "Grace", "Hope"}},
		// A church with no report at all reads No Report, unlike a blank metric.
		{full, 6, []string{"Worship Service", "Attendance", "10.00", "3.33", "March", "10.00", "10.00", "", "No Report"}},
		// A metric with no data has no average or peak month.
		{full, 7, []string{"Sunday School", "Attendance", "", "", "—", "", "", "", "No Report"}},
		{"Monthly Totals", 5, []string{"Month", "Weeks", "2026 Total", "2026 Avg per Week", "2025 Total", "2025 Avg per Week", "Change (Avg)"}},
		{"Monthly Totals", 6, []string{"January", "2", "1,234.50", "617.25", "1,000.00", "1,000.00", "-38.3%"}},
		// Last year's March had no reported week, so it has no average and no change.
		{"Monthly Totals", 7, []string{"March", "1", "2,000.00", "2,000.00", "0.00", "", ""}},
		{"Monthly Totals", 8, []string{"", "", "", "", "", "", ""}},
		{"Submission Compliance", 5, []string{"Church", "Worker", "Jan", "Feb", "Submitted", "Approved", "Submission Rate"}},
		{"Submission Compliance", 6, []string{"Bethel", "Ana", "Approved", "Pending", "2", "1", "100.0%"}},
		{"Submission Compliance", 7, []string{"Grace", "Ben", "No Report", "Needs Revision", "1", "0", "50.0%"}},
		// A month before the account existed is a dash, outside the rate.
		{"Submission Compliance", 8, []string{"Hope", "Dee", "—", "Pending", "1", "0", "100.0%"}},
		// Inactive accounts are left out.
		{"Submission Compliance", 9, []string{"", "", "", "", "", "", ""}},
	} {
		if got := row(c.sheet, c.row, len(c.want)); !slices.Equal(got, c.want) {
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

	if got, want := get(full, "A2"), "All Metrics · Full Year 2026 · Approved · 2 of 3 churches reporting"; got != want {
		t.Errorf("%s subtitle = %q, want %q", full, got, want)
	}
	if layout, err := f.GetPageLayout(full); err != nil || *layout.Orientation != "landscape" || *layout.FitToWidth != 1 {
		t.Errorf("%s page layout = %+v, %v", full, layout, err)
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

func TestDetailTable(t *testing.T) {
	data := stats.AttendanceAnalytics{Year: 2026, Detail: []stats.AttendanceDetail{
		{Metric: "sunday_school", ChurchName: "Bethel", Month: 1, Value: 5},
		{Metric: "worship_service", ChurchName: "Grace", Month: 2, Value: 30},
		{Metric: "worship_service", ChurchName: "Bethel", Month: 2, Value: 20},
		{Metric: "worship_service", ChurchName: "Bethel", Month: 1, Value: 10},
	}}
	got := detailTable(data)
	if !got.filter || !slices.Equal(got.headers, []string{"Metric", "Church", "Month", "2026"}) {
		t.Fatalf("headers = %v, filter = %v", got.headers, got.filter)
	}
	// Form metric order (worship before sunday school), then church, then month.
	want := [][]any{
		{"Worship Service", "Bethel", "January", 10.0},
		{"Worship Service", "Bethel", "February", 20.0},
		{"Worship Service", "Grace", "February", 30.0},
		{"Sunday School", "Bethel", "January", 5.0},
	}
	if fmt.Sprint(got.rows) != fmt.Sprint(want) {
		t.Errorf("rows = %v, want %v", got.rows, want)
	}

	data.Comparison = &stats.AttendanceComparison{Year: 2025, Detail: []stats.AttendanceDetail{
		{Metric: "worship_service", ChurchName: "Bethel", Month: 1, Value: 5},
		{Metric: "baptism", ChurchName: "Zion", Month: 3, Value: 2},
	}}
	got = detailTable(data)
	if !slices.Equal(got.headers, []string{"Metric", "Church", "Month", "2026", "2025", "Change"}) {
		t.Fatalf("compare headers = %v", got.headers)
	}
	if r := got.rows[0]; r[4] != 5.0 || r[5] != change(10, 5) {
		t.Errorf("Bethel January = %v, want 2025 = 5 and change(10, 5)", r)
	}
	// A row only last year still appears, with 0 this year.
	if r := got.rows[len(got.rows)-1]; fmt.Sprint(r[:5]) != fmt.Sprint([]any{"Baptism", "Zion", "March", 0.0, 2.0}) {
		t.Errorf("last row = %v", r)
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
