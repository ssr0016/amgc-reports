package audit

import (
	"errors"
	"strings"
	"testing"
)

func TestActions(t *testing.T) {
	if got := len(Actions); got != 23 {
		t.Fatalf("len(Actions) = %d, want 23", got)
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
	for _, want := range []string{ActionChangeUserStatus, ActionAdminResetPassword, ActionLogin, ActionLogout, ActionLoginFailed,
		ActionCreateRole, ActionUpdateRole, ActionDeleteRole, ActionChangeUserRole} {
		found := false
		for _, a := range Actions {
			found = found || a == want
		}
		if !found {
			t.Errorf("%q is not in Actions", want)
		}
	}
}

func TestDetailKeys_CoverEveryActionAndNoSecret(t *testing.T) {
	if len(DetailKeys) != len(Actions) {
		t.Errorf("len(DetailKeys) = %d, want %d (one entry per action, no extras)", len(DetailKeys), len(Actions))
	}
	secret := []string{"password", "hash", "token", "secret", "cookie", "csrf", "narrative", "prayer", "challenges", "names"}
	for _, a := range Actions {
		keys, ok := DetailKeys[a]
		if !ok {
			t.Errorf("%q has no DetailKeys entry", a)
		}
		for _, k := range keys {
			for _, s := range secret {
				if strings.Contains(k, s) {
					t.Errorf("%q allows key %q, which looks sensitive (%q)", a, k, s)
				}
			}
		}
	}
}

func TestCheckDetails(t *testing.T) {
	tests := []struct {
		name    string
		log     Log
		wantErr error
	}{
		{"allowed keys", Log{Action: ActionApproveReport, Details: map[string]any{"month": "May", "year": 2026, "count": 1}}, nil},
		{"subset of allowed keys", Log{Action: ActionDeleteUser, Details: map[string]any{"sessions_revoked": 0}}, nil},
		{"nil details", Log{Action: ActionLogin}, nil},
		{"empty details where none are allowed", Log{Action: ActionUpdateUser, Details: map[string]any{}}, nil},
		{"failed login reason", Log{Action: ActionLoginFailed, Details: map[string]any{"reason": ReasonWrongPassword}}, nil},
		{"key outside the list", Log{Action: ActionApproveReport, Details: map[string]any{"narrative_report": "x"}}, ErrDetailKey},
		{"any key where none are allowed", Log{Action: ActionLogin, Details: map[string]any{"email": "a@b.c"}}, ErrDetailKey},
		{"password on a reset", Log{Action: ActionAdminResetPassword, Details: map[string]any{"new_password": "x"}}, ErrDetailKey},
		{"role created", Log{Action: ActionCreateRole, Details: map[string]any{"name": "Reviewer", "permissions": []string{"reports:read_any"}}}, nil},
		{"role updated", Log{Action: ActionUpdateRole, Details: map[string]any{"from": "A", "to": "B", "added": []string{}, "removed": []string{}}}, nil},
		{"role deleted", Log{Action: ActionDeleteRole, Details: map[string]any{"name": "Reviewer"}}, nil},
		{"user role changed", Log{Action: ActionChangeUserRole, Details: map[string]any{"from": "user", "to": "Reviewer"}}, nil},
		{"description on a role create", Log{Action: ActionCreateRole, Details: map[string]any{"description": "x"}}, ErrDetailKey},
		{"description on a role update", Log{Action: ActionUpdateRole, Details: map[string]any{"description": "x"}}, ErrDetailKey},
		{"permissions on a role delete", Log{Action: ActionDeleteRole, Details: map[string]any{"permissions": []string{}}}, ErrDetailKey},
		{"role id on a user role change", Log{Action: ActionChangeUserRole, Details: map[string]any{"role_id": 4}}, ErrDetailKey},
		{"unknown action", Log{Action: "NOPE"}, ErrDetailKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.log.CheckDetails()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CheckDetails() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
