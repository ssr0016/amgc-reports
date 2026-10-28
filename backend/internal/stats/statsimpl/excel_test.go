package statsimpl

import (
	"bytes"
	"encoding/csv"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
)

func TestMetricLabels_CoverEveryMetric(t *testing.T) {
	for _, k := range report.MetricKeys {
		assert.Contains(t, stats.MetricLabels, k)
	}
	assert.Len(t, stats.MetricLabels, len(report.MetricKeys))
	assert.Equal(t, "Legacy Persons Evangelized — meaning unverified", stats.MetricLabels[report.LegacyMetric])
}

// Every metric sits under exactly one analytics group.
func TestMetricGroups(t *testing.T) {
	var grouped []string
	for _, g := range stats.MetricGroups {
		grouped = append(grouped, g.Metrics...)
	}
	assert.Equal(t, slices.Sorted(slices.Values(report.MetricKeys)), slices.Sorted(slices.Values(grouped)))
}

func openWorkbook(t *testing.T, raw []byte) *excelize.File {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// sheetTable reads a sheet's header (row 5) and rows as raw values.
func sheetTable(t *testing.T, f *excelize.File, sheet string) ([]string, [][]string) {
	t.Helper()
	rows, err := f.GetRows(sheet, excelize.Options{RawCellValue: true})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rows), headerRow)
	return rows[headerRow-1], rows[headerRow:]
}

func col(t *testing.T, header []string, name string) int {
	t.Helper()
	i := slices.Index(header, name)
	require.GreaterOrEqual(t, i, 0, "no column %q in %v", name, header)
	return i
}

func at(row []string, i int) string {
	if i < len(row) {
		return row[i]
	}
	return ""
}

func TestBuildAnalyticsWorkbook_ReconcilesWithAnalysisAndCSV(t *testing.T) {
	churches, reports := complete()
	// A pending report under the Approved filter, and a blank metric.
	early := rep(9001, 1, 2026, 10, "pending", map[string]*[5]*float64{"worship_service": weeks(f64(5))})
	early.CreatedAt = time.Date(2026, 10, 20, 9, 0, 0, 0, manila)
	reports = append(reports, early)
	reports[0].Metrics["sunday_school"] = weeks(nil, f64(0))
	q := query("worship_service")
	q.Compare = true
	data := analyze(analysis{q: q, now: testNow, churches: churches, reports: reports})
	raw, err := buildAnalyticsWorkbook(data)
	require.NoError(t, err)
	f := openWorkbook(t, raw)

	assert.Equal(t, []string{"Full Metrics Summary", "Summary", "Monthly Totals", "Weekly Trend", "Church Breakdown",
		"Metric Detail", "Weekly Detail", "Submission Compliance", "Notes"}, f.GetSheetList())
	for _, sheet := range f.GetSheetList() {
		title, _ := f.GetCellValue(sheet, "A1")
		assert.True(t, strings.HasPrefix(title, "AMGCReports — "+sheet), "%s title %q", sheet, title)
		scope, _ := f.GetCellValue(sheet, "A2")
		assert.Equal(t, "Full Year 2026 · All 24 churches · Approved reports only", scope)
		cutoff, _ := f.GetCellValue(sheet, "A3")
		assert.Contains(t, cutoff, "(Asia/Manila)")
		assert.Contains(t, cutoff, "January–September 2026 (year to date")
	}

	// Full Metrics Summary: worship total and every church kept, as numbers.
	header, rows := sheetTable(t, f, "Full Metrics Summary")
	assert.Len(t, header, 7+24)
	w := metricSummary(t, data, "worship_service")
	assert.Equal(t, "43200", at(rows[0], col(t, header, "Total for Period")))
	assert.Equal(t, strconv.Itoa(w.Observations), at(rows[0], col(t, header, "Recorded Entries")))
	assert.Equal(t, "50", at(rows[0], col(t, header, "Avg per Church-Service")))
	assert.Equal(t, "1200", at(rows[0], col(t, header, "Network Weekly Avg")))
	typ, err := f.GetCellType("Full Metrics Summary", "D6")
	require.NoError(t, err)
	assert.NotEqual(t, excelize.CellTypeSharedString, typ, "totals are numbers, not text")

	// Weekly Detail: the included worship rows add up to the same total, unrounded.
	header, rows = sheetTable(t, f, "Weekly Detail")
	var sum float64
	var n int
	for _, r := range rows {
		if at(r, col(t, header, "Metric Code")) == "worship_service" && at(r, col(t, header, "Included in Selected Analytics")) == "true" && at(r, col(t, header, "Value State")) == "reported" {
			v, err := strconv.ParseFloat(at(r, col(t, header, "Value")), 64)
			require.NoError(t, err)
			sum += v
			n++
		}
	}
	assert.InDelta(t, *w.Total, sum, 1e-9)
	assert.Equal(t, w.Observations, n)

	// The CSV gives the same rows and total.
	out, err := analyticsCSV(data, stats.CSVWeeklyDetail)
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(out, []byte("\uFEFF")), "byte-order mark for Excel")
	records, err := csv.NewReader(bytes.NewReader(out[3:])).ReadAll()
	require.NoError(t, err)
	assert.Equal(t, weeklyDetailHeaders, records[0])
	sum, n = 0, 0
	blankZero := 0
	for _, r := range records[1:] {
		if r[9] == "worship_service" && r[15] == "true" && r[14] == "reported" {
			v, _ := strconv.ParseFloat(r[13], 64)
			sum += v
			n++
		}
		if r[9] == "sunday_school" && r[6] == "1" {
			switch r[5] {
			case "1":
				assert.Equal(t, []string{"", "blank"}, r[13:15], "a blank week is empty with its state")
			case "2":
				assert.Equal(t, []string{"0", "reported"}, r[13:15], "an explicit zero stays 0")
				blankZero++
			}
		}
		if r[6] == "9001" {
			assert.Equal(t, "false", r[15], "the pending report is listed but not counted")
		}
	}
	assert.InDelta(t, *w.Total, sum, 1e-9)
	assert.Equal(t, w.Observations, n)
	assert.Equal(t, 1, blankZero)

	// Compliance: the month matrix and its legend colours.
	header, rows = sheetTable(t, f, "Submission Compliance")
	assert.Equal(t, "Approved · on time", at(rows[0], col(t, header, "Sep")))
	assert.Equal(t, "Pending · before month end", at(rows[0], col(t, header, "Oct")), "an admin entry before the month ended")
	assert.Equal(t, "Not Yet Open", at(rows[1], col(t, header, "Oct")))

	// Summary: the comparison has no data, so changes are n/a, never 0% or infinite.
	header, rows = sheetTable(t, f, "Summary")
	assert.Equal(t, "n/a", at(rows[1], col(t, header, "Change")))
}

