package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/0funct0ry/squad/internal/hooks"
)

// cmdHooks implements ".hooks list|create|edit|test|enable|disable|rm|log",
// mirroring `squad hooks` and calling internal/hooks directly rather than
// shelling out. Dispatched from dispatchDotCommand's free-text prefix phase
// (like .functions/.mount) because ".hooks test ID '{"new":{...}}'" carries
// JSON with spaces and quotes that the generic whitespace tokenizer would
// shred.
func (s *State) cmdHooks(rest string) {
	if !s.HooksEnabled {
		s.shellError(fmt.Errorf("Lua trigger hooks are off; relaunch with --hooks to enable them"))
		return
	}
	tokens, err := tokenizeMountArgs(rest)
	if err != nil {
		s.shellError(err)
		return
	}
	action := "list"
	if len(tokens) > 0 {
		action = strings.ToLower(tokens[0])
		tokens = tokens[1:]
	}

	switch action {
	case "list":
		s.hooksList(tokens)
	case "create":
		s.hooksCreate(tokens)
	case "edit":
		s.hooksEdit(tokens)
	case "test":
		s.hooksTest(tokens)
	case "enable":
		s.hooksSetEnabled(tokens, true)
	case "disable":
		s.hooksSetEnabled(tokens, false)
	case "rm", "delete":
		s.hooksRemove(tokens)
	case "log", "logs":
		s.hooksLog(tokens)
	default:
		s.shellError(fmt.Errorf("unknown .hooks action %q (want list/create/edit/test/enable/disable/rm/log)", action))
	}
}

func (s *State) hooksRequireWrite() bool {
	if !s.Write {
		s.shellError(fmt.Errorf("READ_ONLY: managing hooks requires --write mode"))
		return false
	}
	return true
}

func hookArgID(tokens []string) (int64, error) {
	if len(tokens) == 0 {
		return 0, fmt.Errorf("a hook ID is required")
	}
	id, err := strconv.ParseInt(tokens[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid hook id %q", tokens[0])
	}
	return id, nil
}

func (s *State) hooksList(tokens []string) {
	table := ""
	for i := 0; i < len(tokens); i++ {
		if tokens[i] == "--table" && i+1 < len(tokens) {
			table = tokens[i+1]
			i++
		}
	}
	list, err := hooks.List(s.DB, table)
	if err != nil {
		s.shellError(err)
		return
	}
	if len(list) == 0 {
		fmt.Fprintln(s.Out, "no hooks defined")
		return
	}
	fmt.Fprintf(s.Out, "%-5s %-20s %-8s %-7s %-10s %-8s %s\n", "ID", "TABLE", "EVENT", "TIMING", "SCOPE", "ENABLED", "NAME")
	for _, h := range list {
		fmt.Fprintf(s.Out, "%-5d %-20s %-8s %-7s %-10s %-8v %s\n", h.ID, h.Table, h.Event, h.Timing, h.Scope, h.Enabled, h.Name)
	}
}

// hooksCreate parses ".hooks create --table T --event insert --timing after
// --scope row --name N --file PATH".
func (s *State) hooksCreate(tokens []string) {
	if !s.hooksRequireWrite() {
		return
	}
	h := hooks.Hook{Event: "insert", Timing: "after", Scope: "row", Enabled: true}
	file := ""
	for i := 0; i+1 < len(tokens); i++ {
		switch tokens[i] {
		case "--table":
			h.Table = tokens[i+1]
			i++
		case "--event":
			h.Event = tokens[i+1]
			i++
		case "--timing":
			h.Timing = tokens[i+1]
			i++
		case "--scope":
			h.Scope = tokens[i+1]
			i++
		case "--name":
			h.Name = tokens[i+1]
			i++
		case "--description":
			h.Description = tokens[i+1]
			i++
		case "--file":
			file = tokens[i+1]
			i++
		case "--source":
			h.Source = tokens[i+1]
			i++
		}
	}
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			s.shellError(err)
			return
		}
		h.Source = string(b)
	}
	if h.Source == "" {
		s.shellError(fmt.Errorf("a hook body is required: pass --file PATH or --source 'LUA'"))
		return
	}
	created, err := hooks.Create(s.DB, h)
	if err != nil {
		s.shellError(err)
		return
	}
	fmt.Fprintf(s.Out, "created hook %d on %s (%s %s)\n", created.ID, created.Table, created.Timing, created.Event)
}

