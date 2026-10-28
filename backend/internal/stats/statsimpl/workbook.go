package statsimpl

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
)

// statusLabel is the status filter as the exports name it.
func statusLabel(s string) string {
	if s == stats.AttendanceStatusAll {
		return "All submitted reports (pending, needs revision, approved)"
	}
	return "Approved reports only"
}

// scopeLine names the year, period, churches and status filter.
func scopeLine(data stats.AttendanceAnalytics) string {
	churches := fmt.Sprintf("All %d churches", len(data.Churches))
	if inactive := inactiveCount(data.Churches); inactive > 0 {
		churches += fmt.Sprintf(" (%d with an active account, %d inactive with reports or an end month)", len(data.Churches)-inactive, inactive)
	}
	if len(data.Churches) < len(data.ChurchOptions) {
		names := make([]string, 0, len(data.Churches))
		for _, c := range data.Churches {
			names = append(names, c.Name)
		}
		churches = fmt.Sprintf("%d of %d churches: %s", len(data.Churches), len(data.ChurchOptions), strings.Join(names, ", "))
	}
	return fmt.Sprintf("%s %d · %s · %s", periodLabel(data.Period), data.Year, churches, statusLabel(data.Status))
}

func inactiveCount(churches []stats.ChurchScope) int {
	n := 0
	for _, c := range churches {
		if !c.Active {
			n++
		}
	}
	return n
}

// startText is a church's start month, marked when it is the organization default.
func startText(c stats.ChurchScope) string {
	if c.StartIsDefault {
		return c.StartMonth + " (default)"
	}
	return orDash(c.StartMonth)
}

// cutoffLine is the generated time in Manila and the months the figures count.
func cutoffLine(data stats.AttendanceAnalytics) string {
	generated := "Generated " + data.GeneratedAt.Format("2 Jan 2006, 15:04") + " (Asia/Manila)"
	if data.CutoffMonth == 0 {
		return generated + " · No month of this period has ended: nothing is counted yet."
	}
	start, end, _ := stats.AnalyticsPeriodBounds(data.Period)
	through := fmt.Sprintf("Counts activity months %s–%s %d", report.Months[start-1], report.Months[data.CutoffMonth-1], data.Year)
	if data.CutoffMonth < end {
		through += " (year to date; later months have not ended)"
	}
	return generated + " · " + through
}