func TestBuildAnalyticsWorkbook_NoReportAndPendingCells(t *testing.T) {
	churches := []stats.ChurchRecord{churchRec(1, month(2026, 1), nil, "active"), churchRec(2, month(2026, 1), nil, "active"), churchRec(3, month(2026, 1), nil, "active")}
	reports := []stats.ReportData{
		rep(1, 1, 2026, 1, "approved", map[string]*[5]*float64{"tithes_offering": weeks(f64(0))}),
		rep(2, 2, 2026, 1, "pending", map[string]*[5]*float64{"worship_service": weeks(f64(9))}),
	}
	data := analyze(analysis{q: query("worship_service"), now: testNow, churches: churches, reports: reports})
	raw, err := buildAnalyticsWorkbook(data)
	require.NoError(t, err)
	f := openWorkbook(t, raw)
	header, rows := sheetTable(t, f, "Full Metrics Summary")
	worship := rows[0]
	assert.Equal(t, "—", at(worship, col(t, header, "Church 01")), "reported, left blank")
	assert.Equal(t, "Not Approved", at(worship, col(t, header, "Church 02")))
	assert.Equal(t, "No Report", at(worship, col(t, header, "Church 03")))
	assert.Equal(t, "", at(worship, col(t, header, "Total for Period")), "no value is a blank, not 0")

	// Church Breakdown matches the page's church rows; submission completeness counts the
	// pending report, approval does not.
	header, rows = sheetTable(t, f, "Church Breakdown")
	require.Len(t, rows, len(data.ChurchRows))
	for i, r := range data.ChurchRows {
		assert.Equal(t, strconv.Itoa(r.Submitted), at(rows[i], col(t, header, "Submitted")))
		assert.Equal(t, strconv.Itoa(r.Approved), at(rows[i], col(t, header, "Approved")))
		require.NotNil(t, r.Completeness)
		assert.Equal(t, strconv.FormatFloat(*r.Completeness, 'f', -1, 64), at(rows[i], col(t, header, "Submission Completeness")))
	}
	assert.InDelta(t, 1.0/9, *data.ChurchRows[1].Completeness, 1e-9, "Church 02 submitted 1 of 9 owed months")
	assert.Equal(t, "0", at(rows[1], col(t, header, "Approved")), "but nothing is approved yet")
}

func TestBuildAnalyticsWorkbook_EmptyPeriod(t *testing.T) {
	q := query("baptism")
	q.Year = 2027
	raw, err := buildAnalyticsWorkbook(analyze(analysis{q: q, now: testNow}))
	require.NoError(t, err)
	f := openWorkbook(t, raw)
	v, _ := f.GetCellValue("Metric Detail", "A6")
	assert.Equal(t, "No data for this period.", v)
	v, _ = f.GetCellValue("Summary", "A3")
	assert.Contains(t, v, "No month of this period has ended")
}

func TestBuildCSV_EscapesAndGuardsFormulas(t *testing.T) {
	out, err := buildCSV([]string{"Text", "Number", "Missing"}, [][]any{
		{"Bethel, \"North\"\nchapel", 0.0, nil},
		{"=HYPERLINK(\"x\")", -12.5, nil},
		{"+63 912", 1234567.25, nil},
		{"@cmd", 7, nil},
		{"-dash", true, nil},
	})
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(out, []byte("\uFEFF")))
	assert.Contains(t, string(out), "\r\n", "CRLF rows")
	records, err := csv.NewReader(bytes.NewReader(out[3:])).ReadAll()
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"Text", "Number", "Missing"},
		{"Bethel, \"North\"\nchapel", "0", ""},
		{"'=HYPERLINK(\"x\")", "-12.5", ""},
		{"'+63 912", "1234567.25", ""},
		{"'@cmd", "7", ""},
		{"'-dash", "true", ""},
	}, records, "numbers stay plain numbers; text that starts like a formula is guarded")
}

func TestExportFilename(t *testing.T) {
	q := stats.AttendanceQuery{Year: 2026, Period: "full_year", Status: stats.AttendanceStatusAll, ChurchIDs: []int64{1, 2}}
	assert.Equal(t, "amgc-compliance_2026_full-year_all-submitted_2-churches.csv", stats.ExportFilename(q, stats.CSVCompliance, "csv"))
}
