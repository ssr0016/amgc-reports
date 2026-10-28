package statsimpl

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"

	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
)

// The Analytics page's Excel download (M5 follow-up). Each sheet is one table
// under a three-row title block, with a frozen header, zebra rows and columns
// sized to their content. It carries what the page's CSV carried.

const (
	workbookTitle = "AMGCReports — Analytics Report"
	headerRow     = 5 // rows 1-3 are the title block, row 4 is blank
	minTitleCols  = 6 // the title is merged across at least this many columns so it is not clipped
)

// metricLabels are the frontend's METRICS labels (frontend/src/lib/report-form.ts).
var metricLabels = map[string]string{
	"worship_service": "Worship Service", "sunday_school": "Sunday School", "prayer_meeting": "Prayer Meeting",
	"bible_studies": "Bible Studies", "mens_fellowship": "Men's Fellowship", "womens_fellowship": "Women's Fellowship",
	"youth_fellowship": "Youth Fellowship", "children_fellowship": "Children's Fellowship", "tithes_offering": "Tithes & Offering",
	"home_visited": "Homes Visited", "bible_study_group_led": "Bible Study Group Led", "sermon_preached": "Sermons Preached",
	"person_newly_contacted": "Newly Contacted", "person_followed_up": "Persons Followed Up", "person_evangelized": "Persons Evangelized",
	"outreach": "Outreach", "training": "Training", "leadership": "Leadership", "baptism": "Baptism",
	"other": "Other", "family_day": "Family Day",
}

func metricLabel(key string) string {
	if l, ok := metricLabels[key]; ok {
		return l
	}
	return key
}

// periodLabel is the page's period label: "Full Year", "Q1", or the month name.
func periodLabel(period string) string {
	switch {
	case period == stats.AttendancePeriodFullYear:
		return "Full Year"
	case len(period) == 2 && period[0] == 'q':
		return strings.ToUpper(period)
	case period == "":
		return period
	}
	return strings.ToUpper(period[:1]) + period[1:]
}

// Cell values: a string is text, an int a count, a float64 a metric value, a
// pct a ratio shown as a signed percentage, and nil a blank cell.
type pct float64

type table struct {
	sheet          string
	headers        []string
	rows           [][]any
	freezeFirstCol bool
}

type cellKind int

const (
	kindText cellKind = iota
	kindCount
	kindValue
	kindPct
)

func kindOf(v any) cellKind {
	switch v.(type) {
	case int:
		return kindCount
	case float64:
		return kindValue
	case pct:
		return kindPct
	}
	return kindText
}

type analyticsStyles struct {
	title, subtitle, generated, header int
	cells                              [4][2]int // [kind][odd row]
}

func newAnalyticsStyles(f *excelize.File) (analyticsStyles, error) {
	border := []excelize.Border{
		{Type: "left", Color: "BFBFBF", Style: 1}, {Type: "right", Color: "BFBFBF", Style: 1},
		{Type: "top", Color: "BFBFBF", Style: 1}, {Type: "bottom", Color: "BFBFBF", Style: 1},
	}
	font := func(size float64, bold bool, color string) *excelize.Font {
		return &excelize.Font{Family: "Calibri", Size: size, Bold: bold, Color: color}
	}
	var st analyticsStyles
	specs := []struct {
		id    *int
		style *excelize.Style
	}{
		{&st.title, &excelize.Style{Font: font(18, true, "1F1F1F"), Alignment: &excelize.Alignment{Vertical: "center"}}},
		{&st.subtitle, &excelize.Style{Font: font(12, true, "404040")}},
		{&st.generated, &excelize.Style{Font: &excelize.Font{Family: "Calibri", Size: 10, Italic: true, Color: "7F7F7F"}}},
		{&st.header, &excelize.Style{
			Font: font(11, true, "FFFFFF"), Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"404040"}},
			Border: border, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		}},
	}
	numFmts := [4]string{kindCount: "#,##0", kindValue: "#,##0.00", kindPct: "+0.0%;-0.0%;0.0%"}
	for kind := range st.cells {
		for odd := range st.cells[kind] {
			s := &excelize.Style{Font: font(11, false, "000000"), Border: border, Alignment: &excelize.Alignment{Vertical: "center"}}
			if odd == 1 {
				s.Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F2F2F2"}}
			}
			if numFmts[kind] != "" {
				s.CustomNumFmt = &numFmts[kind]
				s.Alignment.Horizontal = "right"
			}
			specs = append(specs, struct {
				id    *int
				style *excelize.Style
			}{&st.cells[kind][odd], s})
		}
	}
	for _, sp := range specs {
		id, err := f.NewStyle(sp.style)
		if err != nil {
			return analyticsStyles{}, fmt.Errorf("new style: %w", err)
		}
		*sp.id = id
	}
	return st, nil
}

