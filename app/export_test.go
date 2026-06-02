package app

import "io"

// PrintCommandHelpForTest exposes printCommandHelp to the
// black-box _test package without leaking it into the public surface.
func PrintCommandHelpForTest(w io.Writer, prog string, cmds []CommandDef) {
	printCommandHelp(w, prog, cmds)
}

// RunCommandIfMatchedForTest exposes the argv dispatcher to tests
// without going through Run (which calls os.Exit).
func RunCommandIfMatchedForTest(args []string, opts ...Option) (handled bool, exitCode int) {
	cfg := resolve(opts)
	return runCommandIfMatched(args, cfg)
}
