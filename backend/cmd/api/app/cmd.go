package app

import (
	"fmt"
	"io"
	"log/slog"
)

const usage = `usage: api [migrate]

  (no arguments)  start the HTTP server
  migrate         apply database migrations and exit
`

// commands are what dispatch can run. They are fields so tests can replace
// them without a database or a listening server.
type commands struct {
	run     func() error
	migrate func() error
}

// Execute runs the API command and returns its process exit code.
func Execute(args []string, stderr io.Writer) int {
	return dispatch(args, stderr, commands{run: run, migrate: runMigrate})
}

// dispatch runs the subcommand named by args and returns the process exit
// code: 0 on success, 1 when the command fails, 2 for a usage error. An
// unrecognised argument is an error rather than "start the server", so a typo
// in a pre-deploy command cannot silently launch the server instead.
func dispatch(args []string, stderr io.Writer, cmds commands) int {
	var cmd func() error
	switch {
	case len(args) == 0:
		cmd = cmds.run
	case args[0] == "migrate" && len(args) == 1:
		cmd = cmds.migrate
	case args[0] == "migrate":
		_, _ = fmt.Fprintf(stderr, "unexpected argument %q for migrate\n%s", args[1], usage)
		return 2
	default:
		_, _ = fmt.Fprintf(stderr, "unknown subcommand: %q\n%s", args[0], usage)
		return 2
	}

	if err := cmd(); err != nil {
		slog.Error("fatal error", "error", err)
		return 1
	}
	return 0
}
