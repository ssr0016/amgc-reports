package statsimpl

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"

	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
)

// The Analytics page's Excel download (D-A26, D-A31). Each sheet is one table
// under a three-row title block (its own title, the scope, the generated time
// and cutoff), with a frozen header, zebra rows and columns sized to content.
// Every figure comes from analyze, as on the page and in the CSVs.

const (
	workbookTitle = "AMGCReports — "
	headerRow     = 5 // rows 1-3 are the title block, row 4 is blank
	minTitleCols  = 6 // the title is merged across at least this many columns so it is not clipped
)

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
// pct a ratio shown as a signed percentage, a rate an unsigned one, a badge a
// colour-coded status text, and nil a blank cell.
type (
	pct   float64
	rate  float64
	tone  string
	badge struct {
		text string
		tone tone
	}
)

const (
	toneGood  tone = "good"  // approved
	toneWarn  tone = "warn"  // still needs someone: pending, needs revision, awaiting, overdue
	toneBad   tone = "bad"   // not filed, no report
	toneMuted tone = "muted" // nothing expected: not required, not reviewed, not yet open
)

// toneColors are Excel's own good/neutral/bad cell styles and a grey: {fill, font}.
var toneColors = map[tone][2]string{
	toneGood: {"C6EFCE", "006100"}, toneWarn: {"FFEB9C", "9C5700"}, toneBad: {"FFC7CE", "9C0006"}, toneMuted: {"EDEDED", "595959"},
}

type table struct {
	sheet          string
	title          string // the sheet's own title, after "AMGCReports — "
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
	status                             map[tone]int
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
	st.status = map[tone]int{}
	for s, c := range toneColors {
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
	case badge:
		return utf8.RuneCountInString(v.text)
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
	if err := writeTitle(f, st, t.sheet, max(len(t.headers), minTitleCols), workbookTitle+cmp.Or(t.title, t.sheet), subtitle, generated); err != nil {
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
func writeTitle(f *excelize.File, st analyticsStyles, sheet string, lastCol int, title, subtitle, generated string) error {
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
	}{{1, title, st.title}, {2, subtitle, st.subtitle}, {3, generated, st.generated}} {
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
			if b, ok := v.(badge); ok && st.status[b.tone] != 0 {
				style = st.status[b.tone] // a coloured status replaces the zebra fill
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
	case badge:
		return f.SetCellStr(sheet, ref, v.text)
	default:
		return f.SetCellValue(sheet, ref, v)
	}
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
