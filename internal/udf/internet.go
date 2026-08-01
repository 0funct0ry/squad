package udf

import (
	"database/sql/driver"
	"encoding/json"
	"mime"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"modernc.org/sqlite"
	"mvdan.cc/xurls/v2"
)

const catInternet = "Internet-related"

var urlRe = xurls.Strict()

func registerInternet() error {
	if err := sqlite.RegisterDeterministicScalarFunction("MIME_TYPE_FROM_EXT", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			ext := filepath.Ext(argString(args[0]))
			t := mime.TypeByExtension(ext)
			if t == "" {
				return nil, nil
			}
			if i := strings.Index(t, ";"); i >= 0 {
				t = strings.TrimSpace(t[:i])
			}
			return t, nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "MIME_TYPE_FROM_EXT", Category: catInternet, Signature: "MIME_TYPE_FROM_EXT(filename) -> str",
		Description: "Guesses a MIME type from a filename's extension.", ExampleCall: `MIME_TYPE_FROM_EXT('report.pdf')`,
		ExampleResult: "application/pdf", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("MIME_TYPE_FROM_BLOB", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			m := mimetype.Detect(argBytes(args[0]))
			return m.String(), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "MIME_TYPE_FROM_BLOB", Category: catInternet, Signature: "MIME_TYPE_FROM_BLOB(blob) -> str",
		Description: "Sniffs the actual content type of blob.", ExampleCall: "SELECT MIME_TYPE_FROM_BLOB(file_data) FROM uploads",
		ExampleResult: "(a MIME type)", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("EXTRACT_URLS", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			urls := urlRe.FindAllString(argString(args[0]), -1)
			if urls == nil {
				urls = []string{}
			}
			out, err := json.Marshal(urls)
			if err != nil {
				return nil, err
			}
			return string(out), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "EXTRACT_URLS", Category: catInternet, Signature: "EXTRACT_URLS(text) -> json",
		Description:   "Extracts every URL found in text as a JSON array.",
		ExampleCall:   `EXTRACT_URLS('see https://squad.dev and http://x.io')`,
		ExampleResult: `["https://squad.dev","http://x.io"]`, Deterministic: true})

	return nil
}
