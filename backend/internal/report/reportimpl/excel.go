package reportimpl

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/xuri/excelize/v2"

	"github.com/ssr0016/pastoral-reports/backend/internal/report"
)

// The legacy "Church Growth Program Report" sheet layout (D-A6). Cell positions,
// labels, merges, formats and formulas follow the template exactly.

// metricRows is the template row of each of the 21 metrics.
var metricRows = map[string]int{
	"worship_service": 10, "sunday_school": 11, "prayer_meeting": 12, "bible_studies": 13,
	"mens_fellowship": 14, "womens_fellowship": 15, "youth_fellowship": 16, "children_fellowship": 17,
	"outreach": 18, "baptism": 19, "training": 20, "leadership": 21,
	"other": 24, "family_day": 25, "tithes_offering": 27,
	"home_visited": 31, "bible_study_group_led": 32, "sermon_preached": 33,
	"person_newly_contacted": 34, "person_followed_up": 35, "person_evangelized": 36,
}

// rowLabels are the column A labels of the metric rows. Row 22 has a label but
// no metric: there is no separate Leadership Training value.
var rowLabels = map[int]string{
	10: "Worship Service", 11: "Sunday School", 12: "Prayer Meeting", 13: "Bible Studies",
	14: "Mens Fellowships", 15: "Womens Fellowships", 16: "Youth Fellowships", 17: "Children Fellowships",
	18: "Outreach", 19: "Baptism", 20: "Training/ Seminars", 21: "Leadership Conference",
	22: "Leadership Training", 24: "Other", 25: "Family Day", 27: "Tithes & Offering",
	31: "Home Visited", 32: "Bible Study Group Led", 33: "Sermon/Message Preached",
	34: "Person Newly Contacted", 35: "Person Followed-up", 36: "Person Led To Christ",
}

var weekCols = [5]string{"B", "C", "D", "E", "F"}

// excelStyles are the legacy workbook's cell styles (D-A17), read from its
// styles.xml: Helvetica Neue 12 for labels, Arial 10 for values, borders on
// the right and bottom of each cell, and a left border on column A.
type excelStyles struct {
	// Metric sheet.
	title, subtitle, label, month, value, bandLabel, band, headerLabel, header,
	weekHeader, mainLabel, subLabel, cell, money, names, textLabel, text int
	// Report Dashboard.
	dashTitle, dashHeader, dashChurch, dashMonth, dashYear, dashApproved, dashOpen, dashRemark, dashRemarkSet int
	// Approved Reports: [flatText|flatCount|flatMoney][odd row], then the bold total row.
	flatHeader int
	flat       [3][2]int
	flatTotal  [3]int
}

// sides returns thin black borders on the named sides: l, r, t and b.
func sides(names string) []excelize.Border {
	types := map[rune]string{'l': "left", 'r': "right", 't': "top", 'b': "bottom"}
	out := make([]excelize.Border, 0, len(names))
	for _, n := range names {
		out = append(out, excelize.Border{Type: types[n], Color: "000000", Style: 1})
	}
	return out
}

func fill(color string) excelize.Fill {
	return excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{color}}
}

