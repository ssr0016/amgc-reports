package statsimpl

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

// The analytics rules (D-A31), computed once from the weekly report rows so the
// page, the workbook and the CSVs cannot disagree:
//   - A value is a recorded (church, report, metric, week slot). A blank week is no
//     value: it is never 0, and never in a denominator. 0 is a value.
//   - One report counts per church and month: the approved one, else the latest updated.
//   - Only months that have ended count (year to date); the previous year uses the same months.
//   - Sums are taken before dividing; averages are never averaged.

// monthBounds are the instants a month's report opens, falls due (after the
// 15th) and closes (after the next month's last day), in loc.
func monthBounds(year int, m time.Month, loc *time.Location) (open, due, closed time.Time) {
	open = time.Date(year, m+1, 1, 0, 0, 0, 0, loc)
	return open, open.AddDate(0, 0, 15), open.AddDate(0, 1, 0)
}

// monthState is the reporting state of the activity month at now.
func monthState(year int, m time.Month, now time.Time) string {
	open, due, closed := monthBounds(year, m, now.Location())
	switch {
	case now.Before(open):
		return stats.MonthNotYetOpen
	case now.Before(due):
		return stats.MonthOpen
	case now.Before(closed):
		return stats.MonthGrace
	}
	return stats.MonthClosed
}

// timeliness classes a report by when it was created (submitted): drafts were
// removed (D-A25), so creation is submission.
func timeliness(year int, m time.Month, created time.Time, loc *time.Location) string {
	open, due, closed := monthBounds(year, m, loc)
	c := created.In(loc)
	switch {
	case c.Before(open):
		return stats.TimelyBeforeOpen
	case c.Before(due):
		return stats.TimelyOnTime
	case c.Before(closed):
		return stats.TimelyLate
	}
	return stats.TimelyAfterCutoff
}

type metricInfo struct{ group, unit string }

var metricInfos = func() map[string]metricInfo {
	out := map[string]metricInfo{}
	for _, g := range stats.MetricGroups {
		for _, m := range g.Metrics {
			out[m] = metricInfo{g.Name, g.Unit}
		}
	}
	return out
}()

func isAttendance(metric string) bool { return metricInfos[metric].unit == stats.UnitPeople }

func metricLabel(key string) string {
	if l, ok := stats.MetricLabels[key]; ok {
		return l
	}
	return key
}

func ym(year int, m int) int { return year*12 + m - 1 }

func monthKey(t *time.Time) int {
	if t == nil {
		return 0
	}
	return ym(t.Year(), int(t.Month()))
}

func monthText(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01")
}

// church is a church in scope with its eligibility.
type church struct {
	stats.ChurchScope
	start, end int // ym of the eligible months; start 0 is not set, end 0 open-ended
}

// eligible reports whether the church is expected to report the month: within its
// months, and active or with an end month (an inactive church with no end month
// has an unknown end, so none of its months is owed).
func (c church) eligible(year, month int) bool {
	k := ym(year, month)
	return c.start != 0 && k >= c.start && (c.end == 0 || k <= c.end) && (c.Active || c.end != 0)
}

type churchMonth struct {
	church      int64
	year, month int
}

// analysis is what analyze needs: the query (validated), now in Manila, every
// church, and the reports of the year (and the previous one when comparing).
type analysis struct {
	q        stats.AttendanceQuery
	now      time.Time
	churches []stats.ChurchRecord
	reports  []stats.ReportData
	// defaultStart is the organization-wide first reporting month; nil when not configured.
	defaultStart *time.Time
}

