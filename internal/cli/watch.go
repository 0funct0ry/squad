package cli

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

// cmdWatch implements ".watch INTERVAL_SECONDS QUERY": re-runs QUERY every
// INTERVAL_SECONDS, re-rendering in place, until Ctrl-C. This is the one
// command that needs its own scoped interrupt handling distinct from the
// REPL's normal buffer-clearing Ctrl-C, since it's blocked in time.Sleep/
// DB.Query rather than readline.Readline() -- signal.Stop cleanly
// unregisters the channel on return, restoring normal Ctrl-C handling.
func (s *State) cmdWatch(rest string) {
	parts := strings.SplitN(strings.TrimSpace(rest), " ", 2)
	if len(parts) != 2 {
		s.shellError(fmt.Errorf("usage: .watch INTERVAL_SECONDS QUERY"))
		return
	}
	interval, err := strconv.ParseFloat(parts[0], 64)
	if err != nil || interval <= 0 {
		s.shellError(fmt.Errorf("invalid interval: %s", parts[0]))
		return
	}
	query := unquoteDotCommandText(strings.TrimSpace(parts[1]))
	if query == "" {
		s.shellError(fmt.Errorf("usage: .watch INTERVAL_SECONDS QUERY"))
		return
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	runOnce := func() bool {
		rendered, err := preprocessTemplate(query)
		if err != nil {
			s.shellError(err)
			return false
		}
		rows, err := s.DB.Query(rendered)
		if err != nil {
			s.shellError(err)
			return false
		}
		defer rows.Close()
		cols, resultRows, err := scanRowsToValues(rows)
		if err != nil {
			s.shellError(err)
			return false
		}
		fmt.Fprint(s.Out, "\033[H\033[2J")
		if err := s.Render(cols, resultRows); err != nil {
			s.shellError(err)
		}
		return true
	}

	if !runOnce() {
		return
	}

	ticker := time.NewTicker(time.Duration(interval * float64(time.Second)))
	defer ticker.Stop()

	for {
		select {
		case <-sigCh:
			fmt.Fprintln(s.Out, "^C")
			return
		case <-ticker.C:
			if !runOnce() {
				return
			}
		}
	}
}
