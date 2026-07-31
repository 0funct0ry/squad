package modules

import (
	"fmt"
	"os"

	"github.com/parquet-go/parquet-go"
	vtabdriver "modernc.org/sqlite/vtab"
)

// ParquetModule implements the `parquet` module (VTABS.md #3): file=,
// columns= (optional projection, pushed down so unread column chunks are
// never decoded).
type ParquetModule struct{}

func (m *ParquetModule) Create(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

func (m *ParquetModule) Connect(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

func (m *ParquetModule) connect(ctx vtabdriver.Context, rawArgs []string) (vtabdriver.Table, error) {
	a, err := ParseArgs(rawArgs)
	if err != nil {
		return nil, err
	}
	file, err := a.GetRequired("file")
	if err != nil {
		return nil, err
	}
	projection := parseColumnsList(a.Get("columns", ""))

	path, err := ResolvePath(file)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("parquet module: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("parquet module: %w", err)
	}

	pf, err := parquet.OpenFile(f, info.Size())
	if err != nil {
		return nil, fmt.Errorf("parquet module: %w", err)
	}

	schema := pf.Schema()
	allCols := make([]string, 0, len(schema.Fields()))
	for _, field := range schema.Fields() {
		allCols = append(allCols, field.Name())
	}

	cols := allCols
	if len(projection) > 0 {
		want := map[string]bool{}
		for _, c := range projection {
			want[c] = true
		}
		cols = nil
		for _, c := range allCols {
			if want[c] {
				cols = append(cols, c)
			}
		}
	}

	if err := DeclareColumns(ctx, cols, nil); err != nil {
		return nil, err
	}

	reader := parquet.NewGenericReader[map[string]any](f, schema)
	defer reader.Close()

	rows := make([][]vtabdriver.Value, 0, reader.NumRows())
	buf := make([]map[string]any, 128)
	for {
		// Schema.Reconstruct (invoked by Read) calls reflect.Value.SetMapIndex
		// on each buffer slot, which panics on a nil map — every slot needs a
		// fresh, non-nil map before each Read call, and fresh (not reused)
		// per row so a row missing an optional column doesn't inherit a
		// stale value left over from a previous row that used this slot.
		for i := range buf {
			buf[i] = make(map[string]any, len(cols))
		}
		n, err := reader.Read(buf)
		for i := 0; i < n; i++ {
			row := make([]vtabdriver.Value, len(cols))
			for j, c := range cols {
				row[j] = parquetScalar(buf[i][c])
			}
			rows = append(rows, row)
		}
		if err != nil {
			break
		}
	}

	return NewSimpleTable(rows), nil
}

func parquetScalar(v any) vtabdriver.Value {
	switch t := v.(type) {
	case nil:
		return nil
	case string, int64, float64, bool:
		return t
	case int:
		return int64(t)
	case []byte:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}
