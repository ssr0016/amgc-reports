package statsimpl

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
)

// buildCSV writes one flat table as CSV for Excel: UTF-8 with a byte-order
// mark, CRLF rows, RFC 4180 quoting (commas, quotes, newlines). Numbers are
// plain (no currency sign or separators), nil is an empty cell, and a text cell
// that a spreadsheet would read as a formula gets a leading apostrophe.
func buildCSV(headers []string, rows [][]any) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\uFEFF")
	w := csv.NewWriter(&buf)
	w.UseCRLF = true
	if err := w.Write(headers); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}
	record := make([]string, len(headers))
	for _, row := range rows {
		for i := range record {
			record[i] = ""
			if i < len(row) {
				record[i] = csvCell(row[i])
			}
		}
		if err := w.Write(record); err != nil {
			return nil, fmt.Errorf("write csv row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("write csv: %w", err)
	}
	return buf.Bytes(), nil
}

func csvCell(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	case badge:
		return safeText(v.text)
	case string:
		return safeText(v)
	}
	return safeText(fmt.Sprint(v))
}

// safeText neutralises formula injection (=, +, -, @, tab, CR at the start).
func safeText(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func analyticsCSV(data stats.AttendanceAnalytics, kind string) ([]byte, error) {
	switch kind {
	case stats.CSVWeeklyDetail:
		return buildCSV(weeklyDetailHeaders, weeklyDetailRows(data))
	case stats.CSVCompliance:
		return buildCSV(complianceHeaders, complianceRows(data))
	}
	return nil, stats.ErrAnalyticsQuery
}
