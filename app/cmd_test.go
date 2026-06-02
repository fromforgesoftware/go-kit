package app_test

import (
	"bytes"
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"

	"github.com/fromforgesoftware/go-kit/app"
)

func TestWithCommands_RejectsReservedNames(t *testing.T) {
	assert.PanicsWithValue(t, "app.WithCommands: reserved command name help", func() {
		_ = app.WithCommands(app.Command("help", "x", func() {}))
	})
}

func TestWithCommands_RejectsEmptyName(t *testing.T) {
	assert.PanicsWithValue(t, "app.WithCommands: command name is required", func() {
		_ = app.WithCommands(app.Command("", "x", func() {}))
	})
}

func TestWithCommands_RejectsNilInvoke(t *testing.T) {
	assert.PanicsWithValue(t, "app.WithCommands: command foo has no Invoke", func() {
		_ = app.WithCommands(app.CommandDef{Name: "foo", Summary: "x"})
	})
}

func TestPrintCommandHelp_ListsCommandsSortedWithSummaries(t *testing.T) {
	var buf bytes.Buffer
	app.PrintCommandHelpForTest(&buf, "auth", []app.CommandDef{
		{Name: "dump-user", Summary: "print a user's internal state", Invoke: func() {}},
		{Name: "backfill-emails", Summary: "re-run email normalisation", Invoke: func() {}},
	})
	out := buf.String()
	assert.Contains(t, out, "Commands:")
	// Names listed in alpha order (backfill-emails before dump-user).
	bIdx := indexOrFail(t, out, "backfill-emails")
	dIdx := indexOrFail(t, out, "dump-user")
	assert.True(t, bIdx < dIdx, "commands should be alphabetically sorted")
	assert.Contains(t, out, "re-run email normalisation")
	assert.Contains(t, out, "print a user's internal state")
}

func TestPrintCommandHelp_EmptySetMentionsBareBoot(t *testing.T) {
	var buf bytes.Buffer
	app.PrintCommandHelpForTest(&buf, "svc", nil)
	assert.Contains(t, buf.String(), "Bare invocation boots the HTTP server")
	assert.Contains(t, buf.String(), "No admin commands are registered")
}

func TestDispatcher_RunsMatchedCommandViaLifecycleHook(t *testing.T) {
	// Exercises the same dispatcher path that Run() uses for `./svc
	// <name>` argv — without going through Run() (which calls
	// os.Exit). The command schedules an OnStart hook; the
	// dispatcher's app.Start fires it; Stop runs cleanly.
	t.Setenv("REST_ADDRESS", "127.0.0.1:0") // Not used (WithoutHTTP is forced by dispatcher), set just in case.
	var ran atomic.Bool

	handled, code := app.RunCommandIfMatchedForTest(
		[]string{"svc-test", "do-the-thing"},
		app.WithName("cmd-dispatcher-test"),
		app.WithCommands(app.Command("do-the-thing", "exercise the hook", func(lc fx.Lifecycle) {
			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					ran.Store(true)
					return nil
				},
			})
		})),
	)
	assert.True(t, handled, "dispatcher should claim known command names")
	assert.Equal(t, 0, code)
	assert.True(t, ran.Load(), "OnStart hook should have fired")
}

func TestDispatcher_UnknownCommandSurfacesError(t *testing.T) {
	handled, code := app.RunCommandIfMatchedForTest(
		[]string{"svc-test", "definitely-not-registered"},
		app.WithName("cmd-dispatcher-test"),
		app.WithCommands(app.Command("known", "x", func() {})),
	)
	assert.True(t, handled, "dispatcher should claim non-flag positional args as command attempts")
	assert.Equal(t, 2, code, "unknown command should exit 2")
}

func TestDispatcher_BareInvocationFallsThrough(t *testing.T) {
	handled, _ := app.RunCommandIfMatchedForTest(
		[]string{"svc-test"},
		app.WithName("cmd-dispatcher-test"),
		app.WithCommands(app.Command("known", "x", func() {})),
	)
	assert.False(t, handled, "bare binary invocation should fall through to HTTP boot")
}

func TestDispatcher_HelpFlagHandled(t *testing.T) {
	for _, flag := range []string{"--help", "-h", "help"} {
		t.Run(flag, func(t *testing.T) {
			handled, code := app.RunCommandIfMatchedForTest(
				[]string{"svc-test", flag},
				app.WithName("cmd-dispatcher-test"),
				app.WithCommands(app.Command("known", "x", func() {})),
			)
			assert.True(t, handled, "help flag should be claimed")
			assert.Equal(t, 0, code)
		})
	}
}

func indexOrFail(t *testing.T, hay, needle string) int {
	t.Helper()
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return i
		}
	}
	t.Fatalf("substring %q not found in %q", needle, hay)
	return -1
}
