package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"go.uber.org/fx"
)

// CommandDef is a single admin command registered against a service
// binary. When the binary is invoked with `./svc <name>` instead of
// the bare `./svc`, the matching command runs to completion (full
// fx graph + lifecycle) and the process exits — no HTTP server.
//
// Invoke is anything fx.Invoke accepts: the kit hands it to fx
// verbatim. Inside the function, declare whatever deps you need as
// parameters (fx resolves them) and use fx.Lifecycle to schedule
// the actual work after Start so OnStart hooks of upstream modules
// (DB connect, NATS subscribe, …) have already fired.
//
//	app.Command("backfill-emails", "re-run normalisation across users",
//	    func(lc fx.Lifecycle, db *sql.DB) {
//	        lc.Append(fx.Hook{
//	            OnStart: func(ctx context.Context) error {
//	                return backfill.RunEmails(ctx, db)
//	            },
//	        })
//	    })
type CommandDef struct {
	Name    string
	Summary string
	Invoke  any
}

// Command is the constructor for CommandDef — kept as a helper so
// the call site reads naturally.
func Command(name, summary string, fn any) CommandDef {
	return CommandDef{Name: name, Summary: summary, Invoke: fn}
}

// WithCommands registers admin commands on the service binary.
// Each command becomes invokable via `./svc <name>`; the bare binary
// still boots the HTTP server by default.
//
// Reserved names ("help", "-h", "--help") are rejected at boot via
// panic; help output is auto-generated.
func WithCommands(cmds ...CommandDef) Option {
	// Validate eagerly — wiring bugs (typo'd name, missing invoke)
	// should surface at the call site, not at app boot.
	for _, cmd := range cmds {
		if cmd.Name == "" {
			panic("app.WithCommands: command name is required")
		}
		if isReservedCommandName(cmd.Name) {
			panic("app.WithCommands: reserved command name " + cmd.Name)
		}
		if cmd.Invoke == nil {
			panic("app.WithCommands: command " + cmd.Name + " has no Invoke")
		}
	}
	return newAppOption(func(c *config) {
		c.commands = append(c.commands, cmds...)
	})
}

// runCommandIfMatched checks os.Args for a registered command name.
// Returns ok=true (and an error from the command, or nil) when a
// command ran; ok=false means the bare binary should boot normally.
//
// The matched command runs with HTTP disabled — admin commands and
// HTTP servers are mutually exclusive per-process invocation.
func runCommandIfMatched(args []string, cfg *config) (handled bool, exitCode int) {
	if len(args) < 2 {
		return false, 0
	}
	first := args[1]

	if isHelpFlag(first) {
		printCommandHelp(os.Stdout, args[0], cfg.commands)
		return true, 0
	}

	for _, cmd := range cfg.commands {
		if cmd.Name != first {
			continue
		}
		// Force HTTP off + append the command's invoke so fx wires it
		// when we build the app below.
		cfg.withoutHTTP = true
		cfg.userOptions = append(cfg.userOptions, fx.Invoke(cmd.Invoke))
		if err := runCommand(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "forge: %s: %v\n", cmd.Name, err)
			return true, 1
		}
		return true, 0
	}

	// Not a registered command and not a help flag — could be a
	// genuine arg the caller passed through to the HTTP server (rare
	// but legal). Fall through to bare boot rather than guessing.
	if looksLikeCommandAttempt(first) {
		fmt.Fprintf(os.Stderr, "forge: unknown command %q\n\n", first)
		printCommandHelp(os.Stderr, args[0], cfg.commands)
		return true, 2
	}
	return false, 0
}

// runCommand builds the fx app, Starts it (which fires the command's
// lifecycle hook), then Stops it cleanly.
func runCommand(cfg *config) error {
	app := buildAppFromConfig(cfg)

	startCtx, cancelStart := context.WithTimeout(context.Background(), app.StartTimeout())
	defer cancelStart()
	if err := app.Start(startCtx); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	stopCtx, cancelStop := context.WithTimeout(context.Background(), app.StopTimeout())
	defer cancelStop()
	if err := app.Stop(stopCtx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("stop: %w", err)
	}
	return nil
}

func isReservedCommandName(name string) bool {
	switch name {
	case "help", "-h", "--help":
		return true
	}
	return false
}

func isHelpFlag(s string) bool {
	return s == "-h" || s == "--help" || s == "help"
}

// looksLikeCommandAttempt is a tiny heuristic: a positional arg that
// doesn't start with "-" is probably meant to be a command name. We
// surface "unknown command" for those rather than silently booting
// the HTTP server (which would confuse the operator).
func looksLikeCommandAttempt(s string) bool {
	return s != "" && !strings.HasPrefix(s, "-")
}

func printCommandHelp(w io.Writer, prog string, cmds []CommandDef) {
	_, _ = fmt.Fprintf(w, "%s — forge service binary.\n\n", prog)
	_, _ = fmt.Fprintf(w, "Usage:\n  %s [<command>] [args...]\n\n", prog)
	if len(cmds) == 0 {
		_, _ = fmt.Fprintln(w, "  Bare invocation boots the HTTP server. No admin commands are registered.")
		return
	}
	_, _ = fmt.Fprintln(w, "  Bare invocation boots the HTTP server.")
	_, _ = fmt.Fprintln(w, "  Named commands run admin tasks instead (full fx graph, then exit).")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Commands:")
	sorted := append([]CommandDef(nil), cmds...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	width := 0
	for _, c := range sorted {
		if len(c.Name) > width {
			width = len(c.Name)
		}
	}
	for _, c := range sorted {
		_, _ = fmt.Fprintf(w, "  %-*s  %s\n", width, c.Name, c.Summary)
	}
}
