package hooks

import (
	"context"
	"database/sql"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// scriptTimeout is the hard per-invocation budget, independent of
// --hook-mode (PROMPTS.md M10c).
const scriptTimeout = 2 * time.Second

// Result is one hook invocation's outcome.
type Result struct {
	// Result is the script's first return value when it is a boolean; nil
	// when the script returned nothing or a non-boolean.
	Result *bool `json:"result"`
	// Message is the script's optional second return value.
	Message string `json:"message"`
	// Logs is everything the script wrote with print().
	Logs []string `json:"logs"`
	// DurationMs is the wall-clock execution time.
	DurationMs int64 `json:"durationMs"`
	// Err is set when the script raised a Lua error (including READ_ONLY and
	// network-disabled failures).
	Err error `json:"-"`
	// Error is Err's message, for JSON responses.
	Error string `json:"error,omitempty"`
}

// Aborts reports whether this result should abort the triggering statement:
// only a `before` hook that explicitly returned false does.
func (r Result) Aborts() bool {
	return r.Result != nil && !*r.Result
}

// RunConfig carries the per-invocation environment.
type RunConfig struct {
	DB       *sql.DB
	Write    bool
	AllowNet bool
	// Record, when true, appends the outcome to __squad_hook_runs.
	Record bool
	// InTrigger marks a synchronous invocation from inside the SQLite
	// trigger callback, where the triggering statement still holds the write
	// lock and every write must be deferred (see the package doc).
	InTrigger bool
}

// Run executes a hook's Lua source against the supplied old/new row data in
// a fresh sandbox. It never panics out to the caller: a Lua error becomes
// Result.Err.
func Run(h Hook, oldRow, newRow map[string]any, rc RunConfig) Result {
	logs := []string{}
	L := NewSandbox(SandboxConfig{
		DB:        rc.DB,
		InTrigger: rc.InTrigger,
		Write:     rc.Write,
		AllowNet:  rc.AllowNet,
		HookTable: h.Table,
		Logs:      &logs,
	})
	defer L.Close()

	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	L.SetContext(ctx)

	setRowGlobal(L, "old", oldRow)
	setRowGlobal(L, "new", newRow)
	L.SetGlobal("hook", hookInfoTable(L, h))

	start := time.Now()
	err := L.DoString(h.Source)
	dur := time.Since(start).Milliseconds()

	res := Result{Logs: logs, DurationMs: dur}
	if err != nil {
		res.Err = err
		res.Error = err.Error()
	} else {
		top := L.GetTop()
		if top >= 1 {
			if b, ok := L.Get(1).(lua.LBool); ok {
				v := bool(b)
				res.Result = &v
			}
		}
		if top >= 2 {
			if s, ok := L.Get(2).(lua.LString); ok {
				res.Message = string(s)
			}
		}
	}

	if rc.Record {
		success := res.Err == nil && !res.Aborts()
		msg := res.Error
		if msg == "" && res.Aborts() {
			msg = res.Message
			if msg == "" {
				msg = "hook returned false"
			}
		}
		recordRun(rc.DB, rc.InTrigger, h.ID, h.Timing+" "+h.Event, success, msg, dur, logs)
	}
	return res
}

func setRowGlobal(L *lua.LState, name string, row map[string]any) {
	if row == nil {
		L.SetGlobal(name, lua.LNil)
		return
	}
	tbl := L.NewTable()
	for k, v := range row {
		tbl.RawSetString(k, goToLua(L, v))
	}
	L.SetGlobal(name, tbl)
}

func hookInfoTable(L *lua.LState, h Hook) *lua.LTable {
	t := L.NewTable()
	t.RawSetString("id", lua.LNumber(h.ID))
	t.RawSetString("table", lua.LString(h.Table))
	t.RawSetString("event", lua.LString(h.Event))
	t.RawSetString("timing", lua.LString(h.Timing))
	t.RawSetString("scope", lua.LString(h.Scope))
	t.RawSetString("name", lua.LString(h.Name))
	return t
}
