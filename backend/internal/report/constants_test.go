package report

import (
	"encoding/json"
	"regexp"
	"testing"
	"time"
)

var snakeCase = regexp.MustCompile(`^[a-z]+(_[a-z]+)*$`)

func TestMetricKeys(t *testing.T) {
	if got := len(MetricKeys); got != 21 {
		t.Fatalf("len(MetricKeys) = %d, want 21", got)
	}
	seen := map[string]bool{}
	for _, k := range MetricKeys {
		if !snakeCase.MatchString(k) {
			t.Errorf("metric key %q is not snake_case", k)
		}
		if len(k) > 50 {
			t.Errorf("metric key %q exceeds VARCHAR(50)", k)
		}
		if seen[k] {
			t.Errorf("duplicate metric key %q", k)
		}
		seen[k] = true
	}
}

func TestMonths(t *testing.T) {
	if len(Months) != 12 || Months[0] != "January" || Months[11] != "December" {
		t.Fatalf("Months = %v, want January..December", Months)
	}
}

func TestParseMonth(t *testing.T) {
	tests := []struct {
		in     string
		want   time.Month
		wantOK bool
	}{
		{"January", time.January, true},
		{"september", time.September, true},
		{"DECEMBER", time.December, true},
		{"Sept", 0, false},
		{"", 0, false},
		{"13", 0, false},
	}
	for _, tt := range tests {
		got, ok := ParseMonth(tt.in)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("ParseMonth(%q) = %v, %v; want %v, %v", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestMetricWeeks_NonNil(t *testing.T) {
	one, three := json.Number("1.50"), json.Number("0")
	got := MetricWeeks{Week1: &one, Week3: &three}.NonNil()

	if len(got) != 2 || got[1] != "1.50" || got[3] != "0" {
		t.Fatalf("NonNil() = %v, want {1: 1.50, 3: 0}", got)
	}
	if _, ok := got[2]; ok {
		t.Error("absent week 2 must not appear (D-5: absent is not zero)")
	}
}

func TestCanonicalMonth(t *testing.T) {
	tests := []struct{ in, want string }{
		{"March", "March"},
		{"march", "March"},
		{"SEPTEMBER", "September"},
		{"Sept", "Sept"}, // not a month: unchanged, so it matches no report
		{"", ""},
	}
	for _, tt := range tests {
		if got := CanonicalMonth(tt.in); got != tt.want {
			t.Errorf("CanonicalMonth(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
