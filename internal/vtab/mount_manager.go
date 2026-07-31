package vtab

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// CreateMount validates and creates a new mount: checks the alias against
// existing tables/views, registered module names, and other active mounts;
// issues the CREATE VIRTUAL TABLE DDL on a throwaway pinned connection to
// validate the module accepts these args and to read back its declared
// columns; then records the mount in store. The throwaway connection's temp
// table is gone the moment it's released — only the metadata survives, to be
// replayed by WithMounts on whatever connection a later query uses.
func CreateMount(ctx context.Context, database *sql.DB, store *MountStore, module, alias string, args map[string]string) (Mount, error) {
	if !Exists(module) {
		return Mount{}, fmt.Errorf("unknown module %q", module)
	}
	if err := store.ValidateAlias(alias); err != nil {
		return Mount{}, err
	}

	conn, err := database.Conn(ctx)
	if err != nil {
		return Mount{}, err
	}
	defer conn.Close()

	var exists string
	err = conn.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type IN ('table','view') AND name = ?", alias).Scan(&exists)
	if err == nil {
		return Mount{}, fmt.Errorf("alias %q collides with an existing table or view", alias)
	} else if err != sql.ErrNoRows {
		return Mount{}, err
	}

	// This connection may be a pooled one that previously created a
	// same-named temp table for an alias since unmounted (see
	// MountStore.everMounted) and never dropped on this specific
	// connection — drop it first so re-mounting the same alias name after
	// an unmount doesn't fail with "table already exists".
	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp.`+quoteIdentifier(alias)); err != nil {
		return Mount{}, fmt.Errorf("mount failed: %w", err)
	}

	stmt := BuildMountSQL(alias, module, args)
	if _, err := conn.ExecContext(ctx, stmt); err != nil {
		return Mount{}, fmt.Errorf("mount failed: %w", err)
	}

	cols, err := declaredColumns(ctx, conn, alias)
	if err != nil {
		return Mount{}, err
	}

	m := Mount{
		Alias:           alias,
		Module:          module,
		Args:            args,
		DeclaredColumns: cols,
		CreatedAt:       time.Now(),
	}
	store.Add(m)
	return m, nil
}

// declaredColumns reads back a just-created temp virtual table's column
// names via PRAGMA table_info on the connection that created it.
func declaredColumns(ctx context.Context, conn *sql.Conn, alias string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`PRAGMA temp.table_info(%s)`, quoteIdentifier(alias)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}

// DropMount unmounts by alias, returning false if it wasn't mounted.
func DropMount(store *MountStore, alias string) bool {
	return store.Remove(alias)
}

// PreviewMount returns declared columns and up to limit rows for an active
// mount, replaying it onto a fresh connection.
func PreviewMount(ctx context.Context, database *sql.DB, store *MountStore, alias string, limit int) ([]string, [][]any, error) {
	m, ok := store.Get(alias)
	if !ok {
		return nil, nil, fmt.Errorf("no active mount named %q", alias)
	}

	var cols []string
	var out [][]any
	err := WithMounts(ctx, database, store, func(conn *sql.Conn) error {
		rows, err := conn.QueryContext(ctx, fmt.Sprintf(`SELECT * FROM temp.%s LIMIT ?`, quoteIdentifier(alias)), limit)
		if err != nil {
			return err
		}
		defer rows.Close()

		cols, err = rows.Columns()
		if err != nil {
			return err
		}

		for rows.Next() {
			raw := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range raw {
				ptrs[i] = &raw[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				return err
			}
			out = append(out, raw)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, nil, err
	}
	if len(cols) == 0 {
		cols = m.DeclaredColumns
	}
	return cols, out, nil
}
