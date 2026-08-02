// Package hooks implements M10c's Lua trigger hooks: sandboxed gopher-lua
// scripts attached to a table's INSERT/UPDATE/DELETE through real SQLite
// triggers whose bodies call a single process-global scalar SQL function,
// __squad_invoke_hook.
//
// # Two behaviours confirmed against modernc.org/sqlite v1.54.0 by spike
//
//  1. Returning a non-nil Go error from a scalar function registered with
//     sqlite.RegisterScalarFunction aborts the statement that (transitively)
//     invoked it. A BEFORE INSERT trigger body calling __squad_invoke_hook
//     therefore aborts the INSERT and surfaces the Go error's text as
//     "SQL logic error: <message> (1)" to whatever ran the write. This is
//     what makes `before` hooks able to reject a write with a custom
//     message; no special-casing is needed anywhere else in squad.
//
//  2. Reentrancy: a *read* (db.Query/QueryRow) issued on the same *sql.DB
//     from inside the callback works — database/sql hands out a second
//     pooled connection, and SQLite allows concurrent readers. A *write*
//     (db.Exec) from inside the callback, however, fails immediately with
//     "database is locked (5) (SQLITE_BUSY)": the triggering statement holds
//     the write lock on another connection and cannot release it until the
//     callback returns, so waiting would deadlock.
//
//     Fallback adopted because of (2): every write issued from inside a hook
//     callback (both the user's db.exec and squad's own execution-log
//     bookkeeping) goes through execOrDefer — one inline attempt, and on a
//     busy/locked error the statement is queued onto a package-level
//     deferred-writer goroutine that retries with a short backoff. The
//     triggering statement releases its lock microseconds later, so the
//     deferred write lands essentially immediately, but it is not part of
//     the triggering transaction. Drain() waits for the queue so tests (and
//     the CLI, which exits right after) can observe the result.
package hooks

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Valid enum values for a hook's event/timing/scope.
var (
	validEvents = []string{"insert", "update", "delete"}
	validTiming = []string{"before", "after"}
	validScopes = []string{"row", "statement"}
)

