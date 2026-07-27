package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func (s *State) cmdTimer(args []string) {
	if len(args) != 1 || (args[0] != "on" && args[0] != "off") {
		s.shellError(fmt.Errorf("usage: .timer on|off"))
		return
	}
	s.TimerOn = args[0] == "on"
}

func (s *State) cmdStats(args []string) {
	if len(args) != 1 || (args[0] != "on" && args[0] != "off") {
		s.shellError(fmt.Errorf("usage: .stats on|off"))
		return
	}
	s.StatsOn = args[0] == "on"
}

type explainPlanRow struct {
	id, parent, notused int
	detail              string
}

// cmdExplain implements ".explain QUERY" / ".plan QUERY": runs
// "EXPLAIN QUERY PLAN <query>" (auto-prepending if the caller didn't type it
// themselves) through the same read path as an ordinary query, then renders
// its id/parent/notused/detail columns as an indented tree via parent/id
// chaining (root nodes have parent = 0) instead of a flat table.
func (s *State) cmdExplain(query string) {
	if query == "" {
		s.shellError(fmt.Errorf("usage: .explain QUERY"))
		return
	}
	rendered, err := preprocessTemplate(unquoteDotCommandText(query))
	if err != nil {
		s.shellError(err)
		return
	}
	rendered = strings.TrimRight(strings.TrimSpace(rendered), ";")
	upper := strings.ToUpper(rendered)
	if !strings.HasPrefix(upper, "EXPLAIN QUERY PLAN") {
		if strings.HasPrefix(upper, "EXPLAIN") {
			rendered = "EXPLAIN QUERY PLAN " + strings.TrimSpace(rendered[len("EXPLAIN"):])
		} else {
			rendered = "EXPLAIN QUERY PLAN " + rendered
		}
	}

	rows, err := s.DB.Query(rendered)
	if err != nil {
		s.shellError(err)
		return
	}
	defer rows.Close()

	var planRows []explainPlanRow
	for rows.Next() {
		var pr explainPlanRow
		if err := rows.Scan(&pr.id, &pr.parent, &pr.notused, &pr.detail); err != nil {
			s.shellError(err)
			return
		}
		planRows = append(planRows, pr)
	}
	if err := rows.Err(); err != nil {
		s.shellError(err)
		return
	}

	byParent := map[int][]explainPlanRow{}
	for _, pr := range planRows {
		byParent[pr.parent] = append(byParent[pr.parent], pr)
	}
	var walk func(parent, depth int)
	walk = func(parent, depth int) {
		for _, pr := range byParent[parent] {
			fmt.Fprintf(s.Out, "%s%s\n", strings.Repeat("  ", depth), pr.detail)
			walk(pr.id, depth+1)
		}
	}
	walk(0, 0)
}

// cmdShell implements ".shell CMD" / ".sh CMD": runs CMD via $SHELL -c
// (fallback /bin/sh), inheriting the process's stdio.
func (s *State) cmdShell(cmdText string) {
	if cmdText == "" {
		s.shellError(fmt.Errorf("usage: .shell CMD"))
		return
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	cmd := exec.Command(shell, "-c", cmdText)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		s.shellError(err)
	}
}
