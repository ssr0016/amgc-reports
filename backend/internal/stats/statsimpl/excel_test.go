package statsimpl

import (
	"bytes"
	"slices"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
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
			{Metric: "worship_service", Total: &ten, Churches: []stats.AttendanceSummaryChurch{{ChurchName: "Bethel", Value: &ten}, {ChurchName: "Grace"}}},
		},
		Comparison: &stats.AttendanceComparison{Year: 2025,
			Trend:    []stats.AttendanceTrendPoint{{Month: "January", Week: 1, Value: 1000}},
			Churches: []stats.AttendanceChurch{{ChurchName: "Zion", Value: 1000}},
		},
	}
	loc, _ := time.LoadLocation("Asia/Manila")
	raw, err := buildAnalyticsWorkbook(data, 24, time.Date(2026, 10, 28, 14, 5, 0, 0, loc))
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	if got, want := f.GetSheetList(), []string{"Summary", "Weekly Trend", "Church Breakdown", "All Metrics"}; !slices.Equal(got, want) {
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
	// A church with no value for a metric gets a blank cell, not 0.
	if got := []string{get("All Metrics", "A6"), get("All Metrics", "C6"), get("All Metrics", "D6")}; !slices.Equal(got, []string{"Worship Service", "10.00", ""}) {
		t.Errorf("All Metrics row = %v", got)
	}

	panes, err := f.GetPanes("All Metrics")
	if err != nil || !panes.Freeze || panes.YSplit != headerRow || panes.XSplit != 1 {
		t.Errorf("All Metrics panes = %+v, %v", panes, err)
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
	raw, err := buildAnalyticsWorkbook(stats.AttendanceAnalytics{Year: 2026, Metric: "baptism", Status: stats.AttendanceStatusAll, Period: "q3"}, 0, time.Now())
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
	if v, _ := f.GetCellValue("Summary", "A2"); v != "Baptism · Q3 2026 · All Submitted" {
		t.Errorf("subtitle = %q", v)
	}
}