func analyze(in analysis) stats.AttendanceAnalytics {
	q, now := in.q, in.now
	out := stats.AttendanceAnalytics{Year: q.Year, Metric: q.Metric, Status: q.Status, Period: q.Period, GeneratedAt: now}
	counted := periodMonths(&out, now)
	reports := markCanonical(in.reports)
	out.DefaultStartMonth = monthText(in.defaultStart)
	scope := scopeChurches(&out, churchList(in.churches, reports, in.defaultStart), q.ChurchIDs)
	byMonth, dups := indexReports(&out, reports)

	included := func(r *stats.ReportData) bool {
		return r != nil && (q.Status == stats.AttendanceStatusAll || r.Status == stats.AttendanceStatusApproved)
	}
	current := yearFigures(q.Year, counted, scope, byMonth, included, q.Metric)
	out.Coverage, out.Metrics, out.Slots = current.coverage, current.metrics, current.slots
	out.Trend = trendPoints(out.Months, current, q.Metric)
	out.Compliance, out.ComplianceTotals = compliance(q.Year, out.Months, scope, byMonth, dups, now)

	var previous *figures
	if q.Compare {
		p := yearFigures(q.Year-1, counted, scope, byMonth, included, q.Metric)
		previous = &p
		prevMonths := make([]stats.AnalyticsMonth, 0, len(out.Months))
		for _, m := range out.Months {
			prevMonths = append(prevMonths, stats.AnalyticsMonth{Month: m.Month, Name: m.Name, State: monthState(q.Year-1, time.Month(m.Month), now)})
		}
		out.Comparison = &stats.Comparison{
			Year: q.Year - 1, Coverage: p.coverage, Metrics: p.metrics, Slots: p.slots,
			Trend:       trendPoints(prevMonths, p, q.Metric),
			LikeForLike: likeForLike(current, p, q.Metric),
		}
	}
	out.ChurchRows = churchRows(scope, q.Metric, current, previous, out.Compliance)
	return out
}

// periodMonths fills the period's months and cutoff and returns the counted
// months: those of the period that have ended.
func periodMonths(out *stats.AttendanceAnalytics, now time.Time) []int {
	startMonth, endMonth, _ := stats.AnalyticsPeriodBounds(out.Period)
	var counted []int
	for m := startMonth; m <= endMonth; m++ {
		state := monthState(out.Year, time.Month(m), now)
		out.Months = append(out.Months, stats.AnalyticsMonth{Month: m, Name: report.Months[m-1], State: state})
		if state != stats.MonthNotYetOpen {
			out.CutoffMonth = m
			counted = append(counted, m)
		}
	}
	return counted
}

// scopeChurches fills the filter options and the churches in scope, and returns the latter.
func scopeChurches(out *stats.AttendanceAnalytics, all []church, ids []int64) []church {
	scope := all
	if len(ids) > 0 {
		scope = slices.DeleteFunc(slices.Clone(all), func(c church) bool { return !slices.Contains(ids, c.ID) })
	}
	for _, c := range all {
		out.ChurchOptions = append(out.ChurchOptions, stats.ChurchOption{ID: c.ID, Name: c.Name})
	}
	out.Churches = make([]stats.ChurchScope, 0, len(scope))
	for _, c := range scope {
		out.Churches = append(out.Churches, c.ChurchScope)
	}
	return scope
}

// indexReports keeps the in-scope churches' reports: the year's in out.Reports,
// each church-month's canonical report (any status) and its count of others.
func indexReports(out *stats.AttendanceAnalytics, reports []stats.ReportData) (map[churchMonth]*stats.ReportData, map[churchMonth]int) {
	inScope := map[int64]bool{}
	for _, c := range out.Churches {
		inScope[c.ID] = true
	}
	byMonth, dups := map[churchMonth]*stats.ReportData{}, map[churchMonth]int{}
	out.Reports = []stats.ReportData{}
	for i := range reports {
		r := &reports[i]
		if !inScope[r.ChurchID] {
			continue
		}
		k := churchMonth{r.ChurchID, r.Year, r.Month}
		if r.Canonical {
			byMonth[k] = r
		} else {
			dups[k]++
		}
		if r.Year == out.Year {
			out.Reports = append(out.Reports, *r)
		}
	}
	slices.SortFunc(out.Reports, func(a, b stats.ReportData) int {
		return cmp.Or(strings.Compare(strings.ToLower(a.ChurchName), strings.ToLower(b.ChurchName)), cmp.Compare(a.Month, b.Month), cmp.Compare(a.ID, b.ID))
	})
	return byMonth, dups
}

