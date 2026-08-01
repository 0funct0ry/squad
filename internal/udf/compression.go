package udf

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"

	"modernc.org/sqlite"
)

const catCompression = "Compression"

func gzipCompress(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(b); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gzipDecompress(b []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func registerCompression() error {
	if err := sqlite.RegisterDeterministicScalarFunction("GZIP_COMPRESS", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return gzipCompress(argBytes(args[0]))
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "GZIP_COMPRESS", Category: catCompression, Signature: "GZIP_COMPRESS(blob) -> blob",
		Description: "Gzip-compresses blob.", ExampleCall: "SELECT LENGTH(GZIP_COMPRESS(notes)) FROM logs",
		ExampleResult: "(compressed byte length)", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("GZIP_DECOMPRESS", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			out, err := gzipDecompress(argBytes(args[0]))
			if err != nil {
				return nil, fmt.Errorf("GZIP_DECOMPRESS: %w", err)
			}
			return out, nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "GZIP_DECOMPRESS", Category: catCompression, Signature: "GZIP_DECOMPRESS(blob) -> blob",
		Description: "Gzip-decompresses blob.", ExampleCall: "SELECT GZIP_DECOMPRESS(compressed_notes) FROM logs_archive",
		ExampleResult: "(original bytes)", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("COMPRESSION_RATIO", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			orig, err := argFloat(args[0])
			if err != nil {
				return nil, err
			}
			compressed, err := argFloat(args[1])
			if err != nil {
				return nil, err
			}
			if compressed == 0 {
				return nil, nil
			}
			return orig / compressed, nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "COMPRESSION_RATIO", Category: catCompression, Signature: "COMPRESSION_RATIO(blob_orig, blob_compressed) -> float",
		Description:   "Arithmetic ratio of two lengths.",
		ExampleCall:   "SELECT COMPRESSION_RATIO(LENGTH(notes), LENGTH(GZIP_COMPRESS(notes))) FROM logs",
		ExampleResult: "(a ratio >= 1)", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("ZIP_LIST_ENTRIES", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			b := argBytes(args[0])
			r, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
			if err != nil {
				return nil, fmt.Errorf("ZIP_LIST_ENTRIES: %w", err)
			}
			names := make([]string, 0, len(r.File))
			for _, f := range r.File {
				names = append(names, f.Name)
			}
			out, err := json.Marshal(names)
			if err != nil {
				return nil, err
			}
			return string(out), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "ZIP_LIST_ENTRIES", Category: catCompression, Signature: "ZIP_LIST_ENTRIES(blob) -> json",
		Description: "Lists files in a zip blob as a JSON array.",
		ExampleCall: "SELECT ZIP_LIST_ENTRIES(archive_blob) FROM uploads", ExampleResult: `["a.txt","b.csv"]`,
		Deterministic: true})

	return nil
}
