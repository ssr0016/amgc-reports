package report

import (
	"strings"
	"time"
)

// Months are the English month names allowed by chk_reports_month (exact casing).
var Months = []string{
	"January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
}

// ParseMonth reads an English month name in any case.
func ParseMonth(name string) (time.Month, bool) {
	for i, m := range Months {
		if strings.EqualFold(m, name) {
			return time.Month(i + 1), true
		}
	}
	return 0, false
}

// CanonicalMonth returns name in the stored casing ("march" gives "March"), or
// name unchanged when it is not a month, so it still matches no report.
// Queries compare r.month exactly with it, which lets the month indexes apply.
func CanonicalMonth(name string) string {
	if m, ok := ParseMonth(name); ok {
		return Months[m-1]
	}
	return name
}

// MetricKeys are the 22 snake_case metric keys of chk_report_metrics_metric (D-6),
// in form order. person_evangelized is last: it is the legacy field, kept for
// historical values whose meaning is unverified (D-A31); new reports use
// person_led_to_christ.
// This list is the source of truth: a test compares it with the DTO validate tag
// and another with the CHECK constraint. Adding a metric also needs a migration.
var MetricKeys = []string{
	"worship_service", "sunday_school", "prayer_meeting", "bible_studies",
	"mens_fellowship", "womens_fellowship", "youth_fellowship", "children_fellowship",
	"tithes_offering", "home_visited", "bible_study_group_led", "sermon_preached",
	"person_newly_contacted", "person_followed_up", "person_led_to_christ", "outreach",
	"training", "leadership", "baptism", "other", "family_day", "person_evangelized",
}

// LegacyMetric is the metric kept only for historical values (D-A31).
const LegacyMetric = "person_evangelized"
