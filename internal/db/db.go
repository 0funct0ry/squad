package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Queryer is the subset of *sql.DB's read API that GetTables/GetTableSchema/
// BuildTableQuery/GetTableRows need. *sql.DB satisfies it as-is, so every
// existing call site keeps compiling unchanged; WrapConn additionally lets a
// pinned *sql.Conn (from vtab.WithMounts, so mounted tables are visible on
// the connection they were replayed onto) satisfy it too.
type Queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// connQueryer adapts a *sql.Conn to the Queryer interface via the context
// variants (sql.Conn has no non-context Query/QueryRow methods).
type connQueryer struct{ conn *sql.Conn }

func (c *connQueryer) Query(query string, args ...any) (*sql.Rows, error) {
	return c.conn.QueryContext(context.Background(), query, args...)
}

func (c *connQueryer) QueryRow(query string, args ...any) *sql.Row {
	return c.conn.QueryRowContext(context.Background(), query, args...)
}

// WrapConn adapts a *sql.Conn (typically one pinned by vtab.WithMounts so
// mounted tables are visible) to Queryer.
func WrapConn(conn *sql.Conn) Queryer {
	return &connQueryer{conn: conn}
}

// RegisterModulesHook is called once at the top of every OpenDB call, before
// sql.Open. It exists so internal/vtab (which depends on internal/seed,
// which depends on this package) can be wired in without an import cycle:
// cmd/ sets this to vtab.Register in an init(), so it's always in place
// however the process opens its first database — cmd/root.go, cmd/cli.go,
// or the sandbox Registry's registerOpened/Rescan paths. vtab.Register is
// itself a sync.Once-guarded no-op unless vtab.Configure(true, ...) was
// called (i.e. --modules was passed), so this is a no-op by default.
var RegisterModulesHook func() error

// RegisterUDFHook is called once at the top of every OpenDB call, before
// sql.Open, the same way RegisterModulesHook is. cmd/ sets this to
// udf.RegisterAll in an init(). Unlike modules, the curated UDF library
// (internal/udf, M10b) is always-on — no enable flag gates it — but the
// indirection still avoids internal/db needing to import internal/udf
// directly, and udf.RegisterAll is itself sync.Once-guarded since
// modernc.org/sqlite's function registration is process-global and errors
// on a duplicate name.
var RegisterUDFHook func() error

// RegisterHooksHook is called once at the top of every OpenDB call, before
// sql.Open, exactly like RegisterModulesHook/RegisterUDFHook. cmd/ sets it to
// hooks.RegisterAll in an init(). It registers the single process-global
// __squad_invoke_hook scalar function that every Lua-hook trigger body calls
// (M10c); it is sync.Once-guarded on the hooks side, and registering the
// function is harmless when no hooks are defined.
var RegisterHooksHook func() error