// churchRows is the church comparison for metric: one row per church in scope.
func churchRows(scope []church, metric string, current figures, previous *figures, comp []stats.ComplianceRow) []stats.ChurchMetric {
	byChurch := map[int64]stats.ComplianceRow{}
	for _, row := range comp {
		byChurch[row.ChurchID] = row
	}
	att := isAttendance(metric)
	rows := make([]stats.ChurchMetric, 0, len(scope))
	for _, c := range scope {
		cur := current.byChurch[metric][c.ID]
		row := stats.ChurchMetric{ChurchID: c.ID, ChurchName: c.Name, Total: cur.total(), Observations: cur.obs,
			TithesTotal: current.byChurch["tithes_offering"][c.ID].total()}
		if att {
			row.Average = cur.average()
		}
		if previous != nil {
			prev := previous.byChurch[metric][c.ID]
			row.PreviousTotal, row.PreviousObs = prev.total(), prev.obs
			row.Change = change(row.Total, row.PreviousTotal)
			if att {
				row.PreviousAverage = prev.average()
				row.Change = change(row.Average, row.PreviousAverage)
			}
		}
		cr := byChurch[c.ID]
		row.Submitted, row.Approved, row.Expected = cr.Submitted, cr.Approved, cr.Expected
		if cr.Expected > 0 {
			v := float64(cr.Submitted) / float64(cr.Expected)
			row.Completeness = &v
		}
		rows = append(rows, row)
	}
	return rows
}

// change is the relative change from previous to current, or nil when either
// has no data or previous is 0 (a percentage from 0 means nothing).
func change(current, previous *float64) *float64 {
	if current == nil || previous == nil || *previous == 0 {
		return nil
	}
	v := (*current - *previous) / *previous
	return &v
}

// markCanonical returns reports with Canonical set: per church and month, the
// approved report, else the latest updated, else the highest id.
func markCanonical(reports []stats.ReportData) []stats.ReportData {
	out := slices.Clone(reports)
	best := map[churchMonth]int{}
	for i, r := range out {
		k := churchMonth{r.ChurchID, r.Year, r.Month}
		j, ok := best[k]
		if !ok || better(r, out[j]) {
			best[k] = i
		}
	}
	for _, i := range best {
		out[i].Canonical = true
	}
	return out
}

func better(a, b stats.ReportData) bool {
	aa, ba := a.Status == stats.AttendanceStatusApproved, b.Status == stats.AttendanceStatusApproved
	if aa != ba {
		return aa
	}
	if !a.UpdatedAt.Equal(b.UpdatedAt) {
		return a.UpdatedAt.After(b.UpdatedAt)
	}
	return a.ID > b.ID
}

// churchList is every church with an active church account (as the admin cards
// count them), plus inactive churches with reports read or an end month. A church
// without its own start month uses defaultStart. A report whose church has no row
// (none is expected) is grouped under id 0.
func churchList(records []stats.ChurchRecord, reports []stats.ReportData, defaultStart *time.Time) []church {
	withReports := map[int64]string{}
	for _, r := range reports {
		withReports[r.ChurchID] = r.ChurchName
	}
	var out []church
	for _, rec := range records {
		var workers []string
		status, active := "none", false
		for _, a := range rec.Accounts {
			if a.Deleted {
				continue
			}
			workers = append(workers, a.Name)
			if status == "none" || a.Status == user.UserStatusActive {
				status = a.Status
			}
			active = active || a.Status == user.UserStatusActive
		}
		if _, ok := withReports[rec.ID]; !ok && !active && rec.EndMonth == nil {
			continue
		}
		start := rec.StartMonth
		if start == nil {
			start = defaultStart
		}
		out = append(out, church{
			ChurchScope: stats.ChurchScope{ID: rec.ID, Name: rec.Name, StartMonth: monthText(start), StartIsDefault: rec.StartMonth == nil && start != nil,
				EndMonth: monthText(rec.EndMonth), NeedsReview: start == nil, Active: active, AccountStatus: status, Workers: strings.Join(workers, ", ")},
			start: monthKey(start), end: monthKey(rec.EndMonth),
		})
	}
	if name, ok := withReports[0]; ok {
		out = append(out, church{ChurchScope: stats.ChurchScope{Name: name, NeedsReview: true, AccountStatus: "none"}})
	}
	slices.SortFunc(out, func(a, b church) int {
		return cmp.Or(strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.ID, b.ID))
	})
	return out
}

// acc sums recorded values.
type acc struct {
	sum      float64
	obs      int
	zeros    int
	hasValue bool
}

func (a *acc) add(v float64) {
	a.sum += v
	a.obs++
	a.hasValue = true
	if v == 0 {
		a.zeros++
	}
}

