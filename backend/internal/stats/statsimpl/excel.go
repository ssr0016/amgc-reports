package statsimpl

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"

	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
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
// pct a ratio shown as a signed percentage, a rate an unsigned one, a status a
// colour-coded report status, and nil a blank cell.
type (
	pct    float64
	rate   float64
	status string
)

const (
	statusApproved      status = "Approved"
	statusPending       status = "Pending"
	statusNeedsRevision status = "Needs Revision"
	statusNoReport      status = "No Report"
)

// statusColors are Excel's own good/neutral/bad cell styles: {fill, font}.
// Pending and Needs Revision share amber: both still need an admin.
var statusColors = map[status][2]string{
	statusApproved: {"C6EFCE", "006100"}, statusPending: {"FFEB9C", "9C5700"}, statusNeedsRevision: {"FFEB9C", "9C5700"},
	statusNoReport: {"FFC7CE", "9C0006"},
}

type table struct {
	sheet          string
	headers        []string
	rows           [][]any
	freezeFirstCol bool
	filter         bool   // adds Excel's AutoFilter dropdowns to the header row
	subtitle       string // overrides the workbook subtitle when set
	printReport    bool   // landscape, one page wide, for printing the sheet as the report
}

type cellKind int

const (
	kindText cellKind = iota
	kindCount
	kindValue
	kindPct
	kindRate
)

func kindOf(v any) cellKind {
	switch v.(type) {
	case int:
		return kindCount
	case float64:
		return kindValue
	case pct:
		return kindPct
	case rate:
		return kindRate
	}
	return kindText
}

