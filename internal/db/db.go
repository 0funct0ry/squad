package db

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

// OpenDB opens a SQLite database using modernc.org/sqlite.
// If readOnly is true, it opens the database in read-only mode using mode=ro DSN parameter.
func OpenDB(path string, readOnly bool) (*sql.DB, error) {
	dsn := path
	if readOnly {
		// SQLite expects URI filename for query parameters like mode=ro
		if !strings.HasPrefix(dsn, "file:") && dsn != ":memory:" {
			dsn = "file:" + dsn + "?mode=ro"
		} else if dsn == ":memory:" {
			dsn = "file::memory:?mode=ro"
		} else {
			if strings.Contains(dsn, "?") {
				dsn = dsn + "&mode=ro"
			} else {
				dsn = dsn + "?mode=ro"
			}
		}
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
