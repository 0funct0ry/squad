package cli

import (
	"fmt"
	"strconv"
	"strings"
)

// cmdRepeat implements ".repeat N QUERY": runs QUERY N times, going through
// the normal Execute pipeline each time -- including DDL, since Execute
// itself dispatches dot-commands vs Classify/exec/render, it applies the
// same read-only gating, .timer/.stats, and LastColumns/LastRows tracking
// on every iteration. Each iteration re-runs {{ }} template preprocessing
// from scratch (Execute calls preprocessTemplate itself), so a call like
// {{firstName}} produces a fresh value on every repetition rather than
// reusing the first expansion.
func (s *State) cmdRepeat(rest string) {
	parts := strings.SplitN(strings.TrimSpace(rest), " ", 2)
	if len(parts) != 2 {
		s.shellError(fmt.Errorf(`usage: .repeat N "QUERY"`))
		return
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil || n < 1 {
		s.shellError(fmt.Errorf("invalid repeat count: %s", parts[0]))
		return
	}
	query := unquoteDotCommandText(strings.TrimSpace(parts[1]))
	if query == "" {
		s.shellError(fmt.Errorf(`usage: .repeat N "QUERY"`))
		return
	}

	for i := 0; i < n; i++ {
		s.Execute(query)
	}
}
