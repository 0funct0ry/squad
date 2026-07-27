package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// cmdEdit implements ".edit -h N" (seed from history entry N) and ".edit -c"
// (seed from the OS clipboard). It opens $EDITOR (falling back to vi) on a
// temp file containing the seed text, then loads the edited result into the
// *next* readline prompt for review (State.pendingDefault, consumed by
// RunREPL) rather than auto-executing it.
func (s *State) cmdEdit(args []string) {
	if !s.Interactive {
		s.shellError(fmt.Errorf(".edit is only available in interactive mode"))
		return
	}

	var seedText string
	switch {
	case len(args) == 2 && args[0] == "-h":
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1 || n > len(s.History) {
			s.shellError(fmt.Errorf("no such history entry: %s", args[1]))
			return
		}
		seedText = s.History[n-1]
	case len(args) == 1 && args[0] == "-c":
		text, err := readClipboard()
		if err != nil {
			s.shellError(err)
			return
		}
		seedText = text
	default:
		s.shellError(fmt.Errorf("usage: .edit -h N | .edit -c"))
		return
	}

	tmp, err := os.CreateTemp("", "squad-edit-*.sql")
	if err != nil {
		s.shellError(err)
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(seedText); err != nil {
		tmp.Close()
		s.shellError(err)
		return
	}
	tmp.Close()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, tmpPath)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		s.shellError(fmt.Errorf("editor exited with error: %w", err))
		return
	}

	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		s.shellError(err)
		return
	}
	s.pendingDefault = string(edited)
}