func newExcelStyles(f *excelize.File) (excelStyles, error) {
	helv := func(bold bool, color string) *excelize.Font {
		return &excelize.Font{Family: "Helvetica Neue", Size: 12, Bold: bold, Color: color}
	}
	arial := func(bold bool, color string) *excelize.Font {
		return &excelize.Font{Family: "Arial", Size: 10, Bold: bold, Color: color}
	}
	white, black := fill("FFFFFF"), fill("000000")
	top, bottom := &excelize.Alignment{Vertical: "top"}, &excelize.Alignment{Vertical: "bottom"}
	wrap := &excelize.Alignment{Vertical: "top", WrapText: true}
	center := &excelize.Alignment{Horizontal: "center", Vertical: "center"}
	monthFmt, moneyFmt, countFmt := "mmmm yyyy", 4, 3 // 4 is the built-in #,##0.00, 3 is #,##0

	var st excelStyles
	for _, s := range []struct {
		id    *int
		style *excelize.Style
	}{
		{&st.title, &excelize.Style{Font: &excelize.Font{Family: "Times New Roman", Size: 18, Color: "0000FF"}, Fill: white, Border: sides("rb"), Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "bottom"}}},
		{&st.subtitle, &excelize.Style{Font: helv(true, ""), Fill: white, Border: sides("lrb"), Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "bottom"}}},
		{&st.label, &excelize.Style{Font: helv(false, ""), Fill: white, Border: sides("lrb"), Alignment: bottom}},
		{&st.month, &excelize.Style{Font: arial(true, ""), Fill: white, Border: sides("lrtb"), CustomNumFmt: &monthFmt, Alignment: &excelize.Alignment{Horizontal: "left"}}},
		{&st.value, &excelize.Style{Font: arial(false, ""), Fill: white, Border: sides("lrtb")}},
		{&st.bandLabel, &excelize.Style{Font: helv(true, "FFFFFF"), Fill: black, Border: sides("lrb"), Alignment: bottom}},
		{&st.band, &excelize.Style{Fill: black, Border: sides("rb")}},
		{&st.headerLabel, &excelize.Style{Font: helv(true, ""), Border: sides("lrb"), Alignment: top}},
		{&st.header, &excelize.Style{Font: helv(true, ""), Border: sides("rb"), Alignment: top}},
		{&st.weekHeader, &excelize.Style{Font: helv(false, ""), Border: sides("rb"), Alignment: top}},
		{&st.mainLabel, &excelize.Style{Font: helv(false, ""), Border: sides("lrb"), Alignment: top}},
		{&st.subLabel, &excelize.Style{Font: arial(false, ""), Border: sides("lrb"), Alignment: bottom}},
		{&st.cell, &excelize.Style{Font: arial(false, ""), Border: sides("rb"), Alignment: bottom}},
		{&st.money, &excelize.Style{Font: arial(false, ""), Border: sides("rb"), Alignment: bottom, NumFmt: moneyFmt}},
		{&st.names, &excelize.Style{Font: arial(false, ""), Fill: white, Border: sides("rb"), Alignment: wrap}},
		{&st.textLabel, &excelize.Style{Font: helv(false, "000000"), Fill: white, Border: sides("lrb"), Alignment: wrap}},
		{&st.text, &excelize.Style{Font: helv(false, "000000"), Fill: white, Border: sides("rb"), Alignment: wrap}},
		{&st.dashTitle, &excelize.Style{Font: &excelize.Font{Family: "Arial", Size: 18, Bold: true, Color: "0000FF"}, Fill: fill("DDDDFF"), Alignment: center}},
		{&st.dashHeader, &excelize.Style{Font: arial(true, ""), Fill: fill("E0E0E0"), Border: sides("lrtb"), Alignment: center}},
		{&st.dashChurch, &excelize.Style{Font: arial(false, ""), Border: sides("lrtb")}},
		{&st.dashMonth, &excelize.Style{Font: arial(false, ""), Border: sides("lrtb"), Alignment: center}},
		{&st.dashYear, &excelize.Style{Font: arial(true, ""), Border: sides("lrtb"), Alignment: center}},
		{&st.dashApproved, &excelize.Style{Font: arial(true, "FFFFFF"), Fill: fill("00AA00"), Border: sides("lrtb"), Alignment: center}},
		{&st.dashOpen, &excelize.Style{Font: arial(true, "FFFFFF"), Fill: fill("FF0000"), Border: sides("lrtb"), Alignment: center}},
		{&st.dashRemark, &excelize.Style{Font: arial(false, ""), Border: sides("lrtb")}},
		{&st.dashRemarkSet, &excelize.Style{Font: arial(true, "FFFFFF"), Fill: fill("0000CC"), Border: sides("lrtb"), Alignment: center}},
	} {
		id, err := f.NewStyle(s.style)
		if err != nil {
			return excelStyles{}, fmt.Errorf("new style: %w", err)
		}
		*s.id = id
	}
	return st, newFlatStyles(f, &st, countFmt, moneyFmt)
}