func (a acc) total() *float64 {
	if a.obs == 0 {
		return nil
	}
	v := a.sum
	return &v
}

func (a acc) average() *float64 {
	if a.obs == 0 {
		return nil
	}
	v := a.sum / float64(a.obs)
	return &v
}

type slotKey struct{ month, week int }

// figures is one year's numbers over the counted months.
type figures struct {
	coverage stats.Coverage
	metrics  []stats.MetricSummary
	slots    []stats.SlotPoint
	byChurch map[string]map[int64]acc
	// byMonth is per metric and month; reported and expected are per month.
	byMonth           map[string]map[int]acc
	churchesWithValue map[string]map[int]map[int64]bool
	reported          map[int]int
	expected          map[int]int
	expectedReported  map[int]int
	bySlot            map[string]map[slotKey]acc
	slotChurches      map[string]map[slotKey]int
	reportsWithValue  map[string]int
	churchReported    map[int64]bool // a counted report in scope of the status filter
	churchFiled       map[int64]bool // any counted report, whatever its status
}

func yearFigures(year int, counted []int, scope []church, reports map[churchMonth]*stats.ReportData, included func(*stats.ReportData) bool, selected string) figures {
	f := figures{
		byChurch: map[string]map[int64]acc{}, byMonth: map[string]map[int]acc{},
		churchesWithValue: map[string]map[int]map[int64]bool{}, reported: map[int]int{}, expected: map[int]int{}, expectedReported: map[int]int{},
		bySlot: map[string]map[slotKey]acc{}, slotChurches: map[string]map[slotKey]int{}, reportsWithValue: map[string]int{},
		churchReported: map[int64]bool{}, churchFiled: map[int64]bool{},
	}
	f.coverage.Months, f.coverage.Churches = len(counted), len(scope)
	for _, m := range counted {
		for _, c := range scope {
			r := reports[churchMonth{c.ID, year, m}]
			owed := c.eligible(year, m)
			if owed {
				f.expected[m]++
				f.coverage.Expected++
			}
			if r != nil {
				f.churchFiled[c.ID] = true
			}
			if !included(r) {
				continue
			}
			f.addReport(c.ID, m, r)
			if owed {
				f.expectedReported[m]++
				f.coverage.ExpectedReported++
			} else {
				f.coverage.OutsideReports++
			}
		}
	}
	f.coverage.ChurchesReported = len(f.churchReported)
	for _, g := range stats.MetricGroups {
		for _, metric := range g.Metrics {
			f.metrics = append(f.metrics, f.summary(g, metric, counted, scope))
		}
	}
	for _, m := range counted {
		for w := 1; w <= 5; w++ {
			k := slotKey{m, w}
			f.slots = append(f.slots, stats.SlotPoint{Month: m, Week: w, Total: f.bySlot[selected][k].total(),
				ChurchesWithValue: f.slotChurches[selected][k], ChurchesReported: f.reported[m]})
		}
	}
	return f
}

// addReport adds one counted report's recorded values.
func (f *figures) addReport(church int64, m int, r *stats.ReportData) {
	f.reported[m]++
	f.coverage.ReportsInScope++
	f.churchReported[church] = true
	for metric, weeks := range r.Metrics {
		recorded := false
		for w, v := range weeks {
			if v == nil {
				continue
			}
			recorded = true
			add(f.byChurch, metric, church, *v)
			add(f.byMonth, metric, m, *v)
			sk := slotKey{m, w + 1}
			add(f.bySlot, metric, sk, *v)
			if f.slotChurches[metric] == nil {
				f.slotChurches[metric] = map[slotKey]int{}
			}
			f.slotChurches[metric][sk]++
		}
		if !recorded {
			continue
		}
		f.reportsWithValue[metric]++
		if f.churchesWithValue[metric] == nil {
			f.churchesWithValue[metric] = map[int]map[int64]bool{}
		}
		if f.churchesWithValue[metric][m] == nil {
			f.churchesWithValue[metric][m] = map[int64]bool{}
		}
		f.churchesWithValue[metric][m][church] = true
	}
}

