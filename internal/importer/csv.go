package importer

import (
	"encoding/csv"
	"fmt"
	"io"
)

// ParseCSV reads a CSV file with a header row into a ParsedFile. All values
// are strings (CSV has no native type system); InferSchema does the value
// sampling for create-table-from-file.
func ParseCSV(r io.Reader) (ParsedFile, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // tolerate ragged rows; short rows leave trailing columns unset

	header, err := cr.Read()
	if err == io.EOF {
		return ParsedFile{}, fmt.Errorf("CSV file has no header row")
	}
	if err != nil {
		return ParsedFile{}, fmt.Errorf("failed to read CSV header: %w", err)
	}

	var rows []map[string]any
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ParsedFile{}, fmt.Errorf("failed to read CSV row: %w", err)
		}
		row := make(map[string]any, len(header))
		for i, col := range header {
			if i < len(record) {
				row[col] = record[i]
			} else {
				row[col] = nil
			}
		}
		rows = append(rows, row)
	}

	return ParsedFile{Columns: header, Rows: rows}, nil
}