// Hook is one Lua trigger hook definition.
type Hook struct {
	ID          int64  `json:"id"`
	Table       string `json:"table"`
	Event       string `json:"event"`
	Timing      string `json:"timing"`
	Scope       string `json:"scope"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// RunRecord is one recorded execution of a hook.
type RunRecord struct {
	ID         int64    `json:"id"`
	HookID     int64    `json:"hookId"`
	RanAt      string   `json:"ranAt"`
	Event      string   `json:"event"`
	Success    bool     `json:"success"`
	Error      string   `json:"error"`
	DurationMs int64    `json:"durationMs"`
	Logs       []string `json:"logs"`
}

// maxRunsPerHook caps __squad_hook_runs per hook (spec: last 200 runs).
const maxRunsPerHook = 200

const createSchemaSQL = `
CREATE TABLE IF NOT EXISTS __squad_hooks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	table_name TEXT NOT NULL,
	event TEXT NOT NULL,
	timing TEXT NOT NULL,
	scope TEXT NOT NULL,
	name TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	source TEXT NOT NULL DEFAULT '',
	enabled INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS __squad_hook_runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	hook_id INTEGER NOT NULL,
	ran_at TEXT NOT NULL DEFAULT (datetime('now')),
	event TEXT NOT NULL DEFAULT '',
	success INTEGER NOT NULL DEFAULT 1,
	error TEXT NOT NULL DEFAULT '',
	duration_ms INTEGER NOT NULL DEFAULT 0,
	logs TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS __squad_hook_runs_hook_idx ON __squad_hook_runs(hook_id, id);
`

// Config captures the process-wide flags hooks honor.
type Config struct {
	Mode     string // "sync" | "async"
	AllowNet bool
	Write    bool
	// Enabled mirrors --hooks. Its zero value is false, so a process that
	// never calls Configure (i.e. --hooks was not passed) leaves the whole
	// feature off by default: RegisterAll skips registering
	// __squad_invoke_hook, so any pre-existing hook-backed trigger fails
	// with "no such function" on the next write rather than silently
	// keeping working — a deliberate fail-closed choice so a restart
	// without --hooks can't run Lua on a hunch of leftover configuration.
	Enabled bool
}

var (
	cfgMu     sync.RWMutex
	cfg       = Config{Mode: "sync"}
	activeDB  *sql.DB
	cacheMu   sync.RWMutex
	hookCache = map[int64]Hook{}
)

// Configure records the resolved --hooks/--hook-mode/--allow-net/--write
// flags. Call before Init. Not calling it at all (i.e. --hooks was never
// passed) is equivalent to Configure(..., enabled: false) since the package
// var's zero value already has Enabled == false.
func Configure(mode string, allowNet, write, enabled bool) {
	if mode != "async" {
		mode = "sync"
	}
	cfgMu.Lock()
	cfg = Config{Mode: mode, AllowNet: allowNet, Write: write, Enabled: enabled}
	cfgMu.Unlock()
}

// Current returns the resolved configuration.
func Current() Config {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return cfg
}

// Mode reports "sync" or "async".
func Mode() string { return Current().Mode }

// Enabled reports whether --hooks was passed for this process.
func Enabled() bool { return Current().Enabled }

// DB returns the database hooks are attached to (nil before Init).
func DB() *sql.DB {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return activeDB
}

func setDB(d *sql.DB) {
	cfgMu.Lock()
	activeDB = d
	cfgMu.Unlock()
}

// Init attaches hooks to an open database: it creates the metadata tables
// (when the connection is writable), loads the definition cache, starts the
// deferred writer and, in async mode, the worker pool, and re-installs every
// enabled hook's trigger. Safe to call when the DB is read-only — schema
// creation is then skipped and hook management is limited to reads/tests.
//
// A no-op unless Configure(..., enabled: true) was already called — mirrors
// RegisterAll's internal gate so callers (cmd/root.go, cmd/cli.go) can call
// this unconditionally rather than wrapping every call site in `if
// cfg.HooksEnabled`.
func Init(d *sql.DB) error {
	if !Enabled() {
		return nil
	}
	setDB(d)
	startDeferredWriter(d)
	startAsyncWorkers()
	if err := EnsureSchema(d); err != nil {
		// Read-only databases can't have the metadata tables created; that's
		// not fatal, it just means there are no hooks to run.
		return nil //nolint:nilerr // intentional: read-only DBs have no hooks
	}
	if err := reloadCache(d); err != nil {
		return err
	}
	return InstallTriggers(d)
}

// schemaExists reports whether __squad_hooks is present.
func schemaExists(d *sql.DB) bool {
	var n int
	if err := d.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='__squad_hooks'`).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

// EnsureSchema creates the two metadata tables if they don't exist yet.
func EnsureSchema(d *sql.DB) error {
	if d == nil {
		return fmt.Errorf("no database open")
	}
	if schemaExists(d) {
		return nil
	}
	if _, err := d.Exec(createSchemaSQL); err != nil {
		return fmt.Errorf("failed to create hook metadata tables: %w", err)
	}
	return nil
}

func reloadCache(d *sql.DB) error {
	all, err := List(d, "")
	if err != nil {
		return err
	}
	m := make(map[int64]Hook, len(all))
	for _, h := range all {
		m[h.ID] = h
	}
	cacheMu.Lock()
	hookCache = m
	cacheMu.Unlock()
	return nil
}

func cachedHook(id int64) (Hook, bool) {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	h, ok := hookCache[id]
	return h, ok
}

// ValidateEnums checks event/timing/scope against the fixed enums and
// rejects `before` timing when the process runs --hook-mode async.
func ValidateEnums(event, timing, scope string) error {
	if !contains(validEvents, event) {
		return fmt.Errorf("invalid event %q (want one of %s)", event, strings.Join(validEvents, ", "))
	}
	if !contains(validTiming, timing) {
		return fmt.Errorf("invalid timing %q (want one of %s)", timing, strings.Join(validTiming, ", "))
	}
	if !contains(validScopes, scope) {
		return fmt.Errorf("invalid scope %q (want one of %s)", scope, strings.Join(validScopes, ", "))
	}
	if timing == "before" && Mode() == "async" {
		return fmt.Errorf(`timing "before" is not supported under --hook-mode async: a before hook can only be meaningful when it can block the write synchronously`)
	}
	return nil
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// List returns hook definitions, optionally filtered to one table. Source is
// included; callers that want the light list payload strip it.
func List(d *sql.DB, table string) ([]Hook, error) {
	if d == nil {
		return nil, fmt.Errorf("no database open")
	}
	if !schemaExists(d) {
		return []Hook{}, nil
	}
	q := `SELECT id, table_name, event, timing, scope, name, description, source, enabled, created_at, updated_at FROM __squad_hooks`
	var args []any
	if table != "" {
		q += ` WHERE table_name = ?`
		args = append(args, table)
	}
	q += ` ORDER BY id`
	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Hook{}
	for rows.Next() {
		h, err := scanHook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanHook(s scanner) (Hook, error) {
	var h Hook
	var enabled int
	err := s.Scan(&h.ID, &h.Table, &h.Event, &h.Timing, &h.Scope, &h.Name, &h.Description, &h.Source, &enabled, &h.CreatedAt, &h.UpdatedAt)
	h.Enabled = enabled != 0
	return h, err
}

// Get returns one hook by id.
func Get(d *sql.DB, id int64) (Hook, error) {
	if d == nil {
		return Hook{}, fmt.Errorf("no database open")
	}
	if !schemaExists(d) {
		return Hook{}, fmt.Errorf("hook %d not found", id)
	}
	row := d.QueryRow(`SELECT id, table_name, event, timing, scope, name, description, source, enabled, created_at, updated_at FROM __squad_hooks WHERE id = ?`, id)
	h, err := scanHook(row)
	if err == sql.ErrNoRows {
		return Hook{}, fmt.Errorf("hook %d not found", id)
	}
	return h, err
}

// Create validates, compile-checks and stores a new hook, then installs its
// trigger.
func Create(d *sql.DB, h Hook) (Hook, error) {
	if d == nil {
		return Hook{}, fmt.Errorf("no database open")
	}
	h.Table = strings.TrimSpace(h.Table)
	h.Event = strings.ToLower(strings.TrimSpace(h.Event))
	h.Timing = strings.ToLower(strings.TrimSpace(h.Timing))
	h.Scope = strings.ToLower(strings.TrimSpace(h.Scope))
	if h.Scope == "" {
		h.Scope = "row"
	}
	if h.Table == "" {
		return Hook{}, fmt.Errorf("table is required")
	}
	if err := ValidateEnums(h.Event, h.Timing, h.Scope); err != nil {
		return Hook{}, err
	}
	if err := CompileCheck(h.Source); err != nil {
		return Hook{}, err
	}
	if err := EnsureSchema(d); err != nil {
		return Hook{}, err
	}
	res, err := d.Exec(`INSERT INTO __squad_hooks (table_name, event, timing, scope, name, description, source, enabled) VALUES (?,?,?,?,?,?,?,?)`,
		h.Table, h.Event, h.Timing, h.Scope, h.Name, h.Description, h.Source, boolInt(h.Enabled))
	if err != nil {
		return Hook{}, err
	}
	id, _ := res.LastInsertId()
	h.ID = id
	if err := reloadCache(d); err != nil {
		return Hook{}, err
	}
	if err := InstallTriggers(d); err != nil {
		return Hook{}, err
	}
	return Get(d, id)
}

// Update applies a partial patch to an existing hook and re-installs it.
type Patch struct {
	Table       *string
	Event       *string
	Timing      *string
	Scope       *string
	Name        *string
	Description *string
	Source      *string
	Enabled     *bool
}

// Update mutates the stored hook and re-installs the underlying trigger.
func Update(d *sql.DB, id int64, p Patch) (Hook, error) {
	h, err := Get(d, id)
	if err != nil {
		return Hook{}, err
	}
	if p.Table != nil {
		h.Table = strings.TrimSpace(*p.Table)
	}
	if p.Event != nil {
		h.Event = strings.ToLower(strings.TrimSpace(*p.Event))
	}
	if p.Timing != nil {
		h.Timing = strings.ToLower(strings.TrimSpace(*p.Timing))
	}
	if p.Scope != nil {
		h.Scope = strings.ToLower(strings.TrimSpace(*p.Scope))
	}
	if p.Name != nil {
		h.Name = *p.Name
	}
	if p.Description != nil {
		h.Description = *p.Description
	}
	if p.Source != nil {
		h.Source = *p.Source
	}
	if p.Enabled != nil {
		h.Enabled = *p.Enabled
	}
	if h.Table == "" {
		return Hook{}, fmt.Errorf("table is required")
	}
	if err := ValidateEnums(h.Event, h.Timing, h.Scope); err != nil {
		return Hook{}, err
	}
	if err := CompileCheck(h.Source); err != nil {
		return Hook{}, err
	}
	if _, err := d.Exec(`UPDATE __squad_hooks SET table_name=?, event=?, timing=?, scope=?, name=?, description=?, source=?, enabled=?, updated_at=datetime('now') WHERE id=?`,
		h.Table, h.Event, h.Timing, h.Scope, h.Name, h.Description, h.Source, boolInt(h.Enabled), id); err != nil {
		return Hook{}, err
	}
	if err := reloadCache(d); err != nil {
		return Hook{}, err
	}
	if err := InstallTriggers(d); err != nil {
		return Hook{}, err
	}
	return Get(d, id)
}

// Delete drops the underlying SQL trigger and the metadata row.
func Delete(d *sql.DB, id int64) error {
	if _, err := Get(d, id); err != nil {
		return err
	}
	if _, err := d.Exec(`DROP TRIGGER IF EXISTS ` + triggerName(id)); err != nil {
		return err
	}
	if _, err := d.Exec(`DELETE FROM __squad_hooks WHERE id = ?`, id); err != nil {
		return err
	}
	if _, err := d.Exec(`DELETE FROM __squad_hook_runs WHERE hook_id = ?`, id); err != nil {
		return err
	}
	return reloadCache(d)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Logs returns a hook's execution history, newest first, with limit/offset
// pagination.
func Logs(d *sql.DB, id int64, limit, offset int) ([]RunRecord, error) {
	if d == nil {
		return nil, fmt.Errorf("no database open")
	}
	if !schemaExists(d) {
		return []RunRecord{}, nil
	}
	if limit <= 0 || limit > maxRunsPerHook {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := d.Query(`SELECT id, hook_id, ran_at, event, success, error, duration_ms, logs FROM __squad_hook_runs WHERE hook_id = ? ORDER BY id DESC LIMIT ? OFFSET ?`, id, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RunRecord{}
	for rows.Next() {
		var r RunRecord
		var success int
		var logs string
		if err := rows.Scan(&r.ID, &r.HookID, &r.RanAt, &r.Event, &success, &r.Error, &r.DurationMs, &logs); err != nil {
			return nil, err
		}
		r.Success = success != 0
		r.Logs = splitLogs(logs)
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountLogs returns the total number of recorded runs for a hook.
func CountLogs(d *sql.DB, id int64) (int, error) {
	if d == nil || !schemaExists(d) {
		return 0, nil
	}
	var n int
	err := d.QueryRow(`SELECT count(*) FROM __squad_hook_runs WHERE hook_id = ?`, id).Scan(&n)
	return n, err
}

// ClearLogs deletes every recorded run for a hook. The hook definition
// itself is untouched.
func ClearLogs(d *sql.DB, id int64) error {
	if d == nil {
		return fmt.Errorf("no database open")
	}
	if !schemaExists(d) {
		return nil
	}
	if _, err := Get(d, id); err != nil {
		return err
	}
	_, err := d.Exec(`DELETE FROM __squad_hook_runs WHERE hook_id = ?`, id)
	return err
}

func splitLogs(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, "\n")
}

// recordRun appends a run row and trims the per-hook history to
// maxRunsPerHook. Writes go through execOrDefer because recordRun is often
// called from inside a trigger callback (see the package doc).
func recordRun(d *sql.DB, inTrigger bool, hookID int64, event string, success bool, errMsg string, durationMs int64, logs []string) {
	if d == nil || !schemaExists(d) {
		return
	}
	execOrDefer(d, inTrigger,
		`INSERT INTO __squad_hook_runs (hook_id, ran_at, event, success, error, duration_ms, logs) VALUES (?,?,?,?,?,?,?)`,
		hookID, time.Now().UTC().Format("2006-01-02 15:04:05"), event, boolInt(success), errMsg, durationMs, strings.Join(logs, "\n"))
	execOrDefer(d, inTrigger,
		`DELETE FROM __squad_hook_runs WHERE hook_id = ? AND id NOT IN (SELECT id FROM __squad_hook_runs WHERE hook_id = ? ORDER BY id DESC LIMIT ?)`,
		hookID, hookID, maxRunsPerHook)
}

// TableColumns returns a table's column names in declaration order.
func TableColumns(d *sql.DB, table string) ([]string, error) {
	rows, err := d.Query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		cols = append(cols, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cols, nil
}
