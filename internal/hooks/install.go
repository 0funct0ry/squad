package hooks

import (
	"database/sql"
	"fmt"
	"strings"
)

// invokeFuncName is the single process-global scalar SQL function every
// generated trigger body calls.
const invokeFuncName = "__squad_invoke_hook"

// isReadOnly reports whether an error is SQLite refusing a write because the
// connection was opened with mode=ro.
func isReadOnly(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "readonly")
}

func triggerName(id int64) string {
	return fmt.Sprintf("__squad_hook_%d", id)
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func quoteLiteral(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}

// TriggerDDL renders the CREATE TRIGGER that wires one hook to
// __squad_invoke_hook. Row data is marshalled by SQLite's own json_object()
// over the table's current columns; DELETE has no NEW and INSERT has no OLD,
// so those arguments are passed as NULL.
//
// SQLite has no FOR EACH STATEMENT triggers, so scope "statement" produces
// the same per-row trigger without the explicit FOR EACH ROW clause (SQLite
// treats it as row-level either way); the distinction is kept in metadata so
// the Lua side can tell the author what they asked for.
func TriggerDDL(h Hook, columns []string) string {
	oldArg, newArg := "NULL", "NULL"
	obj := jsonObjectExpr(columns, "NEW")
	oldObj := jsonObjectExpr(columns, "OLD")
	switch h.Event {
	case "insert":
		newArg = obj
	case "delete":
		oldArg = oldObj
	case "update":
		oldArg, newArg = oldObj, obj
	}

	forEach := " FOR EACH ROW"
	if h.Scope == "statement" {
		forEach = ""
	}

	return fmt.Sprintf("CREATE TRIGGER %s %s %s ON %s%s\nBEGIN\n  SELECT %s(%d, %s, %s);\nEND;",
		quoteIdent(triggerName(h.ID)),
		strings.ToUpper(h.Timing),
		strings.ToUpper(h.Event),
		quoteIdent(h.Table),
		forEach,
		invokeFuncName, h.ID, oldArg, newArg)
}

func jsonObjectExpr(columns []string, prefix string) string {
	if len(columns) == 0 {
		return "NULL"
	}
	parts := make([]string, 0, len(columns)*2)
	for _, c := range columns {
		parts = append(parts, quoteLiteral(c), prefix+"."+quoteIdent(c))
	}
	return "json_object(" + strings.Join(parts, ", ") + ")"
}

// InstallTriggers drops and (re)creates the SQL trigger backing every hook.
// Disabled hooks and, defensively, `before` hooks under --hook-mode async
// (which CRUD already rejects) are left dropped.
func InstallTriggers(d *sql.DB) error {
	if d == nil || !schemaExists(d) {
		return nil
	}
	all, err := List(d, "")
	if err != nil {
		return err
	}
	async := Mode() == "async"
	for _, h := range all {
		if _, err := d.Exec(`DROP TRIGGER IF EXISTS ` + quoteIdent(triggerName(h.ID))); err != nil {
			if isReadOnly(err) {
				// Read-only session (no --write). The triggers are persistent
				// schema objects already written by an earlier write session,
				// so there is nothing to re-install — hook management is
				// limited to listing, testing and log inspection.
				return nil
			}
			return fmt.Errorf("failed to drop trigger for hook %d: %w", h.ID, err)
		}
		if !h.Enabled {
			continue
		}
		if async && h.Timing == "before" {
			continue
		}
		cols, err := TableColumns(d, h.Table)
		if err != nil || len(cols) == 0 {
			// The hook's table doesn't exist (yet). Leave the trigger off
			// rather than failing the whole install pass.
			continue
		}
		if _, err := d.Exec(TriggerDDL(h, cols)); err != nil {
			return fmt.Errorf("failed to install trigger for hook %d (%s): %w", h.ID, h.Name, err)
		}
	}
	return nil
}
