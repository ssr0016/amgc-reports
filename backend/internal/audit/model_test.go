package audit

import "testing"

func TestActions(t *testing.T) {
	if got := len(Actions); got != 14 {
		t.Fatalf("len(Actions) = %d, want 14", got)
	}
	seen := map[string]bool{}
	for _, a := range Actions {
		if seen[a] {
			t.Errorf("duplicate audit action %q", a)
		}
		seen[a] = true
	}
}

func TestActions_IncludeEveryNamedAction(t *testing.T) {
	for _, want := range []string{ActionChangeUserStatus, ActionAdminResetPassword} {
		found := false
		for _, a := range Actions {
			found = found || a == want
		}
		if !found {
			t.Errorf("%q is not in Actions", want)
		}
	}
}
