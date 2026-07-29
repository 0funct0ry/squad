package export

import "io"

// newRowSource builds a RowSource that yields the given static rows in
// order, then io.EOF - shared by the new-writer round-trip tests.
func newRowSource(rows [][]any) RowSource {
	idx := 0
	return func() ([]any, error) {
		if idx >= len(rows) {
			return nil, io.EOF
		}
		r := rows[idx]
		idx++
		return r, nil
	}
}