type analyticsStyles struct {
	title, subtitle, generated, header int
	cells                              [5][2]int // [kind][odd row]
	status                             map[status]int
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
	numFmts := [5]string{kindCount: "#,##0", kindValue: "#,##0.00", kindPct: "+0.0%;-0.0%;0.0%", kindRate: "0.0%"}
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
	st.status = map[status]int{}
	for s, c := range statusColors {
		id, err := f.NewStyle(&excelize.Style{
			Font: font(11, false, c[1]), Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{c[0]}},
			Border: border, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		})
		if err != nil {
			return analyticsStyles{}, fmt.Errorf("new status style: %w", err)
		}
		st.status[s] = id
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
	case status:
		return utf8.RuneCountInString(string(v))
	case int:
		return len(strconv.Itoa(v)) + len(strconv.Itoa(v))/3
	case float64:
		digits := len(strconv.FormatFloat(v, 'f', 0, 64))
		return digits + digits/3 + 3
	case pct, rate:
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
	if t.subtitle != "" {
		subtitle = t.subtitle
	}
	if t.printReport {
		landscape, one, anyTall := "landscape", 1, 0
		if err := f.SetPageLayout(t.sheet, &excelize.PageLayoutOptions{Orientation: &landscape, FitToWidth: &one, FitToHeight: &anyTall}); err != nil {
			return fmt.Errorf("page layout %q: %w", t.sheet, err)
		}
		fit := true
		if err := f.SetSheetProps(t.sheet, &excelize.SheetPropsOptions{FitToPage: &fit}); err != nil {
			return fmt.Errorf("fit %q: %w", t.sheet, err)
		}
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
	if t.filter && len(t.rows) > 0 {
		ref := cellRef(1, headerRow) + ":" + cellRef(len(t.headers), headerRow+len(t.rows))
		if err := f.AutoFilter(t.sheet, ref, nil); err != nil {
			return fmt.Errorf("filter %q: %w", t.sheet, err)
		}
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
			style := st.cells[kindOf(v)][r%2]
			if s, ok := v.(status); ok && st.status[s] != 0 {
				style = st.status[s] // a coloured status replaces the zebra fill
			}
			if err := f.SetCellStyle(t.sheet, ref, ref, style); err != nil {
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
	case rate:
		return f.SetCellFloat(sheet, ref, float64(v), -1, 64)
	case status:
		return f.SetCellStr(sheet, ref, string(v))
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

// monthTracker is one due month's tracker rows, for the compliance sheet.
type monthTracker struct {
	year  int
	month time.Month
	rows  []stats.TrackerRow
}

// counts reports whether row is due for mt: an active account, and not a month
// that ended before the account existed (unless a report was filed anyway).
func (mt monthTracker) counts(row stats.TrackerRow) bool {
	if row.UserStatus != user.UserStatusActive {
		return false
	}
	c := row.CreatedAt
	beforeCreation := mt.year < c.Year() || mt.year == c.Year() && mt.month < c.Month()
	return !beforeCreation || row.SubmissionStatus == stats.TrackerSubmitted
}

// dueMonths is the period's months whose reports are due by now: a month is
// reported after it ends, so the current month and later are not yet due.
func dueMonths(year int, period string, now time.Time) []time.Month {
	start, end, _ := stats.AnalyticsPeriodBounds(period)
	var out []time.Month
	for m := time.Month(start); m <= time.Month(end); m++ {
		if year > now.Year() || year == now.Year() && m >= now.Month() {
			break
		}
		out = append(out, m)
	}
	return out
}

// monthNumber is the month a trend point's name stands for; 0 when unknown.
func monthNumber(name string) time.Month {
	t, err := time.Parse("January", name)
	if err != nil {
		return 0
	}
	return t.Month()
}

// perWeek is total over weeks, blank when there are no weeks.
func perWeek(total float64, weeks int) any {
	if weeks == 0 {
		return nil
	}
	return total / float64(weeks)
}

// buildAnalyticsWorkbook lays out data on seven sheets: Full Metrics Summary
// (the one-page report, first so it opens first), Summary (the KPI cards),
// Monthly Totals, Weekly Trend, Church Breakdown, Metric Detail and Submission
// Compliance. churchesTotal is the page's "out of" count;
// compliance is the tracker for each due month; now is the generated-at time.
func buildAnalyticsWorkbook(data stats.AttendanceAnalytics, churchesTotal int64, compliance []monthTracker, now time.Time) ([]byte, error) {
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
	weeks := data.Weeks
	for _, t := range []table{
		allMetricsTable(data, weeks, status), summaryTable(data, churchesTotal, weeks, compliance), monthlyTable(data),
		trendTable(data), churchTable(data), detailTable(data), complianceTable(compliance),
	} {
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

// summaryTable is the page's three KPI cards, the average per week and the
// submission rate. weeks is data.Weeks, the weeks with reports.
func summaryTable(data stats.AttendanceAnalytics, churchesTotal int64, weeks int, compliance []monthTracker) table {
	total, peak, peakValue := totalAndPeak(data.Trend)
	active := activeChurches(data.Churches)
	t := table{sheet: "Summary", headers: []string{"KPI", "Value"}, rows: [][]any{
		{"Total", total},
		{"Peak Month", orDash(peak)},
		{"Active Churches", fmt.Sprintf("%d out of %d", active, churchesTotal)},
		{"Average per Week", perWeek(total, weeks)},
		{"Submission Rate", submissionRate(compliance)},
	}}
	if cmp := data.Comparison; cmp != nil {
		prevTotal, prevPeak, prevPeakValue := totalAndPeak(cmp.Trend)
		prevActive := activeChurches(cmp.Churches)
		prevAvg := perWeek(prevTotal, cmp.Weeks)
		t.headers = append(t.headers, strconv.Itoa(cmp.Year), "Change")
		t.rows[0] = append(t.rows[0], prevTotal, change(total, prevTotal))
		t.rows[1] = append(t.rows[1], orDash(prevPeak), change(peakValue, prevPeakValue))
		t.rows[2] = append(t.rows[2], prevActive, change(float64(active), float64(prevActive)))
		t.rows[3] = append(t.rows[3], prevAvg, changeOf(t.rows[3][1], prevAvg))
	}
	return t
}

// changeOf is change for two perWeek cells; blank when either is blank.
func changeOf(current, previous any) any {
	c, ok1 := current.(float64)
	p, ok2 := previous.(float64)
	if !ok1 || !ok2 {
		return nil
	}
	return change(c, p)
}

// monthlyTable is each month's total and average per week with reports, so a
// month with fewer weeks reported compares fairly with a full one. It lists months with data this year or last.
func monthlyTable(data stats.AttendanceAnalytics) table {
	y := strconv.Itoa(data.Year)
	t := table{sheet: "Monthly Totals", headers: []string{"Month", "Weeks", y + " Total", y + " Avg per Week"}}
	monthly := func(trend []stats.AttendanceTrendPoint) map[time.Month]float64 {
		out := map[time.Month]float64{}
		for _, p := range trend {
			out[monthNumber(p.Month)] += p.Value
		}
		return out
	}
	current, previous := monthly(data.Trend), map[time.Month]float64{}
	cmp := data.Comparison
	if cmp != nil {
		py := strconv.Itoa(cmp.Year)
		t.headers = append(t.headers, py+" Total", py+" Avg per Week", "Change (Avg)")
		previous = monthly(cmp.Trend)
	}
	start, end, _ := stats.AnalyticsPeriodBounds(data.Period)
	for m := time.Month(start); m <= time.Month(end); m++ {
		if current[m] == 0 && previous[m] == 0 {
			continue
		}
		weeks := data.WeeksByMonth[int(m)]
		avg := perWeek(current[m], weeks)
		row := []any{m.String(), weeks, current[m], avg}
		if cmp != nil {
			prevAvg := perWeek(previous[m], cmp.WeeksByMonth[int(m)])
			row = append(row, previous[m], prevAvg, changeOf(avg, prevAvg))
		}
		t.rows = append(t.rows, row)
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

// allMetricsTable is every metric's period total and peak month, overall and
// per church, grouped by stats.MetricGroups. A church with no value for a metric
// gets a blank cell; a church with no report at all gets No Report cells.
// statusLabel is the workbook's Approved or All Submitted.
func allMetricsTable(data stats.AttendanceAnalytics, weeks int, statusLabel string) table {
	t := table{sheet: "Full Metrics Summary", headers: []string{"Metric", "Category", "Total for Period", "Avg per Week", "Peak Month", "Peak Month Total"}, freezeFirstCol: true, printReport: true}
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
	t.headers = append(t.headers, data.NoReportChurches...)
	t.subtitle = fmt.Sprintf("All Metrics · %s %d · %s · %d of %d churches reporting",
		periodLabel(data.Period), data.Year, statusLabel, len(names), len(names)+len(data.NoReportChurches))
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
		var peakValue any
		if row.PeakMonth != "" {
			peakValue = row.PeakValue
		}
		var avg any
		if row.Total != nil {
			avg = perWeek(*row.Total, weeks)
		}
		out := []any{metricLabel(row.Metric), row.Group, optional(row.Total), avg, orDash(row.PeakMonth), peakValue}
		for _, name := range names {
			out = append(out, optional(values[name]))
		}
		for range data.NoReportChurches {
			out = append(out, statusNoReport)
		}
		t.rows = append(t.rows, out)
	}
	return t
}

// detailTable is every metric for every church and month, one row each, so an
// admin can filter or pivot it. Rows follow the report form's metric order.
func detailTable(data stats.AttendanceAnalytics) table {
	t := table{sheet: "Metric Detail", headers: []string{"Metric", "Church", "Month", strconv.Itoa(data.Year)}, filter: true}
	type key struct {
		metric, church string
		month          int
	}
	current, previous := map[key]float64{}, map[key]float64{}
	var keys []key
	seen := map[key]bool{}
	add := func(rows []stats.AttendanceDetail, into map[key]float64) {
		for _, d := range rows {
			k := key{d.Metric, d.ChurchName, d.Month}
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
			into[k] = d.Value
		}
	}
	add(data.Detail, current)
	prev := data.Comparison
	if prev != nil {
		t.headers = append(t.headers, strconv.Itoa(prev.Year), "Change")
		add(prev.Detail, previous)
	}
	order := map[string]int{}
	for i, m := range report.MetricKeys {
		order[m] = i
	}
	slices.SortFunc(keys, func(a, b key) int {
		return cmp.Or(cmp.Compare(order[a.metric], order[b.metric]), strings.Compare(a.church, b.church), cmp.Compare(a.month, b.month))
	})
	for _, k := range keys {
		row := []any{metricLabel(k.metric), k.church, time.Month(k.month).String(), current[k]}
		if prev != nil {
			row = append(row, previous[k], change(current[k], previous[k]))
		}
		t.rows = append(t.rows, row)
	}
	return t
}

// reviewLabel is a tracker row's status as the compliance sheet shows it.
func reviewLabel(row stats.TrackerRow) status {
	switch {
	case row.SubmissionStatus == stats.TrackerNoReport:
		return statusNoReport
	case row.ReviewStatus == stats.TrackerApproved:
		return statusApproved
	case row.ReviewStatus == stats.TrackerNeedsRevision:
		return statusNeedsRevision
	}
	return statusPending
}

// complianceTable is each active church account's report status for every due
// month of the period, with its submitted and approved counts and rate. A month
// before the account existed shows a dash and is not counted.
// shortcut: uses today's account status, so a church deactivated mid-year drops
// out of earlier months too; upgrade if admins need historical membership.
func complianceTable(compliance []monthTracker) table {
	t := table{sheet: "Submission Compliance", headers: []string{"Church", "Worker"}, freezeFirstCol: true}
	if len(compliance) == 0 {
		t.rows = [][]any{{"No month of this period is due yet."}}
		return t
	}
	type account struct {
		church, name string
		statuses     map[time.Month]status
	}
	var order []int64
	accounts := map[int64]*account{}
	for _, mt := range compliance {
		t.headers = append(t.headers, mt.month.String()[:3])
		for _, row := range mt.rows {
			if !mt.counts(row) {
				continue
			}
			a := accounts[row.UserID]
			if a == nil {
				a = &account{church: orDash(row.ChurchName), name: row.Name, statuses: map[time.Month]status{}}
				accounts[row.UserID] = a
				order = append(order, row.UserID)
			}
			a.statuses[mt.month] = reviewLabel(row)
		}
	}
	t.headers = append(t.headers, "Submitted", "Approved", "Submission Rate")
	for _, id := range order {
		a := accounts[id]
		out := []any{a.church, a.name}
		due, submitted, approved := 0, 0, 0
		for _, mt := range compliance {
			s, ok := a.statuses[mt.month]
			if !ok {
				out = append(out, "—")
				continue
			}
			out = append(out, s)
			due++
			if s != statusNoReport {
				submitted++
			}
			if s == statusApproved {
				approved++
			}
		}
		t.rows = append(t.rows, append(out, submitted, approved, rate(float64(submitted)/float64(due))))
	}
	return t
}

// submissionRate is the share of active accounts' due reports that were
// submitted; blank when nothing is due.
func submissionRate(compliance []monthTracker) any {
	due, submitted := 0, 0
	for _, mt := range compliance {
		for _, row := range mt.rows {
			if !mt.counts(row) {
				continue
			}
			due++
			if row.SubmissionStatus == stats.TrackerSubmitted {
				submitted++
			}
		}
	}
	if due == 0 {
		return nil
	}
	return rate(float64(submitted) / float64(due))
}