// summary is one metric's period figures; attendance adds its averages.
func (f *figures) summary(g stats.MetricGroup, metric string, counted []int, scope []church) stats.MetricSummary {
	s := stats.MetricSummary{Metric: metric, Label: metricLabel(metric), Group: g.Name, Unit: g.Unit, ReportsWithValue: f.reportsWithValue[metric]}
	var all acc
	for _, a := range f.byChurch[metric] {
		all.sum += a.sum
		all.obs += a.obs
		all.zeros += a.zeros
		if a.obs > 0 {
			s.ChurchesWithValue++
		}
	}
	s.Total, s.Observations, s.ZeroObservations = all.total(), all.obs, all.zeros
	for _, m := range counted {
		if a := f.byMonth[metric][m]; a.obs > 0 && a.sum > s.PeakValue {
			s.PeakMonth, s.PeakValue = report.Months[m-1], a.sum
		}
	}
	if g.Unit == stats.UnitPeople {
		s.AvgPerChurchService = all.average()
		s.NetworkWeekly, s.NetworkSlots, s.NetworkSlotsFull = f.networkWeekly(metric)
	}
	s.Churches = make([]stats.ChurchValue, 0, len(scope))
	for _, c := range scope {
		a := f.byChurch[metric][c.ID]
		cv := stats.ChurchValue{ChurchID: c.ID, Value: a.total(), Observations: a.obs, State: "value"}
		switch {
		case a.obs > 0:
		case f.churchReported[c.ID]:
			cv.State = "blank"
		case f.churchFiled[c.ID]:
			cv.State = "not_approved" // filed, but outside the Approved filter: not missing
		default:
			cv.State = "no_report"
		}
		s.Churches = append(s.Churches, cv)
	}
	return s
}

// networkWeekly averages the network totals of the slots with any value, and
// counts those slots and the ones every expected church recorded.
func (f *figures) networkWeekly(metric string) (*float64, int, int) {
	var sum float64
	slots, full := 0, 0
	for k, a := range f.bySlot[metric] {
		slots++
		sum += a.sum
		if e := f.expected[k.month]; e > 0 && f.slotChurches[metric][k] >= e {
			full++
		}
	}
	if slots == 0 {
		return nil, 0, 0
	}
	v := sum / float64(slots)
	return &v, slots, full
}

func add[K comparable](into map[string]map[K]acc, metric string, k K, v float64) {
	if into[metric] == nil {
		into[metric] = map[K]acc{}
	}
	a := into[metric][k]
	a.add(v)
	into[metric][k] = a
}

// trendPoints is metric per month; a month not counted has no value.
func trendPoints(months []stats.AnalyticsMonth, f figures, metric string) []stats.TrendPoint {
	last := maxCounted(f)
	out := make([]stats.TrendPoint, 0, len(months))
	for _, m := range months {
		p := stats.TrendPoint{Month: m.Month, Name: m.Name, State: m.State, Counted: m.State != stats.MonthNotYetOpen && m.Month <= last}
		if p.Counted {
			a := f.byMonth[metric][m.Month]
			p.Value, p.Observations = a.total(), a.obs
			p.ChurchesWithValue = len(f.churchesWithValue[metric][m.Month])
			p.ChurchesReported, p.ChurchesExpected = f.reported[m.Month], f.expected[m.Month]
			p.ExpectedReported = f.expectedReported[m.Month]
			p.Partial = p.ExpectedReported < p.ChurchesExpected
		}
		out = append(out, p)
	}
	return out
}

// maxCounted is the last counted month: the slots cover exactly those.
func maxCounted(f figures) int {
	n := 0
	for _, s := range f.slots {
		n = max(n, s.Month)
	}
	return n
}

func likeForLike(current, previous figures, metric string) stats.LikeForLike {
	var l stats.LikeForLike
	for id, c := range current.byChurch[metric] {
		p := previous.byChurch[metric][id]
		if c.obs == 0 || p.obs == 0 {
			continue
		}
		l.Churches++
		l.CurrentTotal += c.sum
		l.CurrentObservations += c.obs
		l.PreviousTotal += p.sum
		l.PreviousObservations += p.obs
	}
	if l.Churches == 0 {
		return l
	}
	cur, prev := acc{sum: l.CurrentTotal, obs: l.CurrentObservations}, acc{sum: l.PreviousTotal, obs: l.PreviousObservations}
	if isAttendance(metric) {
		l.Change = change(cur.average(), prev.average())
	} else {
		l.Change = change(cur.total(), prev.total())
	}
	return l
}

