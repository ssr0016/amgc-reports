package reportimpl

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

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

type excelStyles struct {
	title, heading, label, header, cell, money, month, text, wrapped int
}

func newExcelStyles(f *excelize.File) (excelStyles, error) {
	thin := []excelize.Border{
		{Type: "left", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		{Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1},
	}
	font := &excelize.Font{Family: "Helvetica Neue", Size: 12}
	monthFmt, moneyFmt := "mmmm yyyy", 4 // 4 is the built-in #,##0.00
	specs := []*excelize.Style{
		{Font: &excelize.Font{Family: "Times New Roman", Size: 18}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}},
		{Font: &excelize.Font{Family: "Helvetica Neue", Size: 12, Bold: true}, Alignment: &excelize.Alignment{Horizontal: "center"}},
		{Font: font, Border: thin},
		{Font: font, Border: thin, Alignment: &excelize.Alignment{Horizontal: "center"}},
		{Border: thin},
		{Border: thin, NumFmt: moneyFmt},
		{Font: font, Border: thin, CustomNumFmt: &monthFmt, Alignment: &excelize.Alignment{Horizontal: "left"}},
		{Font: font, Border: thin},
		{Font: font, Border: thin, Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}},
	}
	ids := make([]int, len(specs))
	for i, s := range specs {
		id, err := f.NewStyle(s)
		if err != nil {
			return excelStyles{}, fmt.Errorf("new style: %w", err)
		}
		ids[i] = id
	}
	return excelStyles{ids[0], ids[1], ids[2], ids[3], ids[4], ids[5], ids[6], ids[7], ids[8]}, nil
}

// buildWorkbook renders one sheet per report, in order, into a new workbook.
// Metrics must be loaded. Every value is written as a string or a number, never
// as a formula, so report text that starts with "=" stays text.
func buildWorkbook(reps []*report.Report) (*excelize.File, error) {
	f := excelize.NewFile()
	st, err := newExcelStyles(f)
	if err != nil {
		return nil, err
	}
	for i, name := range sheetNames(reps) {
		if i == 0 {
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
	styles := []struct {
		from, to string
		id       int
	}{
		{"A1", "G2", st.title}, {"A3", "G3", st.heading},
		{"A4", "A9", st.label}, {"B5", "G7", st.text}, {"B4", "G4", st.month},
		{"B9", "G9", st.header}, {"A29", "A29", st.label}, {"B30", "G30", st.header},
		{"A37", "A42", st.wrapped}, {"B37", "G42", st.wrapped},
	}
	merges := [][2]string{
		{"A1", "G2"}, {"A3", "G3"}, {"B4", "G4"}, {"B5", "G5"}, {"B6", "G6"}, {"B7", "G7"},
		{"B37", "G39"}, {"B40", "G40"}, {"B41", "G41"}, {"B42", "G42"},
	}

	steps := []error{
		f.SetSheetProps(sh, &excelize.SheetPropsOptions{DefaultRowHeight: &height}),
		f.SetColWidth(sh, "A", "A", 26),
		f.SetColWidth(sh, "B", "G", 12.63),
		f.SetRowHeight(sh, 40, 43.5),
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
	steps = append(steps, writeMetrics(f, sh, r, st)...)
	for _, err := range steps {
		if err != nil {
			return err
		}
	}
	return nil
}

// writeMetrics styles every metric row, adds its Average formula and writes the
// reported weeks as numbers. An absent week stays blank: not reported (D-5).
func writeMetrics(f *excelize.File, sh string, r *report.Report, st excelStyles) []error {
	var steps []error
	for row := range rowLabels {
		n := strconv.Itoa(row)
		style := st.cell
		if row == metricRows["tithes_offering"] {
			style = st.money
		}
		steps = append(steps,
			f.SetCellStyle(sh, "A"+n, "A"+n, st.label),
			f.SetCellStyle(sh, "B"+n, "G"+n, style),
			f.SetCellFormula(sh, "G"+n, `IFERROR(AVERAGE(B`+n+`:F`+n+`),"")`),
		)
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

// sheetNames returns a valid, unique Excel sheet name for each report: the
// characters []:*?/\ become "-", names are cut to 31 UTF-16 units with no '
// at either end, and repeats get " (2)", " (3)"… (the base is cut so the
// suffix still fits).
func sheetNames(reps []*report.Report) []string {
	clean := strings.NewReplacer("[", "-", "]", "-", ":", "-", "*", "-", "?", "-", "/", "-", `\`, "-")
	seen := map[string]bool{}
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
