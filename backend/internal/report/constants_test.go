package report

import (
	"encoding/json"
	"regexp"
	"testing"
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
