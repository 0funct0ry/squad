package cmd

import (
	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/udf"
)

// init wires internal/db.OpenDB's UDF registration hook to udf.RegisterAll.
// Unlike vtab modules, the curated UDF library is always-on (M10b) — no
// --flag gates it — so this always runs, once per process (udf.RegisterAll
// is itself sync.Once-guarded).
func init() {
	db.RegisterUDFHook = udf.RegisterAll
}