// displayWidth estimates the characters a cell shows, for sizing its column.
func displayWidth(v any) int {
	switch v := v.(type) {
	case nil:
		return 0
	case string:
		return utf8.RuneCountInString(v)
	case int:
		return len(strconv.Itoa(v)) + len(strconv.Itoa(v))/3
	case float64:
		digits := len(strconv.FormatFloat(v, 'f', 0, 64))
		return digits + digits/3 + 3
	case pct:
		return 8
	}
	return 0
}

func cellRef(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row) // col and row are always >= 1
	return name
}

func writeTable(f *excelize.File, st analyticsStyles, t table, subtitle, generated string) error {
	if _, err := f.NewSheet(t.sheet); err != nil {
		return fmt.Errorf("new sheet %q: %w", t.sheet, err)
	}
	if err := writeTitle(f, st, t.sheet, max(len(t.headers), minTitleCols), subtitle, generated); err != nil {
		return fmt.Errorf("title of %q: %w", t.sheet, err)
	}
	widths, err := writeCells(f, st, t)
	if err != nil {
		return fmt.Errorf("cells of %q: %w", t.sheet, err)
	}
	for c, w := range widths {
		col, _ := excelize.ColumnNumberToName(c + 1)
		if err := f.SetColWidth(t.sheet, col, col, float64(min(max(w+2, 10), 50))); err != nil {
			return err
		}
	}
	panes := &excelize.Panes{Freeze: true, YSplit: headerRow, TopLeftCell: cellRef(1, headerRow+1), ActivePane: "bottomLeft"}
	if t.freezeFirstCol {
		panes = &excelize.Panes{Freeze: true, XSplit: 1, YSplit: headerRow, TopLeftCell: cellRef(2, headerRow+1), ActivePane: "bottomRight"}
	}
	if err := f.SetPanes(t.sheet, panes); err != nil {
		return fmt.Errorf("freeze %q: %w", t.sheet, err)
	}
	return nil
}

// writeTitle writes the title block: the merged title, the period and the generated-at line.
func writeTitle(f *excelize.File, st analyticsStyles, sheet string, lastCol int, subtitle, generated string) error {
	if err := f.MergeCell(sheet, "A1", cellRef(lastCol, 1)); err != nil {
		return err
	}
	if err := f.SetRowHeight(sheet, 1, 30); err != nil {
		return err
	}
	for _, line := range []struct {
		row   int
		text  string
		style int
	}{{1, workbookTitle, st.title}, {2, subtitle, st.subtitle}, {3, generated, st.generated}} {
		if err := f.SetCellStr(sheet, cellRef(1, line.row), line.text); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cellRef(1, line.row), cellRef(lastCol, line.row), line.style); err != nil {
			return err
		}
	}
	return nil
}

