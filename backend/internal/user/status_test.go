package user

import (
	"testing"
	"time"
)

func TestUserStatuses_AreTheFourOwnerValues(t *testing.T) {
	want := []string{"active", "inactive", "disciplinary", "deceased"}
	if len(UserStatuses) != len(want) {
		t.Fatalf("UserStatuses = %v, want %v", UserStatuses, want)
	}
	for i, s := range want {
		if UserStatuses[i] != s {
			t.Errorf("UserStatuses[%d] = %q, want %q", i, UserStatuses[i], s)
		}
	}
}

func TestUser_CanSignIn(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{UserStatusActive, true},
		// NOT NULL in the database, so empty only comes from a hand-built value (a test fixture).
		{"", true},
		{UserStatusInactive, false},
		{UserStatusDisciplinary, false},
		{UserStatusDeceased, false},
		// A value the Go side does not know is refused, never let through.
		{"retired", false},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := (&User{Status: tt.status}).CanSignIn(); got != tt.want {
				t.Errorf("CanSignIn(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

// A deleted account never signs in, even while its status is active (D-A11).
func TestUser_CanSignIn_Deleted(t *testing.T) {
	now := time.Now()
	if (&User{Status: UserStatusActive, DeletedAt: &now}).CanSignIn() {
		t.Error("CanSignIn() = true for a deleted active user, want false")
	}
}
