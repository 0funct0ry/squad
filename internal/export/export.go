package export

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// RowSource is a function that returns the next row of data.
// It returns io.EOF (or a wrapped error containing io.EOF) when there are no more rows.
type RowSource func() ([]any, error)

// ExportCSV streams rows to CSV format per RFC 4180.
func ExportCSV(columns []string, source RowSource, w io.Writer, headers bool) error {
	csvWriter := csv.NewWriter(w)

	if headers {
		if err := csvWriter.Write(columns); err != nil {
			return fmt.Errorf("failed to write CSV headers: %w", err)
		}
	}

	record := make([]string, len(columns))
	for {
		row, err := source()
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "EOF") {
				break
			}
			return err
		}

		for i, val := range row {
			if val == nil {
				record[i] = ""
			} else if b, ok := val.([]byte); ok {
				record[i] = base64.StdEncoding.EncodeToString(b)
			} else {
				record[i] = fmt.Sprintf("%v", val)
			}
		}

		if err := csvWriter.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	csvWriter.Flush()
	return csvWriter.Error()
}

// ExportJSON streams rows as a JSON array of objects.
func ExportJSON(columns []string, source RowSource, w io.Writer) error {
	if _, err := io.WriteString(w, "[\n"); err != nil {
		return err
	}

	first := true
	for {
		row, err := source()
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "EOF") {
				break
			}
			return err
		}

		if !first {
			if _, err := io.WriteString(w, ",\n"); err != nil {
				return err
			}
		}
		first = false

		obj := make(map[string]any, len(columns))
		for i, col := range columns {
			val := row[i]
			if val == nil {
				obj[col] = nil
			} else if b, ok := val.([]byte); ok {
				obj[col] = base64.StdEncoding.EncodeToString(b)
			} else {
				obj[col] = val
			}
		}

		// Use Marshal to avoid the encoder's trailing newline, or keep it.
		// Let's use Marshal and then Write so it doesn't print extra newlines per-row,
		// keeping it clean:
		data, err := json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON row: %w", err)
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}

	if _, err := io.WriteString(w, "\n]"); err != nil {
		return err
	}
	return nil
}

// ExportSQL streams rows as a sequence of SQL INSERT statements.
func ExportSQL(tableName string, columns []string, source RowSource, w io.Writer, includeSchema bool, ddl string) error {
	if includeSchema && ddl != "" {
		terminated := strings.TrimRight(ddl, " \t\n;")
		if _, err := io.WriteString(w, terminated+";\n\n"); err != nil {
			return err
		}
	}

	// Prepare table and column names quoted
	quotedTable := QuoteIdentifier(tableName)
	quotedCols := make([]string, len(columns))
	for i, col := range columns {
		quotedCols[i] = QuoteIdentifier(col)
	}
	colsList := strings.Join(quotedCols, ",")

	for {
		row, err := source()
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "EOF") {
				break
			}
			return err
		}

		valsList := make([]string, len(row))
		for i, val := range row {
			if val == nil {
				valsList[i] = "NULL"
			} else if b, ok := val.([]byte); ok {
				valsList[i] = fmt.Sprintf("X'%s'", strings.ToUpper(hex.EncodeToString(b)))
			} else if s, ok := val.(string); ok {
				valsList[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(s, "'", "''"))
			} else {
				valsList[i] = fmt.Sprintf("%v", val)
			}
		}

		stmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);\n", quotedTable, colsList, strings.Join(valsList, ","))
		if _, err := io.WriteString(w, stmt); err != nil {
			return fmt.Errorf("failed to write SQL insert: %w", err)
		}
	}

	return nil
}

// QuoteIdentifier quotes a SQLite identifier (table or column name) with double quotes.
// Any embedded double quotes are escaped by doubling them.
func QuoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