// newFlatStyles adds the Approved Reports styles: the dashboard's header with
// wrapping, zebra rows with thin borders, and a bold total row.
func newFlatStyles(f *excelize.File, st *excelStyles, countFmt, moneyFmt int) error {
	header, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Family: "Arial", Size: 10, Bold: true}, Fill: fill("E0E0E0"), Border: sides("lrtb"),
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	if err != nil {
		return fmt.Errorf("new style: %w", err)
	}
	st.flatHeader = header
	numFmts := [3]int{flatCount: countFmt, flatMoney: moneyFmt}
	for kind := range st.flat {
		for odd := range 3 { // 0 and 1 are the zebra rows, 2 the total row
			s := &excelize.Style{Font: &excelize.Font{Family: "Arial", Size: 10, Bold: odd == 2}, Border: sides("lrtb"), NumFmt: numFmts[kind]}
			switch odd {
			case 1:
				s.Fill = fill("F2F2F2")
			case 2:
				s.Fill = fill("E0E0E0")
			}
			id, err := f.NewStyle(s)
			if err != nil {
				return fmt.Errorf("new style: %w", err)
			}
			if odd == 2 {
				st.flatTotal[kind] = id
			} else {
				st.flat[kind][odd] = id
			}
		}
	}
	return nil
}

// dashboardSheet is the bulk export's first sheet (D-A16); approvedSheet its
// second, when the month has an approved report.
const (
	dashboardSheet = "Report Dashboard"
	approvedSheet  = "Approved Reports"
)

// buildWorkbook renders one sheet per report, in order, into a new workbook.
// Metrics must be loaded. Every value is written as a string or a number, never
// as a formula, so report text that starts with "=" stays text.
func buildWorkbook(reps []*report.Report) (*excelize.File, error) {
	return renderWorkbook(nil, reps)
}

// buildBulkWorkbook is the bulk export: the Report Dashboard sheet, then one
// sheet per report (D-A16). reps may be empty.
func buildBulkWorkbook(rows []report.DashboardRow, reps []*report.Report) (*excelize.File, error) {
	if rows == nil {
		rows = []report.DashboardRow{}
	}
	return renderWorkbook(rows, reps)
}

// renderWorkbook writes the dashboard first when rows is non-nil, then Approved
// Reports when there are reports, then the report sheets, whose names never
// reuse those two.
func renderWorkbook(rows []report.DashboardRow, reps []*report.Report) (*excelize.File, error) {
	f := excelize.NewFile()
	if err := f.SetDefaultFont("Arial"); err != nil {
		return nil, fmt.Errorf("default font: %w", err)
	}
	st, err := newExcelStyles(f)
	if err != nil {
		return nil, err
	}
	var reserved []string
	if rows != nil {
		if err := f.SetSheetName("Sheet1", dashboardSheet); err != nil {
			return nil, fmt.Errorf("add sheet: %w", err)
		}
		if err := writeDashboard(f, rows, st); err != nil {
			return nil, fmt.Errorf("write sheet %q: %w", dashboardSheet, err)
		}
		reserved = []string{dashboardSheet, approvedSheet}
		if len(reps) > 0 {
			if _, err := f.NewSheet(approvedSheet); err != nil {
				return nil, fmt.Errorf("add sheet: %w", err)
			}
			if err := writeApproved(f, reps, st); err != nil {
				return nil, fmt.Errorf("write sheet %q: %w", approvedSheet, err)
			}
		}
	}
	for i, name := range sheetNames(reserved, reps) {
		if i == 0 && rows == nil {
			err = f.SetSheetName("Sheet1", name)
		} else {
			_, err = f.NewSheet(name)
		}
		if err != nil {
			return nil, fmt.Errorf("add sheet: %w", err)
		}
		if err := writeSheet(f, name, reps[i], st); err != nil {
			return nil, fmt.Errorf("write sheet %q: %w", name, err)
		}
	}
	return f, nil
}