func (s *State) hooksEdit(tokens []string) {
	if !s.hooksRequireWrite() {
		return
	}
	id, err := hookArgID(tokens)
	if err != nil {
		s.shellError(err)
		return
	}
	var source string
	for i := 1; i+1 < len(tokens); i++ {
		switch tokens[i] {
		case "--file":
			b, err := os.ReadFile(tokens[i+1])
			if err != nil {
				s.shellError(err)
				return
			}
			source = string(b)
			i++
		case "--source":
			source = tokens[i+1]
			i++
		}
	}
	if source == "" {
		s.shellError(fmt.Errorf("pass --file PATH or --source 'LUA' with the new hook body"))
		return
	}
	if _, err := hooks.Update(s.DB, id, hooks.Patch{Source: &source}); err != nil {
		s.shellError(err)
		return
	}
	fmt.Fprintf(s.Out, "updated hook %d\n", id)
}

// hooksTest runs `.hooks test ID '{"old":{...},"new":{...}}'` as a dry run:
// the Lua source runs against the supplied sample data with no trigger
// firing and no real row touched.
func (s *State) hooksTest(tokens []string) {
	id, err := hookArgID(tokens)
	if err != nil {
		s.shellError(err)
		return
	}
	var payload struct {
		Old map[string]any `json:"old"`
		New map[string]any `json:"new"`
	}
	if len(tokens) > 1 && strings.TrimSpace(tokens[1]) != "" {
		if err := json.Unmarshal([]byte(tokens[1]), &payload); err != nil {
			s.shellError(fmt.Errorf("sample row JSON: %w", err))
			return
		}
	}
	h, err := hooks.Get(s.DB, id)
	if err != nil {
		s.shellError(err)
		return
	}
	cfg := hooks.Current()
	res := hooks.Run(h, payload.Old, payload.New, hooks.RunConfig{
		DB: s.DB, Write: s.Write, AllowNet: cfg.AllowNet, Record: true,
	})
	hooks.Drain()

	result := "nil"
	if res.Result != nil {
		result = strconv.FormatBool(*res.Result)
	}
	fmt.Fprintf(s.Out, "result:   %s\n", result)
	fmt.Fprintf(s.Out, "message:  %s\n", res.Message)
	fmt.Fprintf(s.Out, "duration: %dms\n", res.DurationMs)
	if res.Error != "" {
		fmt.Fprintf(s.Out, "error:    %s\n", res.Error)
	}
	for _, l := range res.Logs {
		fmt.Fprintf(s.Out, "log:      %s\n", l)
	}
}

func (s *State) hooksSetEnabled(tokens []string, enabled bool) {
	if !s.hooksRequireWrite() {
		return
	}
	id, err := hookArgID(tokens)
	if err != nil {
		s.shellError(err)
		return
	}
	if _, err := hooks.Update(s.DB, id, hooks.Patch{Enabled: &enabled}); err != nil {
		s.shellError(err)
		return
	}
	state := "disabled"
	if enabled {
		state = "enabled"
	}
	fmt.Fprintf(s.Out, "hook %d %s\n", id, state)
}

func (s *State) hooksRemove(tokens []string) {
	if !s.hooksRequireWrite() {
		return
	}
	id, err := hookArgID(tokens)
	if err != nil {
		s.shellError(err)
		return
	}
	if err := hooks.Delete(s.DB, id); err != nil {
		s.shellError(err)
		return
	}
	fmt.Fprintf(s.Out, "removed hook %d\n", id)
}

func (s *State) hooksLog(tokens []string) {
	id, err := hookArgID(tokens)
	if err != nil {
		s.shellError(err)
		return
	}
	runs, err := hooks.Logs(s.DB, id, 50, 0)
	if err != nil {
		s.shellError(err)
		return
	}
	if len(runs) == 0 {
		fmt.Fprintln(s.Out, "no recorded runs")
		return
	}
	for _, r := range runs {
		fmt.Fprintf(s.Out, "%s  %-14s success=%-5v %4dms  %s\n", r.RanAt, r.Event, r.Success, r.DurationMs, r.Error)
	}
}

// hookActionNames feeds .hooks tab-completion.
func hookActionNames() []string {
	return []string{"list", "create", "edit", "test", "enable", "disable", "rm", "log"}
}