// writeCells writes the header and the zebra-striped rows, and returns each
// column's widest content.
func writeCells(f *excelize.File, st analyticsStyles, t table) ([]int, error) {
	widths := make([]int, len(t.headers))
	for c, h := range t.headers {
		widths[c] = utf8.RuneCountInString(h) + 2 // bold header
		if err := f.SetCellStr(t.sheet, cellRef(c+1, headerRow), h); err != nil {
			return nil, err
		}
	}
	if err := f.SetCellStyle(t.sheet, cellRef(1, headerRow), cellRef(len(t.headers), headerRow), st.header); err != nil {
		return nil, err
	}
	rows := t.rows
	if len(rows) == 0 {
		rows = [][]any{{"No data for this period."}}
	}
	for r, values := range rows {
		for c := range t.headers {
			var v any
			if c < len(values) {
				v = values[c]
			}
			ref := cellRef(c+1, headerRow+1+r)
			if err := writeCell(f, t.sheet, ref, v); err != nil {
				return nil, err
			}
			if err := f.SetCellStyle(t.sheet, ref, ref, st.cells[kindOf(v)][r%2]); err != nil {
				return nil, err
			}
			widths[c] = max(widths[c], displayWidth(v))
		}
	}
	return widths, nil
}

func writeCell(f *excelize.File, sheet, ref string, v any) error {
	switch v := v.(type) {
	case nil:
		return nil
	case pct:
		return f.SetCellFloat(sheet, ref, float64(v), -1, 64)
	default:
		return f.SetCellValue(sheet, ref, v)
	}
}

// change is the year-on-year change, as the page shows it: 0% when both are 0,
// "New" when only last year is 0.
func change(current, previous float64) any {
	if previous == 0 {
		if current == 0 {
			return pct(0)
		}
		return "New"
	}
	return pct((current - previous) / previous)
}

// totalAndPeak sums the trend and finds the month with the highest total; the
// earliest wins a tie, and no month above 0 gives "".
func totalAndPeak(trend []stats.AttendanceTrendPoint) (float64, string, float64) {
	var total float64
	monthly := map[string]float64{}
	var order []string
	for _, p := range trend {
		total += p.Value
		if _, ok := monthly[p.Month]; !ok {
			order = append(order, p.Month)
		}
		monthly[p.Month] += p.Value
	}
	peak, peakValue := "", 0.0
	for _, m := range order {
		if monthly[m] > peakValue {
			peak, peakValue = m, monthly[m]
		}
	}
	return total, peak, peakValue
}

func activeChurches(churches []stats.AttendanceChurch) int {
	n := 0
	for _, c := range churches {
		if c.Value > 0 {
			n++
		}
	}
	return n
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// buildAnalyticsWorkbook lays out data on four sheets: Summary (the KPI cards),
// Weekly Trend, Church Breakdown and All Metrics. churchesTotal is the page's
// "out of" count; now is the generated-at time.
func buildAnalyticsWorkbook(data stats.AttendanceAnalytics, churchesTotal int64, now time.Time) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	st, err := newAnalyticsStyles(f)
	if err != nil {
		return nil, err
	}
	status := "Approved"
	if data.Status == stats.AttendanceStatusAll {
		status = "All Submitted"
	}
	subtitle := fmt.Sprintf("%s · %s %d · %s", metricLabel(data.Metric), periodLabel(data.Period), data.Year, status)
	generated := "Generated " + now.Format("2 Jan 2006, 15:04") + " (" + now.Location().String() + ")"
	for _, t := range []table{summaryTable(data, churchesTotal), trendTable(data), churchTable(data), allMetricsTable(data)} {
		if err := writeTable(f, st, t, subtitle, generated); err != nil {
			return nil, err
		}
	}
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return nil, fmt.Errorf("delete default sheet: %w", err)
	}
	f.SetActiveSheet(0)
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write workbook: %w", err)
	}
	return buf.Bytes(), nil
}