// dashboardHeader is the Report Dashboard's header row, under its merged title.
// The remark columns are filled for approved reports only (D-A19).
var dashboardHeader = []string{"Churches", "Month", "Year", "Status", "Coordinator's Remark", "Support Status"}

// writeDashboard writes the owner's reference layout (D-A19): a title merged
// over A1:F1, the header in row 2, then one bordered row per church account.
// Approved is green and Pending or No Report Submitted red. Month and Year are the
// report's and blank when the church has not filed; the year is a number,
// everything else text. Approved rows read "Checked" and "Ready for Pick Up"
// on blue in the remark columns; other rows leave them bordered but blank.
func writeDashboard(f *excelize.File, rows []report.DashboardRow, st excelStyles) error {
	sh := dashboardSheet
	steps := []error{
		f.SetColWidth(sh, "A", "A", 36),
		f.SetColWidth(sh, "B", "B", 13),
		f.SetColWidth(sh, "C", "C", 9.5),
		f.SetColWidth(sh, "D", "D", 20),
		f.SetColWidth(sh, "E", "E", 21),
		f.SetColWidth(sh, "F", "F", 17),
		f.SetRowHeight(sh, 1, 30),
		// Six columns are too wide for a portrait page.
		f.SetPageLayout(sh, &excelize.PageLayoutOptions{Orientation: ptrTo("landscape")}),
		f.MergeCell(sh, "A1", "F1"),
		f.SetCellStyle(sh, "A1", "F1", st.dashTitle),
		f.SetCellStr(sh, "A1", dashboardSheet),
		f.SetCellStyle(sh, "A2", "F2", st.dashHeader),
	}
	for i, h := range dashboardHeader {
		steps = append(steps, f.SetCellStr(sh, previewCols[i]+"2", h))
	}
	for i, r := range rows {
		n := strconv.Itoa(i + 3)
		status, remark := st.dashOpen, st.dashRemark
		if r.Status == report.StatusApproved {
			status, remark = st.dashApproved, st.dashRemarkSet
			steps = append(steps, f.SetCellStr(sh, "E"+n, "Checked"), f.SetCellStr(sh, "F"+n, "Ready for Pick Up"))
		}
		steps = append(steps,
			f.SetCellStyle(sh, "A"+n, "A"+n, st.dashChurch),
			f.SetCellStyle(sh, "B"+n, "B"+n, st.dashMonth),
			f.SetCellStyle(sh, "C"+n, "C"+n, st.dashYear),
			f.SetCellStyle(sh, "D"+n, "D"+n, status),
			f.SetCellStyle(sh, "E"+n, "F"+n, remark),
			f.SetCellStr(sh, "A"+n, r.Church),
			f.SetCellStr(sh, "B"+n, r.Month),
			f.SetCellStr(sh, "D"+n, r.Status),
		)
		if r.Year != 0 {
			steps = append(steps, f.SetCellInt(sh, "C"+n, int64(r.Year)))
		}
	}
	for _, err := range steps {
		if err != nil {
			return err
		}
	}
	return nil
}

// Approved Reports cell kinds.
const (
	flatText = iota
	flatCount
	flatMoney
)

// approvedHeader is Approved Reports' first columns; the 21 metrics follow, in
// report.MetricKeys order, under their report-sheet labels.
var approvedHeader = []string{"Church", "Worker", "Area of Assignment", "Month", "Year"}

// monthTotal sums a metric's reported weeks; false when no week is reported.
func monthTotal(weeks map[int]pgtype.Numeric) (float64, bool) {
	sum, ok := 0.0, false
	for week, v := range weeks {
		fv, err := v.Float64Value()
		if err != nil || !fv.Valid || week < 1 || week > 5 {
			continue
		}
		sum, ok = sum+fv.Float64, true
	}
	return sum, ok
}

