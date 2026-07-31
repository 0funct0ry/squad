package importer

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
)

// BulkInsertRows inserts rows into table using the given ordered column
// list, on the caller-owned transaction tx. The caller is responsible for
// the transaction boundary (so a create-table + insert sequence, or an
// existing-table insert, can be committed or rolled back as one unit).
func BulkInsertRows(tx *sql.Tx, table string, columns []string, rows []map[string]any) (int64, error) {
	if len(columns) == 0 || len(rows) == 0 {
		return 0, nil
	}

	quotedCols := make([]string, len(columns))
	placeholders := make([]string, len(columns))
	for i, col := range columns {
		quotedCols[i] = db.QuoteIdentifier(col)
		placeholders[i] = "?"
	}

	stmt, err := tx.Prepare(fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		db.QuoteIdentifier(table),
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "),
	))
	if err != nil {
		return 0, fmt.Errorf("failed to prepare insert into %q: %w", table, err)
	}
	defer stmt.Close()

	var inserted int64
	for i, row := range rows {
		args := make([]any, len(columns))
		for j, col := range columns {
			args[j] = row[col]
		}
		res, err := stmt.Exec(args...)
		if err != nil {
			return inserted, fmt.Errorf("row %d: %w", i, err)
		}
		affected, _ := res.RowsAffected()
		inserted += affected
	}
	return inserted, nil
}