// compliance is the church-by-month matrix for every month of the period, any
// report status. A month with a report counts as expected even outside the
// church's months; a church with no reviewed start month expects nothing else.
func compliance(year int, months []stats.AnalyticsMonth, scope []church, reports map[churchMonth]*stats.ReportData, dups map[churchMonth]int, now time.Time) ([]stats.ComplianceRow, stats.ComplianceTotals) {
	var totals stats.ComplianceTotals
	rows := make([]stats.ComplianceRow, 0, len(scope))
	for _, c := range scope {
		row := stats.ComplianceRow{ChurchID: c.ID, ChurchName: c.Name}
		for _, m := range months {
			k := churchMonth{c.ID, year, m.Month}
			cell := complianceCell(c, year, m, reports[k], dups[k], now.Location())
			countCell(&row.ComplianceCounts, cell)
			row.Cells = append(row.Cells, cell)
		}
		addCounts(&totals.ComplianceCounts, row.ComplianceCounts)
		if row.Submitted > 0 {
			totals.ChurchesWithSubmission++
		}
		if c.NeedsReview {
			totals.ChurchesNeedingReview++
		}
		rows = append(rows, row)
	}
	return rows, totals
}

func complianceCell(c church, year int, m stats.AnalyticsMonth, r *stats.ReportData, dups int, loc *time.Location) stats.ComplianceCell {
	cell := stats.ComplianceCell{Month: m.Month}
	switch {
	case r != nil:
		cell.State, cell.ReportID, cell.Duplicates, cell.ReviewStatus = stats.CellSubmitted, r.ID, dups, r.Status
		cell.Expected = c.eligible(year, m.Month) && m.State != stats.MonthNotYetOpen
		cell.Timeliness = timeliness(year, time.Month(m.Month), r.CreatedAt, loc)
		cell.SubmittedOn = r.CreatedAt.In(loc).Format(time.DateOnly)
	case c.start == 0 && m.State == stats.MonthNotYetOpen:
		cell.State = stats.CellNotYetOpen
	case c.start == 0:
		cell.State = stats.CellNotReviewed
	case !c.Active && c.end == 0:
		cell.State = stats.CellInactive
	case !c.eligible(year, m.Month):
		cell.State = stats.CellNotRequired
	case m.State == stats.MonthNotYetOpen:
		cell.State = stats.CellNotYetOpen
	case m.State == stats.MonthOpen:
		cell.State, cell.Expected = stats.CellAwaiting, true
	case m.State == stats.MonthGrace:
		cell.State, cell.Expected = stats.CellOverdue, true
	default:
		cell.State, cell.Expected = stats.CellNotFiled, true
	}
	return cell
}

// countCell adds a cell to counts: a report, or a month still owed.
func countCell(n *stats.ComplianceCounts, cell stats.ComplianceCell) {
	if cell.State == stats.CellSubmitted && !cell.Expected {
		n.Outside++
		n.Duplicates += cell.Duplicates
		return
	}
	switch cell.State {
	case stats.CellSubmitted:
		n.Expected++
		n.Submitted++
		n.Duplicates += cell.Duplicates
		switch cell.ReviewStatus {
		case stats.AttendanceStatusApproved:
			n.Approved++
		case "needs_revision":
			n.NeedsRevision++
		default:
			n.Pending++
		}
		switch cell.Timeliness {
		case stats.TimelyOnTime:
			n.OnTime++
		case stats.TimelyLate:
			n.Late++
		}
	case stats.CellAwaiting:
		n.Expected++
		n.Awaiting++
	case stats.CellOverdue:
		n.Expected++
		n.Overdue++
	case stats.CellNotFiled:
		n.Expected++
		n.NotFiled++
	}
}

func addCounts(into *stats.ComplianceCounts, n stats.ComplianceCounts) {
	into.Expected += n.Expected
	into.Outside += n.Outside
	into.Submitted += n.Submitted
	into.Approved += n.Approved
	into.Pending += n.Pending
	into.NeedsRevision += n.NeedsRevision
	into.OnTime += n.OnTime
	into.Late += n.Late
	into.Awaiting += n.Awaiting
	into.Overdue += n.Overdue
	into.NotFiled += n.NotFiled
	into.Duplicates += n.Duplicates
}
