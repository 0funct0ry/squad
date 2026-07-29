package export

import (
	"io"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ExportBSON streams rows as a concatenation of top-level BSON documents
// (one per row) rather than a single BSON array document - the same
// "sequence of framed records" convention MongoDB tooling (e.g. bsondump)
// expects for multi-document BSON files, and what lets this stay a
// row-at-a-time stream like the other writers instead of buffering
// everything to build one array value.
func ExportBSON(columns []string, source RowSource, w io.Writer) error {
	for {
		row, err := source()
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "EOF") {
				break
			}
			return err
		}

		doc := bson.D{}
		for i, col := range columns {
			doc = append(doc, bson.E{Key: col, Value: row[i]})
		}

		data, err := bson.Marshal(doc)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	return nil
}
