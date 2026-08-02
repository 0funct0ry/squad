package hooks

import (
	"database/sql"
	"encoding/json"
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// SandboxConfig controls what a hook's Lua state can reach.
type SandboxConfig struct {
	// DB is the connection db.query/db.exec run against. May be nil, in
	// which case both raise a Lua error.
	DB *sql.DB
	// InTrigger marks a run happening inside a SQLite trigger callback, so
	// db.exec must go straight to the deferred writer.
	InTrigger bool
	// Write mirrors the process's --write flag: db.exec is refused with a
	// READ_ONLY error when false.
	Write bool
	// AllowNet mirrors --allow-net: without it the http module's fields all
	// raise the "network access disabled" error.
	AllowNet bool
	// HookTable is the hook's own triggering table (recorded for error
	// messages/write scoping context).
	HookTable string
	// Logs collects the script's print() output.
	Logs *[]string
}

// dangerousGlobals are base-library globals removed from every hook state.
// gopher-lua's OpenBase installs load/loadstring/dofile/loadfile, which all
// reach outside the hook's own source, plus collectgarbage/newproxy/module
// which have no business in a hook.
var dangerousGlobals = []string{
	"dofile", "loadfile", "load", "loadstring", "dostring",
	"require", "module", "newproxy", "collectgarbage",
}

// NewSandbox builds a fresh gopher-lua state with only the allowlisted
// standard libraries (string/table/math plus a trimmed base) and squad's own
// json/db/http modules. os, io, package, debug and coroutine are never
// opened under any flag combination.
func NewSandbox(cfg SandboxConfig) *lua.LState {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})

	for _, lib := range []struct {
		name string
		fn   lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.StringLibName, lua.OpenString},
		{lua.TabLibName, lua.OpenTable},
		{lua.MathLibName, lua.OpenMath},
	} {
		L.Push(L.NewFunction(lib.fn))
		L.Push(lua.LString(lib.name))
		L.Call(1, 0)
	}

	for _, g := range dangerousGlobals {
		L.SetGlobal(g, lua.LNil)
	}
	// OpenBase leaves the package table behind as a side effect of loading;
	// make sure neither it nor anything else escaping the allowlist is
	// reachable.
	for _, g := range []string{"package", "os", "io", "debug", "coroutine"} {
		L.SetGlobal(g, lua.LNil)
	}

	logs := cfg.Logs
	L.SetGlobal("print", L.NewFunction(func(l *lua.LState) int {
		parts := make([]string, 0, l.GetTop())
		for i := 1; i <= l.GetTop(); i++ {
			parts = append(parts, l.ToStringMeta(l.Get(i)).String())
		}
		if logs != nil {
			*logs = append(*logs, joinSpace(parts))
		}
		return 0
	}))

	registerJSONModule(L)
	registerDBModule(L, cfg)
	registerHTTPModule(L, cfg)

	return L
}

func joinSpace(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "\t"
		}
		out += p
	}
	return out
}

// CompileCheck parses a hook's Lua source without running it, so a syntax
// error is reported at save time rather than at first fire.
func CompileCheck(source string) error {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()
	if _, err := L.LoadString(source); err != nil {
		return fmt.Errorf("lua syntax error: %v", err)
	}
	return nil
}

// ---- json module ----------------------------------------------------------

func registerJSONModule(L *lua.LState) {
	mod := L.NewTable()
	L.SetField(mod, "encode", L.NewFunction(func(l *lua.LState) int {
		v := luaToGo(l.Get(1))
		b, err := json.Marshal(v)
		if err != nil {
			l.RaiseError("json.encode: %v", err)
			return 0
		}
		l.Push(lua.LString(string(b)))
		return 1
	}))
	L.SetField(mod, "decode", L.NewFunction(func(l *lua.LState) int {
		var v any
		if err := json.Unmarshal([]byte(l.CheckString(1)), &v); err != nil {
			l.RaiseError("json.decode: %v", err)
			return 0
		}
		l.Push(goToLua(l, v))
		return 1
	}))
	L.SetGlobal("json", mod)
}

// goToLua converts a decoded-JSON Go value into its Lua equivalent.
func goToLua(L *lua.LState, v any) lua.LValue {
	switch t := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(t)
	case float64:
		return lua.LNumber(t)
	case int64:
		return lua.LNumber(t)
	case int:
		return lua.LNumber(t)
	case string:
		return lua.LString(t)
	case []byte:
		return lua.LString(string(t))
	case []any:
		tbl := L.NewTable()
		for i, e := range t {
			tbl.RawSetInt(i+1, goToLua(L, e))
		}
		return tbl
	case map[string]any:
		tbl := L.NewTable()
		for k, e := range t {
			tbl.RawSetString(k, goToLua(L, e))
		}
		return tbl
	default:
		return lua.LString(fmt.Sprint(t))
	}
}

// luaToGo converts a Lua value back to a Go value. Tables whose keys are a
// dense 1..n integer run become slices; everything else becomes a map.
func luaToGo(v lua.LValue) any {
	switch t := v.(type) {
	case *lua.LNilType:
		return nil
	case lua.LBool:
		return bool(t)
	case lua.LNumber:
		f := float64(t)
		if f == float64(int64(f)) {
			return int64(f)
		}
		return f
	case lua.LString:
		return string(t)
	case *lua.LTable:
		maxN := t.Len()
		if maxN > 0 {
			arr := make([]any, 0, maxN)
			dense := true
			for i := 1; i <= maxN; i++ {
				val := t.RawGetInt(i)
				if val == lua.LNil {
					dense = false
					break
				}
				arr = append(arr, luaToGo(val))
			}
			if dense {
				return arr
			}
		}
		m := map[string]any{}
		t.ForEach(func(k, val lua.LValue) {
			m[k.String()] = luaToGo(val)
		})
		return m
	default:
		return v.String()
	}
}
