package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// spy stands in for run and runMigrate so dispatch can be tested without a
// database or a listening server.
type spy struct {
	runCalls     int
	migrateCalls int
	runErr       error
	migrateErr   error
}

func (s *spy) commands() commands {
	return commands{
		run:     func() error { s.runCalls++; return s.runErr },
		migrate: func() error { s.migrateCalls++; return s.migrateErr },
	}
}

func dispatchWith(t *testing.T, s *spy, args ...string) (code int, stderr string) {
	t.Helper()
	var buf bytes.Buffer
	code = dispatch(args, &buf, s.commands())
	return code, buf.String()
}

func TestDispatch_NoArgs_CallsRun(t *testing.T) {
	s := &spy{}

	code, stderr := dispatchWith(t, s)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if s.runCalls != 1 || s.migrateCalls != 0 {
		t.Errorf("run called %d times, migrate %d times; want 1 and 0", s.runCalls, s.migrateCalls)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestDispatch_Migrate_CallsRunMigrate_NotRun(t *testing.T) {
	s := &spy{}

	code, stderr := dispatchWith(t, s, "migrate")

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if s.migrateCalls != 1 {
		t.Errorf("migrate called %d times, want 1", s.migrateCalls)
	}
	if s.runCalls != 0 {
		t.Errorf("run called %d times, want 0: migrate must not start the server", s.runCalls)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestDispatch_MigrateExtraArg_ExitsUsage(t *testing.T) {
	s := &spy{}

	code, stderr := dispatchWith(t, s, "migrate", "extra-arg")

	if code != 2 {
		t.Errorf("exit code = %d, want 2 (usage error)", code)
	}
	if s.runCalls != 0 || s.migrateCalls != 0 {
		t.Errorf("run called %d times, migrate %d times; want 0 and 0", s.runCalls, s.migrateCalls)
	}
	if !strings.Contains(stderr, `"extra-arg"`) {
		t.Errorf("stderr = %q, want it to name the extra argument", stderr)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("stderr = %q, want a usage line", stderr)
	}
}

func TestDispatch_UnknownSubcommand_ExitsUsage(t *testing.T) {
	s := &spy{}

	code, stderr := dispatchWith(t, s, "migrat")

	if code != 2 {
		t.Errorf("exit code = %d, want 2 (usage error)", code)
	}
	if s.runCalls != 0 || s.migrateCalls != 0 {
		t.Errorf("a typo started something: run %d, migrate %d", s.runCalls, s.migrateCalls)
	}
	if !strings.Contains(stderr, `unknown subcommand: "migrat"`) {
		t.Errorf("stderr = %q, want it to name the token", stderr)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("stderr = %q, want a usage line", stderr)
	}
}

func TestDispatch_EmptyStringArg_ExitsUsage(t *testing.T) {
	s := &spy{}

	code, stderr := dispatchWith(t, s, "")

	if code != 2 {
		t.Errorf("exit code = %d, want 2 (usage error)", code)
	}
	if s.runCalls != 0 || s.migrateCalls != 0 {
		t.Errorf("an empty argument started something: run %d, migrate %d", s.runCalls, s.migrateCalls)
	}
	if !strings.Contains(stderr, `unknown subcommand: ""`) {
		t.Errorf("stderr = %q, want it to name the empty token", stderr)
	}
}

// A runtime failure is exit 1, distinct from a usage error (2), so a pre-deploy
// step can tell "bad invocation" from "migration failed".
func TestDispatch_CommandErrors_ExitOne(t *testing.T) {
	boom := errors.New("boom")

	t.Run("server", func(t *testing.T) {
		s := &spy{runErr: boom}
		if code, _ := dispatchWith(t, s); code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	})
	t.Run("migrate", func(t *testing.T) {
		s := &spy{migrateErr: boom}
		if code, _ := dispatchWith(t, s, "migrate"); code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	})
}
