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
	for row, want := range map[int]float64{40: 56.25, 41: 29.25, 42: 45} {
		height, err := f.GetRowHeight(sh, row)
		require.NoError(t, err)
		assert.InDelta(t, want, height, 0.01, "row %d", row)
	}
}

// TestBuildWorkbook_LegacyStyles checks the metric sheet's look against the
// legacy workbook's styles (D-A17).
func TestBuildWorkbook_LegacyStyles(t *testing.T) {
	f := openWorkbook(t, &report.Report{ID: 1, Month: "May", Year: 2026, ChurchName: strPtr("Zion")})
	style := styleOf(t, f, "Zion")

	title := style("A1")
	assert.Equal(t, "Times New Roman", title.Font.Family)
	assert.InDelta(t, 18, title.Font.Size, 0.01)
	assert.Equal(t, "0000FF", title.Font.Color)
	assert.False(t, title.Font.Bold)

	for _, ref := range []string{"A8", "B8", "A29", "G29"} {
		assert.Equal(t, []string{"000000"}, style(ref).Fill.Color, ref)
	}
	assert.Equal(t, "FFFFFF", style("A8").Font.Color, "the band label stays readable")

	assert.Equal(t, "Helvetica Neue", style("A20").Font.Family)
	assert.InDelta(t, 12, style("A20").Font.Size, 0.01)
	assert.Equal(t, "Arial", style("A21").Font.Family, "sub-items are the smaller Arial")
	assert.InDelta(t, 10, style("A21").Font.Size, 0.01)
	assert.True(t, style("B9").Font.Bold)
	assert.False(t, style("B30").Font.Bold, "the second week header is not bold")

	borderSides := func(ref string) []string {
		var out []string
		for _, b := range style(ref).Border {
			out = append(out, b.Type)
		}
		return out
	}
	assert.ElementsMatch(t, []string{"right", "bottom"}, borderSides("B10"))
	assert.ElementsMatch(t, []string{"left", "right", "bottom"}, borderSides("A10"))
}

// styleOf returns a reader of a cell's style on sheet sh.
func styleOf(t *testing.T, f *excelize.File, sh string) func(ref string) *excelize.Style {
	return func(ref string) *excelize.Style {
		t.Helper()
		id, err := f.GetCellStyle(sh, ref)
		require.NoError(t, err)
		s, err := f.GetStyle(id)
		require.NoError(t, err)
		return s
	}
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
		{Church: "=HYPERLINK(\"http://x\")", Month: "May", Year: 2026, Status: report.StatusApproved},
		{Church: "Bethel", Month: "May", Year: 2026, Status: report.StatusPending},
		{Church: "Zion", Status: report.StatusNotFiled},
	}
	reps := []*report.Report{
		{ID: 1, Month: "May", Year: 2026, ChurchName: strPtr("Bethel")},
		{ID: 2, Month: "May", Year: 2026, ChurchName: strPtr("report dashboard")},
	}
	f, err := buildBulkWorkbook(rows, reps)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	assert.Equal(t, []string{"Report Dashboard", "Bethel", "report dashboard (2)"}, f.GetSheetList())

	const sh = "Report Dashboard"
	merges, err := f.GetMergeCells(sh)
	require.NoError(t, err)
	require.Len(t, merges, 1)
	assert.Equal(t, []string{"A1", "F1", "Report Dashboard"}, []string{merges[0].GetStartAxis(), merges[0].GetEndAxis(), merges[0].GetCellValue()})
	formula, err := f.GetCellFormula(sh, "A3")
	require.NoError(t, err)
	assert.Empty(t, formula, "a church name that looks like a formula stays text")
	year, err := f.GetCellType(sh, "C3")
	require.NoError(t, err)
	// A number cell has no t attribute, which excelize reports as unset; text is a shared string.
	assert.Contains(t, []excelize.CellType{excelize.CellTypeUnset, excelize.CellTypeNumber}, year, "the year is a number")

	layout, err := f.GetPageLayout(sh)
	require.NoError(t, err)
	require.NotNil(t, layout.Orientation)
	assert.Equal(t, "landscape", *layout.Orientation, "the six columns print on one page")

	style := styleOf(t, f, sh)
	title := style("A1")
	assert.True(t, title.Font.Bold)
	assert.Empty(t, title.Font.Underline)
	assert.Equal(t, "0000FF", title.Font.Color)
	assert.InDelta(t, 18, title.Font.Size, 0.01)
	for cell, want := range map[string][]string{
		"A1": {"DDDDFF"}, "F1": {"DDDDFF"}, "A2": {"E0E0E0"}, "F2": {"E0E0E0"},
		"D3": {"00AA00"}, // Approved
		"D4": {"FF0000"}, // Pending
		"D5": {"FF0000"}, // Not Filed
	} {
		assert.Equal(t, want, style(cell).Fill.Color, cell)
	}
	for _, cell := range []string{"A3", "B3", "C3", "E3", "F5"} {
		assert.Empty(t, style(cell).Fill.Color, "%s: no fill", cell)
		assert.Len(t, style(cell).Border, 4, "%s: thin borders on every side", cell)
	}
	assert.True(t, style("D2").Font.Bold)
	assert.Equal(t, "center", style("B3").Alignment.Horizontal)
	assert.False(t, style("B3").Font.Bold, "the month is regular")
	assert.True(t, style("C3").Font.Bold, "the year is bold")
	assert.Equal(t, "FFFFFF", style("D4").Font.Color)
	for _, cell := range []string{"E3", "F5"} {
		v, err := f.GetCellValue(sh, cell)
		require.NoError(t, err)
		assert.Empty(t, v, "%s: the remark cells stay blank", cell)
	}

	sheets, err := previewSheets(f, true)
	require.NoError(t, err)
	require.Len(t, sheets, 3)
	assert.Equal(t, [][]string{
		{"Report Dashboard", "", "", "", "", ""},
		{"Churches", "Month", "Year", "Status", "Coordinator's Remark", "Support Status"},
		{`=HYPERLINK("http://x")`, "May", "2026", "Approved", "", ""},
		{"Bethel", "May", "2026", "Pending", "", ""},
		{"Zion", "", "", "Not Filed", "", ""},
	}, sheets[0].Rows)
	assert.Len(t, sheets[1].Rows, 42)
	assert.Len(t, sheets[1].Rows[0], 7)

	t.Run("no reports and no accounts is the title and header alone", func(t *testing.T) {
		f, err := buildBulkWorkbook(nil, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = f.Close() })
		assert.Equal(t, []string{"Report Dashboard"}, f.GetSheetList())
		sheets, err := previewSheets(f, true)
		require.NoError(t, err)
		assert.Equal(t, [][]string{
			{"Report Dashboard", "", "", "", "", ""},
			{"Churches", "Month", "Year", "Status", "Coordinator's Remark", "Support Status"},
		}, sheets[0].Rows)
	})
}
