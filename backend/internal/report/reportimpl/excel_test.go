package reportimpl

import (
	"bytes"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/ssr0016/pastoral-reports/backend/internal/report"
)

func numeric(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	require.NoError(t, n.Scan(s))
	return n
}

func strPtr(s string) *string { return &s }

// openWorkbook builds the workbook, writes it to bytes and opens it again, so
// the assertions see what a spreadsheet application would read.
func openWorkbook(t *testing.T, reps ...*report.Report) *excelize.File {
	t.Helper()
	data, err := workbookBytes(reps)
	require.NoError(t, err)
	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// cell reads a raw value from the layout test's only sheet.
func cell(t *testing.T, f *excelize.File, ref string) string {
	t.Helper()
	v, err := f.GetCellValue(f.GetSheetName(0), ref, excelize.Options{RawCellValue: true})
	require.NoError(t, err)
	return v
}

func TestMetricRows_EveryKeyOnce(t *testing.T) {
	require.Len(t, metricRows, len(report.MetricKeys))
	rows := map[int]string{}
	for _, key := range report.MetricKeys {
		row, ok := metricRows[key]
		require.True(t, ok, "no row for %s", key)
		require.NotContains(t, rows, row, "row %d holds %s and %s", row, rows[row], key)
		rows[row] = key
		assert.Contains(t, rowLabels, row, "row %d has no label", row)
	}
	assert.Equal(t, "Leadership Training", rowLabels[22])
	assert.NotContains(t, rows, 22)
}

func TestBuildWorkbook_Layout(t *testing.T) {
	metrics := map[string]map[int]pgtype.Numeric{}
	for i, key := range report.MetricKeys {
		metrics[key] = map[int]pgtype.Numeric{1: numeric(t, strconv.Itoa(i+1))}
	}
	metrics["worship_service"] = map[int]pgtype.Numeric{1: numeric(t, "59"), 2: numeric(t, "0"), 4: numeric(t, "100")}
	metrics["tithes_offering"] = map[int]pgtype.Numeric{1: numeric(t, "1810.50")}
	rep := &report.Report{
		ID: 7, Month: "September", Year: 2026,
		Worker: strPtr("Ptr. Ana"), AreaAssignment: strPtr("Leyte"), ChurchName: strPtr("Gospel Bearer"),
		Names: strPtr("Juan, Maria"), NarrativeReport: strPtr("=HYPERLINK(\"http://x\")"),
		Challenges: strPtr("Rain"), PrayerRequest: strPtr("Health"),
		Metrics: metrics,
	}
	f := openWorkbook(t, rep)
	sh := "Gospel Bearer"
	require.Equal(t, []string{sh}, f.GetSheetList())

	for ref, want := range map[string]string{
		"A1": "ANG MANANAMPALATAYANG GUMAWA", "A3": "NATIONAL WORKERS' MONTHLY MINISTRY REPORT",
		"A4": "Month of:", "A5": "Worker", "B5": "Ptr. Ana", "A6": "Area of Assignment", "B6": "Leyte",
		"A7": "Name of Church/Outreach:", "B7": "Gospel Bearer", "A8": "Weekly Attendance",
		"A9": "Activities", "B9": "Week 1", "F9": "Week 5", "G9": "Average",
		"A19": "Baptism", "A21": "Leadership Conference", "A22": "Leadership Training",
		"A29": "Other Statistics", "B30": "Week 1", "G30": "Average", "A36": "Person Led To Christ",
		"A37": "Names:", "B37": "Juan, Maria", "A40": "Narrative Report:", "B41": "Rain", "B42": "Health",
	} {
		assert.Equal(t, want, cell(t, f, ref), ref)
	}

	// Every metric lands on its row; blank weeks stay blank and 0 stays 0 (D-5).
	for key, row := range metricRows {
		if key == "worship_service" || key == "tithes_offering" {
			continue
		}
		want := strconv.Itoa(slices.Index(report.MetricKeys, key) + 1)
		assert.Equal(t, want, cell(t, f, "B"+strconv.Itoa(row)), key)
	}
	assert.Equal(t, "59", cell(t, f, "B10"))
	assert.Equal(t, "0", cell(t, f, "C10"))
	assert.Equal(t, "", cell(t, f, "D10"))
	assert.Equal(t, "100", cell(t, f, "E10"))
	assert.Equal(t, "", cell(t, f, "B22"))
	assert.Equal(t, "1810.5", cell(t, f, "B27"))
	money, err := f.GetCellStyle(sh, "B27")
	require.NoError(t, err)
	st, err := f.GetStyle(money)
	require.NoError(t, err)
	assert.Equal(t, 4, st.NumFmt)

	// Average formulas on every labelled metric row.
	for row := range rowLabels {
		n := strconv.Itoa(row)
		got, err := f.GetCellFormula(sh, "G"+n)
		require.NoError(t, err)
		assert.Equal(t, `IFERROR(AVERAGE(B`+n+`:F`+n+`),"")`, got)
	}

	// Text that looks like a formula stays text.
	got, err := f.GetCellFormula(sh, "B40")
	require.NoError(t, err)
	assert.Empty(t, got)
	typ, err := f.GetCellType(sh, "B40")
	require.NoError(t, err)
	assert.NotEqual(t, excelize.CellTypeFormula, typ)
	assert.Equal(t, `=HYPERLINK("http://x")`, cell(t, f, "B40"))

	// Month of: a date shown as "September 2026".
	shown, err := f.GetCellValue(sh, "B4")
	require.NoError(t, err)
	assert.Equal(t, "September 2026", shown)

	merges, err := f.GetMergeCells(sh)
	require.NoError(t, err)
	var refs []string
	for _, m := range merges {
		refs = append(refs, m.GetStartAxis()+":"+m.GetEndAxis())
	}
	assert.ElementsMatch(t, []string{"A1:G2", "A3:G3", "B4:G4", "B5:G5", "B6:G6", "B7:G7", "B37:G39", "B40:G40", "B41:G41", "B42:G42"}, refs)

	wrap, err := f.GetCellStyle(sh, "B40")
	require.NoError(t, err)
	ws, err := f.GetStyle(wrap)
	require.NoError(t, err)
	require.NotNil(t, ws.Alignment)
	assert.True(t, ws.Alignment.WrapText)

	width, err := f.GetColWidth(sh, "A")
	require.NoError(t, err)
	assert.InDelta(t, 26, width, 0.01)
	height, err := f.GetRowHeight(sh, 40)
	require.NoError(t, err)
	assert.InDelta(t, 43.5, height, 0.01)
}

func TestSheetNames(t *testing.T) {
	long := strings.Repeat("x", 40)
	reps := []*report.Report{
		{ID: 1, ChurchName: strPtr("San Isidro")},
		{ID: 2, ChurchName: strPtr("san isidro")},
		{ID: 3, Worker: strPtr("Ptr. Ana")},
		{ID: 4},
		{ID: 5, ChurchName: strPtr("A/B: [C]*?\\")},
		{ID: 6, ChurchName: strPtr(long)},
		{ID: 7, ChurchName: strPtr(long)},
		{ID: 8, ChurchName: strPtr("  ")},
		{ID: 9, ChurchName: strPtr(strings.Repeat("🙏", 20))},
		{ID: 10, ChurchName: strPtr(strings.Repeat("🙏", 20))},
		{ID: 11, ChurchName: strPtr(strings.Repeat("a", 30) + "'b")},
		{ID: 12, ChurchName: strPtr("'''")},
		{ID: 13, ChurchName: strPtr(strings.Repeat("c", 26) + "'dddd")},
		{ID: 14, ChurchName: strPtr(strings.Repeat("c", 26) + "'dddd")},
	}
	got := sheetNames(nil, reps)
	assert.Equal(t, []string{
		"San Isidro", "san isidro (2)", "Ptr. Ana", "Report 4", "A-B- -C----",
		strings.Repeat("x", 31), strings.Repeat("x", 27) + " (2)", "Report 8",
		// Emoji are two UTF-16 units each; a cut never leaves a trailing '.
		strings.Repeat("🙏", 15), strings.Repeat("🙏", 13) + " (2)",
		strings.Repeat("a", 30), "Report 12",
		strings.Repeat("c", 26) + "'dddd", strings.Repeat("c", 26) + " (2)",
	}, got)
	for _, n := range got {
		assert.LessOrEqual(t, len(utf16.Encode([]rune(n))), 31)
	}

	// A workbook with all of them opens (Excel rejects bad or duplicate names).
	for _, r := range reps {
		r.Month, r.Year = "May", 2026
	}
	f := openWorkbook(t, reps...)
	assert.Equal(t, got, f.GetSheetList())
}

func TestExportFilename(t *testing.T) {
	assert.Equal(t, "Gospel Bearer September 2026.xlsx", exportFilename("Gospel Bearer", "September", 2026))
	assert.Equal(t, "ab May 2026.xlsx", exportFilename("a/b\\\n", "May", 2026))
}

func TestPreviewSheets(t *testing.T) {
	rep := &report.Report{
		ID: 7, Month: "September", Year: 2026,
		Worker: strPtr("Ptr. Ana"), ChurchName: strPtr("Gospel Bearer"),
		NarrativeReport: strPtr("=HYPERLINK(\"http://x\")"),
		Metrics: map[string]map[int]pgtype.Numeric{
			"worship_service": {1: numeric(t, "59"), 2: numeric(t, "0"), 4: numeric(t, "100")},
			"tithes_offering": {1: numeric(t, "1810.50"), 2: numeric(t, "200")},
		},
	}
	other := &report.Report{ID: 8, Month: "September", Year: 2026, ChurchName: strPtr("Abba")}
	f, err := buildWorkbook([]*report.Report{rep, other})
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	sheets, err := previewSheets(f, false)
	require.NoError(t, err)
	require.Len(t, sheets, 2)
	assert.Equal(t, "Gospel Bearer", sheets[0].Name)
	assert.Equal(t, "Abba", sheets[1].Name)
	for _, s := range sheets {
		require.Len(t, s.Rows, 42)
		for _, row := range s.Rows {
			require.Len(t, row, 7)
		}
	}

	at := func(ref string) string {
		col, row, err := excelize.CellNameToCoordinates(ref)
		require.NoError(t, err)
		return sheets[0].Rows[row-1][col-1]
	}
	for ref, want := range map[string]string{
		"A1": "ANG MANANAMPALATAYANG GUMAWA", "B1": "", "B4": "September 2026", "C4": "",
		"B5": "Ptr. Ana", "C5": "", "G9": "Average",
		"B10": "59", "C10": "0", "D10": "", "E10": "100", "G10": "53",
		"B11": "", "G11": "", // a row with no weeks has no average
		"B27": "1,810.50", "C27": "200.00", "G27": "1,005.25",
		"B40": `=HYPERLINK("http://x")`,
	} {
		assert.Equal(t, want, at(ref), ref)
	}
}

func TestBuildBulkWorkbook_Dashboard(t *testing.T) {
	rows := []report.DashboardRow{
		{Church: "=HYPERLINK(\"http://x\")", Worker: "Ana", Area: "Leyte", Status: report.StatusApproved},
		{Church: "Bethel", Worker: "Ben", Status: report.StatusPending},
		{Church: "Zion", Worker: "Cora", Area: "Cebu", Status: report.StatusNotFiled},
	}
	reps := []*report.Report{
		{ID: 1, Month: "May", Year: 2026, ChurchName: strPtr("Bethel")},
		{ID: 2, Month: "May", Year: 2026, ChurchName: strPtr("report dashboard")},
	}
	f, err := buildBulkWorkbook(rows, reps)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	assert.Equal(t, []string{"Report Dashboard", "Bethel", "report dashboard (2)"}, f.GetSheetList())
	got, err := f.GetRows("Report Dashboard")
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"Church", "Worker", "Area", "Status"},
		{`=HYPERLINK("http://x")`, "Ana", "Leyte", "Approved"},
		{"Bethel", "Ben", "", "Pending"},
		{"Zion", "Cora", "Cebu", "Not Filed"},
	}, got)
	formula, err := f.GetCellFormula("Report Dashboard", "A2")
	require.NoError(t, err)
	assert.Empty(t, formula, "a church name that looks like a formula stays text")
	head, err := f.GetCellStyle("Report Dashboard", "A1")
	require.NoError(t, err)
	hs, err := f.GetStyle(head)
	require.NoError(t, err)
	assert.True(t, hs.Font.Bold)

	sheets, err := previewSheets(f, true)
	require.NoError(t, err)
	require.Len(t, sheets, 3)
	assert.Equal(t, got[:2], sheets[0].Rows[:2])
	assert.Equal(t, []string{"Bethel", "Ben", "", "Pending"}, sheets[0].Rows[2], "a blank area is padded")
	assert.Len(t, sheets[0].Rows, 4)
	assert.Len(t, sheets[1].Rows, 42)
	assert.Len(t, sheets[1].Rows[0], 7)

	t.Run("no reports and no accounts is the header alone", func(t *testing.T) {
		f, err := buildBulkWorkbook(nil, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = f.Close() })
		assert.Equal(t, []string{"Report Dashboard"}, f.GetSheetList())
		sheets, err := previewSheets(f, true)
		require.NoError(t, err)
		assert.Equal(t, [][]string{{"Church", "Worker", "Area", "Status"}}, sheets[0].Rows)
	})
}