// writeApproved writes every approved report as one row of month totals under
// a merged title and a frozen header, with zebra rows and a bold Total row. A
// metric with no reported week is blank, not 0; text is never a formula.
func writeApproved(f *excelize.File, reps []*report.Report, st excelStyles) error {
	sh := approvedSheet
	header := slices.Clone(approvedHeader)
	for _, key := range report.MetricKeys {
		header = append(header, rowLabels[metricRows[key]])
	}
	lastCol, err := excelize.ColumnNumberToName(len(header))
	if err != nil {
		return err
	}
	steps := []error{
		f.SetRowHeight(sh, 1, 30),
		f.SetRowHeight(sh, 2, 42),
		f.MergeCell(sh, "A1", lastCol+"1"),
		f.SetCellStyle(sh, "A1", lastCol+"1", st.dashTitle),
		f.SetCellStr(sh, "A1", approvedSheet),
		f.SetCellStyle(sh, "A2", lastCol+"2", st.flatHeader),
		f.SetPanes(sh, &excelize.Panes{Freeze: true, XSplit: 1, YSplit: 2, TopLeftCell: "B3", ActivePane: "bottomRight"}),
		f.SetPageLayout(sh, &excelize.PageLayoutOptions{Orientation: ptrTo("landscape")}),
	}
	w := &cellWriter{f: f, sh: sh, steps: steps, widths: make([]int, len(header))}
	put := w.put
	for c, h := range header {
		put(c+1, 2, h, st.flatHeader)
		w.widths[c] = min(w.widths[c], 14) // long metric labels wrap instead of widening the column
	}
	kindOf := func(key string) int {
		if key == "tithes_offering" {
			return flatMoney
		}
		return flatCount
	}
	totals, reported := map[string]float64{}, map[string]bool{}
	for i, r := range reps {
		row, odd := i+3, i%2
		for c, v := range []any{deref(r.ChurchName), deref(r.Worker), deref(r.AreaAssignment), r.Month, int(r.Year)} {
			put(c+1, row, v, st.flat[flatText][odd])
		}
		for c, key := range report.MetricKeys {
			var v any
			if sum, ok := monthTotal(r.Metrics[key]); ok {
				v, totals[key], reported[key] = sum, totals[key]+sum, true
			}
			put(len(approvedHeader)+c+1, row, v, st.flat[kindOf(key)][odd])
		}
	}
	row := len(reps) + 3
	put(1, row, "Total", st.flatTotal[flatText])
	for c := 2; c <= len(approvedHeader); c++ {
		put(c, row, nil, st.flatTotal[flatText])
	}
	for c, key := range report.MetricKeys {
		var v any
		if reported[key] {
			v = totals[key]
		}
		put(len(approvedHeader)+c+1, row, v, st.flatTotal[kindOf(key)])
	}
	return w.finish()
}

// cellWriter writes typed cells to one sheet, collecting errors and each
// column's widest content for finish.
type cellWriter struct {
	f      *excelize.File
	sh     string
	steps  []error
	widths []int
}

// put writes v (a string, an int, a float64, or nil for a blank cell) at
// col, row with style.
func (w *cellWriter) put(col, row int, v any, style int) {
	ref, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		w.steps = append(w.steps, err)
		return
	}
	width := 0
	switch v := v.(type) {
	case string:
		w.steps, width = append(w.steps, w.f.SetCellStr(w.sh, ref, v)), utf8.RuneCountInString(v)
	case int:
		w.steps, width = append(w.steps, w.f.SetCellInt(w.sh, ref, int64(v))), len(strconv.Itoa(v))
	case float64:
		digits := len(strconv.FormatFloat(v, 'f', 0, 64))
		w.steps, width = append(w.steps, w.f.SetCellFloat(w.sh, ref, v, -1, 64)), digits+digits/3+3
	}
	w.widths[col-1] = max(w.widths[col-1], width)
	w.steps = append(w.steps, w.f.SetCellStyle(w.sh, ref, ref, style))
}