// OpenDB opens a SQLite database using modernc.org/sqlite.
// If readOnly is true, it opens the database in read-only mode using mode=ro DSN parameter.
func OpenDB(path string, readOnly bool) (*sql.DB, error) {
	if RegisterModulesHook != nil {
		if err := RegisterModulesHook(); err != nil {
			return nil, fmt.Errorf("failed to register virtual table modules: %w", err)
		}
	}
	if RegisterUDFHook != nil {
		if err := RegisterUDFHook(); err != nil {
			return nil, fmt.Errorf("failed to register SQL functions: %w", err)
		}
	}
	if RegisterHooksHook != nil {
		if err := RegisterHooksHook(); err != nil {
			return nil, fmt.Errorf("failed to register hook dispatcher: %w", err)
		}
	}

	dsn := path
	if readOnly {
		// In read-only mode, SQLite's own mode=ro URI handling refuses to
		// create a missing file, which otherwise surfaces as an opaque
		// "unable to open database file (14)" error. Check up front so we
		// can give a clear, actionable message instead.
		if dsn != ":memory:" && !strings.HasPrefix(dsn, "file:") {
			if _, statErr := os.Stat(dsn); os.IsNotExist(statErr) {
				return nil, fmt.Errorf("database file %q does not exist (use --write to create it)", dsn)
			}
		}
	}

	// SQLite expects a URI filename for query parameters like mode=ro or
	// _pragma. database/sql pools multiple underlying connections, and each
	// is opened independently by the driver, so a PRAGMA executed once after
	// Ping only takes effect on that single connection — _pragma in the DSN
	// is applied by the driver to every connection it opens, which is what
	// foreign key enforcement (and mode=ro) actually needs.
	// busy_timeout matters for M10c's Lua hooks: an `after` hook's db.exec
	// (and squad's own execution-log write) is executed by a background
	// goroutine on a second pooled connection right after the triggering
	// statement commits, so the next statement the user runs can briefly
	// collide with it. Without a busy timeout that surfaces as a spurious
	// SQLITE_BUSY; with one, SQLite simply waits out the other writer.
	params := []string{"_pragma=foreign_keys(1)", "_pragma=busy_timeout(5000)"}
	if readOnly {
		params = append(params, "mode=ro")
	}

	if !strings.HasPrefix(dsn, "file:") && dsn != ":memory:" {
		dsn = "file:" + dsn + "?" + strings.Join(params, "&")
	} else if dsn == ":memory:" {
		dsn = "file::memory:?" + strings.Join(params, "&")
	} else {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn = dsn + sep + strings.Join(params, "&")
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Ping to verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// QuoteIdentifier wraps a schema/table/column name in double quotes and escapes any internal double quotes.
func QuoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// Meta retrieves the SQLite version and the size of the database file on disk.
func Meta(db *sql.DB, path string) (sqliteVersion string, sizeBytes int64, err error) {
	// Query sqlite version
	err = db.QueryRow("SELECT sqlite_version()").Scan(&sqliteVersion)
	if err != nil {
		return "", 0, fmt.Errorf("failed to query sqlite version: %w", err)
	}

	// If it's a memory database, return 0 size
	if path == ":memory:" || strings.Contains(path, "mode=memory") {
		return sqliteVersion, 0, nil
	}

	// Get file size on disk
	info, err := os.Stat(path)
	if err != nil {
		// File might not exist yet, or it's a special URL
		return sqliteVersion, 0, nil
	}

	return sqliteVersion, info.Size(), nil
}

type DBMeta struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	Mode          string `json:"mode"`
	SqliteVersion string `json:"sqliteVersion"`
	SizeBytes     int64  `json:"sizeBytes"`
	PageSize      int64  `json:"pageSize"`
	PageCount     int64  `json:"pageCount"`
	Encoding      string `json:"encoding"`
	JournalMode   string `json:"journalMode"`
	TableCount    int    `json:"tableCount"`
	ViewCount     int    `json:"viewCount"`
}

// GetDBMeta fetches extended file- and pragma-level metadata for the database.
func GetDBMeta(db *sql.DB, dbPath string, write bool) (*DBMeta, error) {
	sqliteVer, size, err := Meta(db, dbPath)
	if err != nil {
		return nil, err
	}

	name := dbPath
	if dbPath != ":memory:" && !strings.HasPrefix(dbPath, "file:") {
		name = filepath.Base(dbPath)
	}

	mode := "ro"
	if write {
		mode = "rw"
	}

	var pageSize int64
	if err := db.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		return nil, fmt.Errorf("failed to query page size: %w", err)
	}

	var pageCount int64
	if err := db.QueryRow("PRAGMA page_count").Scan(&pageCount); err != nil {
		return nil, fmt.Errorf("failed to query page count: %w", err)
	}

	var encoding string
	if err := db.QueryRow("PRAGMA encoding").Scan(&encoding); err != nil {
		return nil, fmt.Errorf("failed to query encoding: %w", err)
	}

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		return nil, fmt.Errorf("failed to query journal mode: %w", err)
	}
	journalMode = strings.ToLower(journalMode)

	var tableCount, viewCount int
	rows, err := db.Query(`
		SELECT type, COUNT(*) 
		FROM sqlite_master 
		WHERE (type = 'table' OR type = 'view')
		  AND name NOT LIKE 'sqlite_%'
		  AND name NOT LIKE '!_!_squad!_%' ESCAPE '!'
		GROUP BY type
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query table and view counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ttype string
		var count int
		if err := rows.Scan(&ttype, &count); err != nil {
			return nil, fmt.Errorf("failed to scan type count: %w", err)
		}
		if ttype == "table" {
			tableCount = count
		} else if ttype == "view" {
			viewCount = count
		}
	}

	return &DBMeta{
		Name:          name,
		Path:          dbPath,
		Mode:          mode,
		SqliteVersion: sqliteVer,
		SizeBytes:     size,
		PageSize:      pageSize,
		PageCount:     pageCount,
		Encoding:      encoding,
		JournalMode:   journalMode,
		TableCount:    tableCount,
		ViewCount:     viewCount,
	}, nil
}
