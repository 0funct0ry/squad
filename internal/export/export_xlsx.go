package export

import (
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ExportXLSX writes rows to a single-sheet workbook using excelize's
// row-streaming writer, keeping per-row memory bounded even though the
// final zip structure is necessarily written out in one Write call at the
// end (xlsx has no incremental on-disk format).
func ExportXLSX(columns []string, source RowSource, w io.Writer) error {
	f := excelize.NewFile()
	defer f.Close()

	const sheet = "Sheet1"
	sw, err := f.NewStreamWriter(sheet)
	if err != nil {
		return err
	}

	headerRow := make([]interface{}, len(columns))
	for i, col := range columns {
		headerRow[i] = col
	}
	if err := sw.SetRow("A1", headerRow); err != nil {
		return err
	}

	rowNum := 2
	for {
		row, err := source()
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "EOF") {
				break
			}
			return err
		}

		vals := make([]interface{}, len(row))
		for i, v := range row {
			if v == nil {
				vals[i] = nil
			} else if b, ok := v.([]byte); ok {
				vals[i] = base64.StdEncoding.EncodeToString(b)
			} else {
				vals[i] = v
			}
		}

		cell, err := excelize.CoordinatesToCellName(1, rowNum)
		if err != nil {
			return err
		}
		if err := sw.SetRow(cell, vals); err != nil {
			return fmt.Errorf("failed to write xlsx row %d: %w", rowNum, err)
		}
		rowNum++
	}

	if err := sw.Flush(); err != nil {
		return err
	}

	_, err = f.WriteTo(w)
	return err
}