// finish sizes the columns to their content and returns the first error.
func (w *cellWriter) finish() error {
	for c, width := range w.widths {
		col, err := excelize.ColumnNumberToName(c + 1)
		if err != nil {
			return err
		}
		w.steps = append(w.steps, w.f.SetColWidth(w.sh, col, col, float64(min(max(width+2, 10), 40))))
	}
	for _, err := range w.steps {
		if err != nil {
			return err
		}
	}
	return nil
}

// styleRange applies one style to the cells from:to.
type styleRange struct {
	from, to string
	id       int
}

func writeSheet(f *excelize.File, sh string, r *report.Report, st excelStyles) error {
	height := 15.75
	strs := map[string]string{
		"A1": "ANG MANANAMPALATAYANG GUMAWA", "A3": "NATIONAL WORKERS' MONTHLY MINISTRY REPORT",
		"A4": "Month of:", "A5": "Worker", "A6": "Area of Assignment", "A7": "Name of Church/Outreach:",
		"A8": "Weekly Attendance", "A9": "Activities", "G9": "Average",
		"A29": "Other Statistics", "G30": "Average",
		"A37": "Names:", "A40": "Narrative Report:", "A41": "Challenges/Problem Encountered", "A42": "Prayer Request",
		"B5": deref(r.Worker), "B6": deref(r.AreaAssignment), "B7": deref(r.ChurchName),
		"B37": deref(r.Names), "B40": deref(r.NarrativeReport), "B41": deref(r.Challenges), "B42": deref(r.PrayerRequest),
	}
	for i, col := range weekCols {
		strs[col+"9"] = "Week " + strconv.Itoa(i+1)
		strs[col+"30"] = "Week " + strconv.Itoa(i+1)
	}
	for row, label := range rowLabels {
		strs["A"+strconv.Itoa(row)] = label
	}
	styles := []styleRange{
		{"A1", "G2", st.title}, {"A3", "G3", st.subtitle},
		{"A4", "A7", st.label}, {"B4", "G4", st.month}, {"B5", "G7", st.value},
		{"A8", "A8", st.bandLabel}, {"B8", "G8", st.band}, {"A9", "A9", st.headerLabel}, {"B9", "G9", st.header},
		{"A10", "A28", st.mainLabel}, {"B10", "G28", st.cell}, {"B27", "G27", st.money},
		{"A29", "A29", st.bandLabel}, {"B29", "G29", st.band}, {"A30", "A30", st.subLabel}, {"B30", "G30", st.weekHeader},
		{"A31", "A37", st.mainLabel}, {"B31", "G36", st.cell}, {"B37", "G39", st.names}, {"A38", "A39", st.subLabel},
		{"A40", "A42", st.textLabel}, {"B40", "G42", st.text},
	}
	// The template's smaller Arial rows: sub-items and spacers. Row 19 keeps the
	// main style because D-A6 gave it the "Baptism" label.
	for _, row := range []string{"21", "22", "23", "25", "26", "28"} {
		styles = append(styles, styleRange{"A" + row, "A" + row, st.subLabel})
	}
	merges := [][2]string{
		{"A1", "G2"}, {"A3", "G3"}, {"B4", "G4"}, {"B5", "G5"}, {"B6", "G6"}, {"B7", "G7"},
		{"B37", "G39"}, {"B40", "G40"}, {"B41", "G41"}, {"B42", "G42"},
	}

	steps := []error{
		f.SetSheetProps(sh, &excelize.SheetPropsOptions{DefaultRowHeight: &height}),
		f.SetColWidth(sh, "A", "A", 26),
		f.SetColWidth(sh, "B", "G", 12.63),
		// Measured from the legacy screenshot: Sheets grows these rows, Excel does not.
		f.SetRowHeight(sh, 40, 56.25),
		f.SetRowHeight(sh, 41, 29.25),
		f.SetRowHeight(sh, 42, 45),
		f.SetPageLayout(sh, &excelize.PageLayoutOptions{Orientation: ptrTo("portrait")}),
		f.SetPageMargins(sh, &excelize.PageLayoutMarginsOptions{Horizontally: ptrTo(true)}),
	}
	for _, s := range styles {
		steps = append(steps, f.SetCellStyle(sh, s.from, s.to, s.id))
	}
	for cell, v := range strs {
		steps = append(steps, f.SetCellStr(sh, cell, v))
	}
	for _, m := range merges {
		steps = append(steps, f.MergeCell(sh, m[0], m[1]))
	}
	if i := slices.Index(report.Months, r.Month); i >= 0 {
		steps = append(steps, f.SetCellValue(sh, "B4", time.Date(int(r.Year), time.Month(i+1), 1, 0, 0, 0, 0, time.UTC)))
	}
	steps = append(steps, writeMetrics(f, sh, r)...)
	for _, err := range steps {
		if err != nil {
			return err
		}
	}
	return nil
}

