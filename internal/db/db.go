package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// OpenDB opens a SQLite database using modernc.org/sqlite.
// If readOnly is true, it opens the database in read-only mode using mode=ro DSN parameter.
func OpenDB(path string, readOnly bool) (*sql.DB, error) {
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
	params := []string{"_pragma=foreign_keys(1)"}
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
		WHERE (type = 'table' OR type = 'view') AND name NOT LIKE 'sqlite_%' 
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
