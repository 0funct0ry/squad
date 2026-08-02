package hooks

import (
	lua "github.com/yuin/gopher-lua"
)

// registerDBModule installs the `db` module: db.query(sql, ...) always
// works; db.exec(sql, ...) requires the process to be running with --write
// and otherwise raises a READ_ONLY Lua error (fail closed, never a silent
// no-op — see PROMPTS.md M10c's write-scoping rule).
//
// Both are parameterized: the sql string is the template and the remaining
// arguments are bound, never concatenated.
func registerDBModule(L *lua.LState, cfg SandboxConfig) {
	mod := L.NewTable()

	L.SetField(mod, "query", L.NewFunction(func(l *lua.LState) int {
		if cfg.DB == nil {
			l.RaiseError("db.query: no database connection available")
			return 0
		}
		query := l.CheckString(1)
		args := luaVarArgs(l, 2)
		rows, err := cfg.DB.Query(query, args...)
		if err != nil {
			l.RaiseError("db.query: %v", err)
			return 0
		}
		defer rows.Close()
		cols, err := rows.Columns()
		if err != nil {
			l.RaiseError("db.query: %v", err)
			return 0
		}
		out := l.NewTable()
		n := 0
		for rows.Next() {
			holders := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range holders {
				ptrs[i] = &holders[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				l.RaiseError("db.query: %v", err)
				return 0
			}
			row := l.NewTable()
			for i, c := range cols {
				row.RawSetString(c, goToLua(l, holders[i]))
			}
			n++
			out.RawSetInt(n, row)
		}
		if err := rows.Err(); err != nil {
			l.RaiseError("db.query: %v", err)
			return 0
		}
		l.Push(out)
		return 1
	}))

	L.SetField(mod, "exec", L.NewFunction(func(l *lua.LState) int {
		if cfg.DB == nil {
			l.RaiseError("db.exec: no database connection available")
			return 0
		}
		if !cfg.Write {
			l.RaiseError("READ_ONLY: db.exec requires --write mode")
			return 0
		}
		query := l.CheckString(1)
		args := luaVarArgs(l, 2)
		// execOrDefer: inside a sync trigger callback the triggering
		// statement still holds SQLite's write lock, so an inline Exec fails
		// SQLITE_BUSY. That case is queued for the deferred writer (see the
		// package doc) rather than reported as a hook failure.
		deferredWrite, err := execOrDefer(cfg.DB, cfg.InTrigger, query, args...)
		if err != nil {
			l.RaiseError("db.exec: %v", err)
			return 0
		}
		if deferredWrite && cfg.Logs != nil {
			*cfg.Logs = append(*cfg.Logs, "db.exec deferred until the triggering statement released its write lock")
		}
		l.Push(lua.LTrue)
		return 1
	}))

	L.SetGlobal("db", mod)
}

func luaVarArgs(l *lua.LState, from int) []any {
	var args []any
	for i := from; i <= l.GetTop(); i++ {
		args = append(args, luaToGo(l.Get(i)))
	}
	return args
}