// writeMetrics adds each metric row's Average formula and writes the
// reported weeks as numbers. An absent week stays blank: not reported (D-5).
func writeMetrics(f *excelize.File, sh string, r *report.Report) []error {
	var steps []error
	for row := range rowLabels {
		n := strconv.Itoa(row)
		steps = append(steps, f.SetCellFormula(sh, "G"+n, `IFERROR(AVERAGE(B`+n+`:F`+n+`),"")`))
	}
	for key, weeks := range r.Metrics {
		row, ok := metricRows[key]
		if !ok {
			continue
		}
		for week, v := range weeks {
			fv, err := v.Float64Value()
			if err != nil || !fv.Valid || week < 1 || week > 5 {
				continue
			}
			steps = append(steps, f.SetCellFloat(sh, weekCols[week-1]+strconv.Itoa(row), fv.Float64, -1, 64))
		}
	}
	return steps
}

// previewRows is the last template row; previewCols are its columns.
const previewRows = 42

var previewCols = [7]string{"A", "B", "C", "D", "E", "F", "G"}

// previewSheets reads every sheet's A1:G42 as the text Excel shows (D-A15).
// The Average formulas are not cached in a new workbook, so column G is
// calculated. excelize repeats a merged range's value in every cell it covers;
// only the top-left cell keeps it here. With dashboard, the first sheet is the
// Report Dashboard and is read as its title, header and one row per account, 6 wide.
func previewSheets(f *excelize.File, dashboard bool) ([]report.PreviewSheet, error) {
	sheets := make([]report.PreviewSheet, 0, f.SheetCount)
	for i, sh := range f.GetSheetList() {
		if dashboard && (i == 0 || sh == approvedSheet) {
			rows, err := dashboardRows(f, sh)
			if err != nil {
				return nil, err
			}
			sheets = append(sheets, report.PreviewSheet{Name: sh, Rows: rows})
			continue
		}
		covered, err := mergedTails(f, sh)
		if err != nil {
			return nil, err
		}
		rows := make([][]string, previewRows)
		for i := range rows {
			rows[i] = make([]string, len(previewCols))
			for j, col := range previewCols {
				cell := col + strconv.Itoa(i+1)
				if covered[cell] {
					continue
				}
				read := f.GetCellValue
				if formula, err := f.GetCellFormula(sh, cell); err != nil {
					return nil, fmt.Errorf("read %s!%s: %w", sh, cell, err)
				} else if formula != "" {
					read = f.CalcCellValue
				}
				v, err := read(sh, cell)
				if err != nil {
					return nil, fmt.Errorf("read %s!%s: %w", sh, cell, err)
				}
				rows[i][j] = v
			}
		}
		sheets = append(sheets, report.PreviewSheet{Name: sh, Rows: rows})
	}
	return sheets, nil
}