// opt is a nullable number as a cell: blank when nil.
func opt(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func optPct(v *float64) any {
	if v == nil {
		return "n/a"
	}
	return pct(*v)
}

func ratio(n, d int) any {
	if d == 0 {
		return nil
	}
	return rate(float64(n) / float64(d))
}

func unitLabel(unit string) string {
	return map[string]string{
		stats.UnitPeople: "attendance entries", stats.UnitCurrency: "PHP", stats.UnitCount: "count",
		stats.UnitAsReported: "as reported (participants or events)",
	}[unit]
}

// cellLabel is a compliance cell as text, with its colour.
func cellLabel(c stats.ComplianceCell) badge {
	switch c.State {
	case stats.CellSubmitted:
		review := map[string]string{"approved": "Approved", "needs_revision": "Needs Revision"}[c.ReviewStatus]
		if review == "" {
			review = "Pending"
		}
		when := map[string]string{
			stats.TimelyOnTime: "on time", stats.TimelyLate: "late",
			stats.TimelyAfterCutoff: "after cutoff", stats.TimelyBeforeOpen: "before month end",
		}[c.Timeliness]
		t := toneWarn
		if c.ReviewStatus == stats.AttendanceStatusApproved {
			t = toneGood
		}
		return badge{review + " · " + when, t}
	case stats.CellNotRequired:
		return badge{"Not Required", toneMuted}
	case stats.CellNotReviewed:
		return badge{"Start Not Set", toneMuted}
	case stats.CellInactive:
		return badge{"Church Inactive", toneMuted}
	case stats.CellNotYetOpen:
		return badge{"Not Yet Open", toneMuted}
	case stats.CellAwaiting:
		return badge{"Awaiting Submission", toneWarn}
	case stats.CellOverdue:
		return badge{"Overdue — Grace Period", toneWarn}
	}
	return badge{"Not Filed", toneBad}
}

// countedReport reports whether r is in the figures: the canonical report of its
// church-month, matching the status filter, in a counted month.
func countedReport(data stats.AttendanceAnalytics, r stats.ReportData) bool {
	return r.Canonical && r.Year == data.Year && r.Month <= data.CutoffMonth &&
		(data.Status == stats.AttendanceStatusAll || r.Status == stats.AttendanceStatusApproved)
}

func buildAnalyticsWorkbook(data stats.AttendanceAnalytics) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	st, err := newAnalyticsStyles(f)
	if err != nil {
		return nil, err
	}
	subtitle, generated := scopeLine(data), cutoffLine(data)
	for _, t := range []table{
		allMetricsTable(data), summaryTable(data), monthlyTable(data), trendTable(data), churchTable(data),
		detailTable(data), weeklyDetailTable(data), complianceTable(data), notesTable(),
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

// allMetricsTable is every metric's period figures, overall and per church.
// A church cell is the church's period total (0 included), "—" when it reported
// but left the metric blank, or No Report when it has no report in scope.
func allMetricsTable(data stats.AttendanceAnalytics) table {
	t := table{sheet: "Full Metrics Summary", title: "Full Metrics Summary: period totals by church",
		headers:        []string{"Metric", "Category", "Unit", "Total for Period", "Recorded Entries", "Avg per Church-Service", "Network Weekly Avg"},
		freezeFirstCol: true, printReport: true}
	for _, c := range data.Churches {
		t.headers = append(t.headers, c.Name)
	}
	for _, m := range data.Metrics {
		row := []any{m.Label, m.Group, unitLabel(m.Unit), opt(m.Total), m.Observations, opt(m.AvgPerChurchService), opt(m.NetworkWeekly)}
		for _, c := range m.Churches {
			switch c.State {
			case "value":
				row = append(row, opt(c.Value))
			case "blank":
				row = append(row, "—")
			case "not_approved":
				row = append(row, badge{"Not Approved", toneWarn})
			default:
				row = append(row, badge{"No Report", toneBad})
			}
		}
		t.rows = append(t.rows, row)
	}
	return t
}

func metricOf(metrics []stats.MetricSummary, key string) stats.MetricSummary {
	for _, m := range metrics {
		if m.Metric == key {
			return m
		}
	}
	return stats.MetricSummary{Metric: key, Label: metricLabel(key)}
}

// summaryTable is the page's KPIs with the denominator each one used.
func summaryTable(data stats.AttendanceAnalytics) table {
	t := table{sheet: "Summary", title: "Summary: KPIs, coverage and submission completeness", headers: []string{"KPI", "Value", "Basis"}}
	cmpData := data.Comparison
	if cmpData != nil {
		t.headers = append(t.headers, strconv.Itoa(cmpData.Year), "Change")
	}
	prev := func(key string) stats.MetricSummary {
		if cmpData == nil {
			return stats.MetricSummary{}
		}
		return metricOf(cmpData.Metrics, key)
	}
	add := func(label string, value any, basis string, previous any, ch *float64) {
		row := []any{label, value, basis}
		if cmpData != nil {
			row = append(row, previous, optPct(ch))
		}
		t.rows = append(t.rows, row)
	}
	w, pw := metricOf(data.Metrics, "worship_service"), prev("worship_service")
	add("Worship attendance entries", opt(w.Total), fmt.Sprintf("Sum of %d recorded church-services; not unique persons", w.Observations), opt(pw.Total), nil)
	add("Avg worship attendance per church-service", opt(w.AvgPerChurchService), fmt.Sprintf("%v ÷ %d recorded church-services", fmtOpt(w.Total), w.Observations), opt(pw.AvgPerChurchService), change(w.AvgPerChurchService, pw.AvgPerChurchService))
	add("Network weekly worship attendance (avg)", opt(w.NetworkWeekly), fmt.Sprintf("Average of %d reporting-week slot totals; %d with every expected church", w.NetworkSlots, w.NetworkSlotsFull), opt(pw.NetworkWeekly), change(w.NetworkWeekly, pw.NetworkWeekly))
	for _, key := range []string{"tithes_offering", "person_led_to_christ", "baptism", "person_newly_contacted", "person_followed_up", "person_evangelized"} {
		m, p := metricOf(data.Metrics, key), prev(key)
		if key == report.LegacyMetric && m.Total == nil && p.Total == nil {
			continue
		}
		add(m.Label, opt(m.Total), fmt.Sprintf("Sum of %d recorded weekly values from %d reports", m.Observations, m.ReportsWithValue), opt(p.Total), change(m.Total, p.Total))
	}
	tithes := metricOf(data.Metrics, "tithes_offering")
	add("Churches with recorded offering", tithes.ChurchesWithValue, fmt.Sprintf("of %d churches in scope; not an activity status", len(data.Churches)), prev("tithes_offering").ChurchesWithValue, nil)
	cov := data.Coverage
	var prevCov any
	if cmpData != nil {
		prevCov = fmt.Sprintf("%d of %d", cmpData.Coverage.ExpectedReported, cmpData.Coverage.Expected)
	}
	add("Expected reports counted (status filter)", fmt.Sprintf("%d of %d", cov.ExpectedReported, cov.Expected),
		fmt.Sprintf("Church-months owed over %d counted months; %d more reports are from months not required and still count in totals", cov.Months, cov.OutsideReports), prevCov, nil)
	ct := data.ComplianceTotals
	add("Submitted / expected", fmt.Sprintf("%d of %d", ct.Submitted, ct.Expected), fmt.Sprintf("Every status; owed months of the period whose window has opened; %d reports from months not required", ct.Outside), nil, nil)
	add("Approved / expected", fmt.Sprintf("%d of %d", ct.Approved, ct.Expected), "", nil, nil)
	add("Pending review", ct.Pending+ct.NeedsRevision, fmt.Sprintf("%d pending, %d needs revision", ct.Pending, ct.NeedsRevision), nil, nil)
	add("Submitted on time / late", fmt.Sprintf("%d / %d", ct.OnTime, ct.Late), "On time: by the 15th of the next month; late: by its last day", nil, nil)
	add("Awaiting / overdue (grace) / not filed", fmt.Sprintf("%d / %d / %d", ct.Awaiting, ct.Overdue, ct.NotFiled), "", nil, nil)
	add("Churches with a submission", ct.ChurchesWithSubmission, fmt.Sprintf("of %d churches in scope", len(data.Churches)), nil, nil)
	add("Churches without a reviewed start month", ct.ChurchesNeedingReview, "Nothing is counted as missing for them until an admin sets it", nil, nil)
	if cmpData != nil {
		l := cmpData.LikeForLike
		add("Like-for-like "+metricLabel(data.Metric), l.CurrentTotal, fmt.Sprintf("%d churches with values in both years; %d vs %d recorded entries", l.Churches, l.CurrentObservations, l.PreviousObservations), l.PreviousTotal, l.Change)
	}
	return t
}

func fmtOpt(v *float64) string {
	if v == nil {
		return "no data"
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

// monthlyTable is the selected metric per month with its coverage.
func monthlyTable(data stats.AttendanceAnalytics) table {
	label := metricLabel(data.Metric)
	t := table{sheet: "Monthly Totals", title: "Monthly Totals: " + label,
		headers: []string{"Month", "Reporting State", "Total", "Recorded Entries", "Churches with Value", "Churches Reported", "Expected Churches Reported", "Churches Expected", "Coverage"}}
	att := isAttendance(data.Metric)
	if att {
		t.headers = append(t.headers, "Avg per Church-Service")
	}
	cmpData := data.Comparison
	if cmpData != nil {
		py := strconv.Itoa(cmpData.Year)
		t.headers = append(t.headers, py+" Total", py+" Recorded Entries", py+" Churches Reported", "Change")
	}
	for i, p := range data.Trend {
		state := p.State
		if !p.Counted {
			state += " (not counted)"
		}
		row := []any{p.Name, state, opt(p.Value), p.Observations, p.ChurchesWithValue, p.ChurchesReported, p.ExpectedReported, p.ChurchesExpected, ratio(p.ExpectedReported, p.ChurchesExpected)}
		if att {
			row = append(row, opt(avgOf(p.Value, p.Observations)))
		}
		if cmpData != nil {
			pp := cmpData.Trend[i]
			basis, prevBasis := p.Value, pp.Value
			if att {
				basis, prevBasis = avgOf(p.Value, p.Observations), avgOf(pp.Value, pp.Observations)
			}
			row = append(row, opt(pp.Value), pp.Observations, pp.ChurchesReported, optPct(change(basis, prevBasis)))
		}
		t.rows = append(t.rows, row)
	}
	return t
}

func avgOf(total *float64, n int) *float64 {
	if total == nil || n == 0 {
		return nil
	}
	v := *total / float64(n)
	return &v
}

// trendTable is the network total per reporting-week slot.
func trendTable(data stats.AttendanceAnalytics) table {
	t := table{sheet: "Weekly Trend", title: "Weekly Trend: " + metricLabel(data.Metric) + " by reporting-week slot",
		headers: []string{"Month", "Week Slot", "Network Total", "Churches with Value", "Churches Reported"}}
	cmpData := data.Comparison
	prev := map[[2]int]stats.SlotPoint{}
	if cmpData != nil {
		py := strconv.Itoa(cmpData.Year)
		t.headers = append(t.headers, py+" Network Total", py+" Churches with Value")
		for _, s := range cmpData.Slots {
			prev[[2]int{s.Month, s.Week}] = s
		}
	}
	for _, s := range data.Slots {
		row := []any{report.Months[s.Month-1], "Week " + strconv.Itoa(s.Week), opt(s.Total), s.ChurchesWithValue, s.ChurchesReported}
		if cmpData != nil {
			p := prev[[2]int{s.Month, s.Week}]
			row = append(row, opt(p.Total), p.ChurchesWithValue)
		}
		t.rows = append(t.rows, row)
	}
	return t
}

// churchTable is the church comparison: one row per church in scope.
func churchTable(data stats.AttendanceAnalytics) table {
	label := metricLabel(data.Metric)
	t := table{sheet: "Church Breakdown", title: "Church Breakdown: " + label + " and submission completeness", filter: true, freezeFirstCol: true,
		headers: []string{"Church", label + " Total", "Recorded Entries", "Average per Service"}}
	cmpData := data.Comparison
	if cmpData != nil {
		py := strconv.Itoa(cmpData.Year)
		t.headers = append(t.headers, py+" Total", py+" Recorded Entries", py+" Average", "Change")
	}
	t.headers = append(t.headers, "Tithes & Offering", "Submitted", "Approved", "Expected", "Submission Completeness")
	for _, r := range data.ChurchRows {
		row := []any{r.ChurchName, opt(r.Total), r.Observations, opt(r.Average)}
		if cmpData != nil {
			row = append(row, opt(r.PreviousTotal), r.PreviousObs, opt(r.PreviousAverage), optPct(r.Change))
		}
		t.rows = append(t.rows, append(row, opt(r.TithesTotal), r.Submitted, r.Approved, r.Expected, opt(r.Completeness)))
	}
	return t
}

// detailTable is every metric's month total for every counted church-month
// report: a metric left blank says so, rather than 0.
func detailTable(data stats.AttendanceAnalytics) table {
	t := table{sheet: "Metric Detail", title: "Metric Detail: month totals per church", filter: true,
		headers: []string{"Metric", "Church", "Month", "Total", "Recorded Entries", "Value State", "Report ID", "Report Status"}}
	for _, key := range report.MetricKeys {
		for _, r := range data.Reports {
			if !countedReport(data, r) {
				continue
			}
			var a acc
			if weeks := r.Metrics[key]; weeks != nil {
				for _, v := range weeks {
					if v != nil {
						a.add(*v)
					}
				}
			}
			if key == report.LegacyMetric && a.obs == 0 {
				continue
			}
			state := "reported"
			if a.obs == 0 {
				state = "blank"
			}
			t.rows = append(t.rows, []any{metricLabel(key), r.ChurchName, report.Months[r.Month-1], opt(a.total()), a.obs, state, int(r.ID), r.Status})
		}
	}
	return t
}

// weeklyDetailHeaders and weeklyDetailRows are the raw export: one row per
// report, metric and week slot, for every report of the period and churches,
// whatever its status. A report with no value in a slot gives a blank row; the
// legacy metric only where the report has it. Missing reports are in compliance.
var weeklyDetailHeaders = []string{"Church ID", "Church", "Worker", "Reporting Year", "Reporting Month", "Week Slot",
	"Report ID", "Report Status", "Counted Report", "Metric Code", "Metric", "Category", "Unit", "Value", "Value State", "Included in Selected Analytics"}

func weeklyDetailRows(data stats.AttendanceAnalytics) [][]any {
	var rows [][]any
	for _, r := range data.Reports {
		included := countedReport(data, r)
		for _, key := range report.MetricKeys {
			weeks := r.Metrics[key]
			if key == report.LegacyMetric && weeks == nil {
				continue
			}
			info := metricInfos[key]
			for w := range 5 {
				var value any
				state := "blank"
				if weeks != nil && weeks[w] != nil {
					value, state = *weeks[w], "reported"
				}
				rows = append(rows, []any{int(r.ChurchID), r.ChurchName, r.Worker, r.Year, report.Months[r.Month-1], w + 1,
					int(r.ID), r.Status, r.Canonical, key, metricLabel(key), info.group, info.unit, value, state, included})
			}
		}
	}
	return rows
}

func weeklyDetailTable(data stats.AttendanceAnalytics) table {
	rows := weeklyDetailRows(data)
	for _, row := range rows {
		for i, v := range row {
			if b, ok := v.(bool); ok {
				row[i] = strconv.FormatBool(b)
			}
		}
	}
	return table{sheet: "Weekly Detail", title: "Weekly Detail: every reported week slot", filter: true, headers: weeklyDetailHeaders, rows: rows}
}

// complianceHeaders and complianceRows are the flat compliance export: one row per church and month.
var complianceHeaders = []string{"Church ID", "Church", "Church Active", "Reporting Start Month", "Start Month Source", "Reporting End Month", "Reporting Year", "Reporting Month",
	"Month State", "Report Owed", "Submission State", "Report ID", "Review Status", "Timeliness", "Submitted On", "Other Reports Same Month"}

func complianceRows(data stats.AttendanceAnalytics) [][]any {
	starts := map[int64]stats.ChurchScope{}
	for _, c := range data.Churches {
		starts[c.ID] = c
	}
	var rows [][]any
	for _, r := range data.Compliance {
		c := starts[r.ChurchID]
		for i, cell := range r.Cells {
			var id any
			if cell.ReportID != 0 {
				id = int(cell.ReportID)
			}
			source := "church override"
			switch {
			case c.StartIsDefault:
				source = "organization default"
			case c.StartMonth == "":
				source = "not set"
			}
			rows = append(rows, []any{int(r.ChurchID), r.ChurchName, c.Active, c.StartMonth, source, c.EndMonth, data.Year, report.Months[cell.Month-1],
				data.Months[i].State, cell.Expected, cell.State, id, cell.ReviewStatus, cell.Timeliness, cell.SubmittedOn, cell.Duplicates})
		}
	}
	return rows
}

// complianceTable is the church-by-month matrix.
func complianceTable(data stats.AttendanceAnalytics) table {
	t := table{sheet: "Submission Compliance", title: "Submission Compliance: every status, regardless of the status filter", freezeFirstCol: true,
		headers: []string{"Church", "Start Month", "End Month"}}
	for _, m := range data.Months {
		t.headers = append(t.headers, m.Name[:3])
	}
	t.headers = append(t.headers, "Expected", "Submitted", "Approved", "On Time", "Late", "Awaiting", "Overdue", "Not Filed", "Submission Rate", "Filed, Not Required")
	starts := map[int64]stats.ChurchScope{}
	for _, c := range data.Churches {
		starts[c.ID] = c
	}
	for _, r := range data.Compliance {
		c := starts[r.ChurchID]
		row := []any{r.ChurchName, startText(c), orDash(c.EndMonth)}
		for _, cell := range r.Cells {
			row = append(row, cellLabel(cell))
		}
		t.rows = append(t.rows, append(row, r.Expected, r.Submitted, r.Approved, r.OnTime, r.Late, r.Awaiting, r.Overdue, r.NotFiled, ratio(r.Submitted, r.Expected), r.Outside))
	}
	return t
}

// notesTable explains the figures, so a reader of the file needs no page.
func notesTable() table {
	t := table{sheet: "Notes", title: "Notes: definitions and rules", headers: []string{"Term", "Definition"}}
	for _, n := range [][2]string{
		{"Recorded entry", "One church's value for one metric in one reporting-week slot of a counted report. 0 is a recorded entry; a blank week is not."},
		{"Attendance entries", "The sum of recorded attendance values. Not unique persons and not membership."},
		{"Avg per church-service", "Attendance entries ÷ recorded entries of that activity, over every church in scope. Each activity uses its own count; a blank week is left out (no 'not held' state is recorded)."},
		{"Average per service (church)", "One church's attendance entries ÷ its recorded entries of that activity."},
		{"Network weekly avg", "For each month and week slot, the sum over churches with a value; then the average of those sums over slots with any value. 'Full' slots have a value from every expected church."},
		{"Week slot", "Week 1-5 of the report form. Slots are not calendar dates and need not match across churches."},
		{"Counted months", "Months of the period that have ended (Asia/Manila). The previous year counts the same months."},
		{"Counted report", "One per church and month: the approved one, else the latest updated. Others are listed in Weekly Detail as not counted."},
		{"Status filter", "Approved counts approved reports only; All Submitted counts pending, needs revision and approved. Compliance ignores it."},
		{"Value states", "A number (0 included) is reported; — is a report that left the metric blank; Not Approved is a church whose reports are still pending under the Approved filter (not missing); No Report is a church with no report in the counted months."},
		{"Reporting window", "A month's report opens when the month ends, is due on the 15th of the next month, may be filed late until that month's last day, and then closes."},
		{"Reporting months", "Every church is expected from the organization-wide start month (marked default) unless it has its own start or end month. A church listed as inactive has no active church account; without an end month none of its months is expected."},
		{"Expected and coverage", "Expected counts only the months a church owed whose window has opened. A report for a month not required (before the start month, after the end month, or for an inactive church) is kept and counts in every total, but is reported separately and never raises coverage."},
		{"Compliance states", "Not Required: outside the church's reporting months. Church Inactive: no active account and no end month. Start Not Set: no organization default and no override, so nothing is missing. Not Yet Open, Awaiting Submission, Overdue — Grace Period, Not Filed."},
		{"Timeliness", "From when the report was submitted: on time by the 15th, late by the month's end, after cutoff or before month end only for admin entries."},
		{"Change", "Shown only when both periods have data and the previous is above 0; otherwise n/a. Attendance compares averages per church-service."},
		{"Like-for-like", "The selected metric over churches with values in both years only."},
		{"Activities", "Outreach, Training/Seminars, Leadership, Family Day and Other may count participants or events; they are shown as reported, with no averages."},
		{"Legacy Persons Evangelized", "The old person_evangelized field. Its meaning is unverified: it is not counted as Persons Led to Christ."},
		{"Churches with recorded offering", "Churches with any Tithes & Offering value. It says nothing about whether a church is active."},
	} {
		t.rows = append(t.rows, []any{n[0], n[1]})
	}
	return t
}

// monthStart parses "YYYY-MM" as its first day in loc; "" is nil.
func monthStart(s string, loc *time.Location) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01", s, loc)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
