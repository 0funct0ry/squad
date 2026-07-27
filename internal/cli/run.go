package cli

import "os"

// Run decides between the interactive REPL, the inline-SQL-argument mode,
// and the stdin/script mode, based on whether an inline SQL arg was given
// and whether stdin is a terminal. s.Interactive must already reflect this
// choice (see NewState) so .mode/.headers defaults were set correctly.
func Run(s *State, inlineSQL string) error {
	defer func() {
		if s.RestManager != nil {
			_ = s.RestManager.Stop("cli exit")
		}
	}()
	if inlineSQL != "" {
		RunInline(s, inlineSQL)
		return nil
	}
	if !s.Interactive {
		return RunScript(s, os.Stdin)
	}
	return RunREPL(s)
}