// dashboardRows reads a table sheet (the dashboard or Approved Reports) as
// written: GetRows ends at the last written row; short rows are padded to the
// header's width and the merged title keeps its text in A1 only.
func dashboardRows(f *excelize.File, sh string) ([][]string, error) {
	rows, err := f.GetRows(sh)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", sh, err)
	}
	covered, err := mergedTails(f, sh)
	if err != nil {
		return nil, err
	}
	width := len(dashboardHeader)
	if len(rows) > 1 {
		width = max(width, len(rows[1]))
	}
	for i, r := range rows {
		r = append(r, make([]string, width-len(r))...)
		for j := range r {
			name, err := excelize.CoordinatesToCellName(j+1, i+1)
			if err != nil {
				return nil, err
			}
			if covered[name] {
				r[j] = ""
			}
		}
		rows[i] = r
	}
	return rows, nil
}

// mergedTails returns the cells of sh's merged ranges other than their top-left cell.
func mergedTails(f *excelize.File, sh string) (map[string]bool, error) {
	merges, err := f.GetMergeCells(sh)
	if err != nil {
		return nil, fmt.Errorf("merged cells of %s: %w", sh, err)
	}
	tails := map[string]bool{}
	for _, m := range merges {
		c1, r1, err := excelize.CellNameToCoordinates(m.GetStartAxis())
		if err != nil {
			return nil, err
		}
		c2, r2, err := excelize.CellNameToCoordinates(m.GetEndAxis())
		if err != nil {
			return nil, err
		}
		for r := r1; r <= r2; r++ {
			for c := c1; c <= c2; c++ {
				if r == r1 && c == c1 {
					continue
				}
				name, err := excelize.CoordinatesToCellName(c, r)
				if err != nil {
					return nil, err
				}
				tails[name] = true
			}
		}
	}
	return tails, nil
}

func ptrTo[T any](v T) *T { return &v }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// sheetLabel is the report's label: church, else worker, else "Report <id>".
func sheetLabel(r *report.Report) string {
	for _, s := range []*string{r.ChurchName, r.Worker} {
		if v := strings.TrimSpace(deref(s)); v != "" {
			return v
		}
	}
	return "Report " + strconv.FormatInt(r.ID, 10)
}

// sheetNames returns a valid, unique Excel sheet name for each report, never
// one of reserved (any case): the
// characters []:*?/\ become "-", names are cut to 31 UTF-16 units with no '
// at either end, and repeats get " (2)", " (3)"… (the base is cut so the
// suffix still fits).
func sheetNames(reserved []string, reps []*report.Report) []string {
	clean := strings.NewReplacer("[", "-", "]", "-", ":", "-", "*", "-", "?", "-", "/", "-", `\`, "-")
	seen := map[string]bool{}
	for _, name := range reserved {
		seen[strings.ToLower(name)] = true
	}
	out := make([]string, 0, len(reps))
	for _, r := range reps {
		base := strings.Trim(clean.Replace(sheetLabel(r)), "'")
		if base == "" {
			base = "Report " + strconv.FormatInt(r.ID, 10)
		}
		name := fitSheetName(base, 31)
		for n := 2; seen[strings.ToLower(name)]; n++ {
			suffix := fmt.Sprintf(" (%d)", n)
			name = fitSheetName(base, 31-len(suffix)) + suffix
		}
		seen[strings.ToLower(name)] = true
		out = append(out, name)
	}
	return out
}

// fitSheetName cuts s to at most n UTF-16 units (whole runes only), the unit
// excelize counts, and trims the ' it forbids at either end.
func fitSheetName(s string, n int) string {
	units := 0
	for i, c := range s {
		if units += utf16.RuneLen(c); units > n {
			s = s[:i]
			break
		}
	}
	return strings.Trim(s, "'")
}

// exportFilename is "<label> <Month> <Year>.xlsx" with path separators and
// control characters removed.
func exportFilename(label, month string, year int) string {
	name := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, fmt.Sprintf("%s %s %d", label, month, year))
	return strings.TrimSpace(name) + ".xlsx"
}
