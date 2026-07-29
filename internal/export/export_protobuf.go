package export

import (
	"encoding/base64"
	"io"
	"strings"

	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/types/known/structpb"
)

// ExportProtobuf streams rows as a sequence of length-delimited
// google.protobuf.Struct messages (one per row) - a generic, schema-less
// representation rather than a hand-authored or runtime-built typed
// message per table. A consumer decodes with protodelim.UnmarshalFrom into
// a structpb.Struct and reads it back with Struct.AsMap().
func ExportProtobuf(columns []string, source RowSource, w io.Writer) error {
	for {
		row, err := source()
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "EOF") {
				break
			}
			return err
		}

		fields := make(map[string]any, len(columns))
		for i, col := range columns {
			fields[col] = toStructValue(row[i])
		}

		st, err := structpb.NewStruct(fields)
		if err != nil {
			return err
		}
		if _, err := protodelim.MarshalTo(w, st); err != nil {
			return err
		}
	}
	return nil
}

// toStructValue coerces a scanned SQL value into one structpb.NewStruct
// accepts (nil/bool/float64/string/[]any/map[string]any) - notably
// widening integers to float64 and base64-encoding blobs, since
// google.protobuf.Value has no dedicated integer or bytes type.
func toStructValue(v any) any {
	switch val := v.(type) {
	case []byte:
		return base64.StdEncoding.EncodeToString(val)
	case int:
		return float64(val)
	case int32:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return val
	}
}