// summaryTable is the page's three KPI cards.
func summaryTable(data stats.AttendanceAnalytics, churchesTotal int64) table {
	total, peak, peakValue := totalAndPeak(data.Trend)
	active := activeChurches(data.Churches)
	t := table{sheet: "Summary", headers: []string{"KPI", "Value"}, rows: [][]any{
		{"Total", total},
		{"Peak Month", orDash(peak)},
		{"Active Churches", fmt.Sprintf("%d out of %d", active, churchesTotal)},
	}}
	if cmp := data.Comparison; cmp != nil {
		prevTotal, prevPeak, prevPeakValue := totalAndPeak(cmp.Trend)
		prevActive := activeChurches(cmp.Churches)
		t.headers = append(t.headers, strconv.Itoa(cmp.Year), "Change")
		t.rows[0] = append(t.rows[0], prevTotal, change(total, prevTotal))
		t.rows[1] = append(t.rows[1], orDash(prevPeak), change(peakValue, prevPeakValue))
		t.rows[2] = append(t.rows[2], prevActive, change(float64(active), float64(prevActive)))
	}
	return t
}

// trendTable lists the weeks with a value this year or last.
func trendTable(data stats.AttendanceAnalytics) table {
	t := table{sheet: "Weekly Trend", headers: []string{"Month", "Week", strconv.Itoa(data.Year)}}
	cmp := data.Comparison
	previous := map[string]float64{}
	if cmp != nil {
		t.headers = append(t.headers, strconv.Itoa(cmp.Year), "Change")
		for _, p := range cmp.Trend {
			previous[p.Month+"-"+strconv.Itoa(p.Week)] = p.Value
		}
	}
	for _, p := range data.Trend {
		prev := previous[p.Month+"-"+strconv.Itoa(p.Week)]
		if p.Value == 0 && prev == 0 {
			continue
		}
		row := []any{p.Month, p.Week, p.Value}
		if cmp != nil {
			row = append(row, prev, change(p.Value, prev))
		}
		t.rows = append(t.rows, row)
	}
	return t
}

// churchTable lists every church in either year.
func churchTable(data stats.AttendanceAnalytics) table {
	t := table{sheet: "Church Breakdown", headers: []string{"Church", strconv.Itoa(data.Year)}}
	cmp := data.Comparison
	current, previous := map[string]float64{}, map[string]float64{}
	var names []string
	for _, c := range data.Churches {
		current[c.ChurchName] = c.Value
		names = append(names, c.ChurchName)
	}
	if cmp != nil {
		t.headers = append(t.headers, strconv.Itoa(cmp.Year), "Change")
		for _, c := range cmp.Churches {
			if _, ok := current[c.ChurchName]; !ok {
				names = append(names, c.ChurchName)
			}
			previous[c.ChurchName] = c.Value
		}
	}
	for _, name := range names {
		row := []any{name, current[name]}
		if cmp != nil {
			row = append(row, previous[name], change(current[name], previous[name]))
		}
		t.rows = append(t.rows, row)
	}
	return t
}

// allMetricsTable is every metric's period total, overall and per church; a
// church with no value for a metric gets a blank cell.
func allMetricsTable(data stats.AttendanceAnalytics) table {
	t := table{sheet: "All Metrics", headers: []string{"Metric", "Total for Period"}, freezeFirstCol: true}
	var names []string
	seen := map[string]bool{}
	for _, row := range data.Summary {
		for _, c := range row.Churches {
			if !seen[c.ChurchName] {
				seen[c.ChurchName] = true
				names = append(names, c.ChurchName)
			}
		}
	}
	t.headers = append(t.headers, names...)
	optional := func(v *float64) any {
		if v == nil {
			return nil
		}
		return *v
	}
	for _, row := range data.Summary {
		values := map[string]*float64{}
		for _, c := range row.Churches {
			values[c.ChurchName] = c.Value
		}
		out := []any{metricLabel(row.Metric), optional(row.Total)}
		for _, name := range names {
			out = append(out, optional(values[name]))
		}
		t.rows = append(t.rows, out)
	}
	return t
}
