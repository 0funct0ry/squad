package hooks

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"sync"

	sqlite "modernc.org/sqlite"
)

var registerOnce sync.Once
var registerErr error

// RegisterAll registers the single process-global scalar SQL function every
// hook trigger calls. modernc.org/sqlite's function registry is
// process-global and must be populated before the first sql.Open, so this is
// wired through db.RegisterHooksHook (set in cmd/hooks.go's init) exactly
// like udf.RegisterAll and vtab.Register. sync.Once-guarded because a
// duplicate name is an error.
func RegisterAll() error {
	registerOnce.Do(func() {
		registerErr = sqlite.RegisterScalarFunction(invokeFuncName, 3, invokeHook)
	})
	return registerErr
}

// invokeHook is __squad_invoke_hook(hook_id, old_json, new_json).
//
// Returning a non-nil error here aborts the statement that fired the trigger
// (verified against modernc.org/sqlite v1.54.0 — see the package doc), which
// is how `before` hooks reject a write with a custom message.
func invokeHook(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("%s: expected 3 arguments", invokeFuncName)
	}
	id, ok := toInt64(args[0])
	if !ok {
		return nil, fmt.Errorf("%s: hook id must be an integer", invokeFuncName)
	}
	h, ok := cachedHook(id)
	if !ok || !h.Enabled {
		return "skipped", nil
	}

	oldRow := decodeRowJSON(args[1])
	newRow := decodeRowJSON(args[2])

	c := Current()
	if c.Mode == "async" && h.Timing == "after" {
		enqueueAsync(h, oldRow, newRow)
		return "queued", nil
	}

	res := Run(h, oldRow, newRow, RunConfig{DB: DB(), Write: c.Write, AllowNet: c.AllowNet, Record: true, InTrigger: true})

	if h.Timing == "before" {
		if res.Err != nil {
			return nil, res.Err
		}
		if res.Aborts() {
			msg := res.Message
			if msg == "" {
				msg = fmt.Sprintf("hook %q rejected the %s", hookLabel(h), h.Event)
			}
			return nil, fmt.Errorf("%s", msg)
		}
	}
	// `after` hooks can't abort — the write already happened — so their
	// errors are recorded in the execution log only.
	return "ok", nil
}

func hookLabel(h Hook) string {
	if h.Name != "" {
		return h.Name
	}
	return fmt.Sprintf("#%d", h.ID)
}

func toInt64(v driver.Value) (int64, bool) {
	switch t := v.(type) {
	case int64:
		return t, true
	case float64:
		return int64(t), true
	case string:
		var n int64
		if _, err := fmt.Sscan(t, &n); err == nil {
			return n, true
		}
	}
	return 0, false
}

func decodeRowJSON(v driver.Value) map[string]any {
	var s string
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		s = t
	case []byte:
		s = string(t)
	default:
		return nil
	}
	if s == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}
