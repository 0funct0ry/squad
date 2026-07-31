package cmd

import (
	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/vtab"
	"github.com/spf13/pflag"
)

// moduleFlags holds the --modules/--modules-root flags shared between root,
// `squad cli`, and `squad sandbox`.
type moduleFlags struct {
	Modules     bool
	ModulesRoot string
}

func registerModuleFlags(fs *pflag.FlagSet, m *moduleFlags) {
	fs.BoolVar(&m.Modules, "modules", false, "Enable virtual table modules (see internal-docs/VTABS.md)")
	fs.StringVar(&m.ModulesRoot, "modules-root", "", "Confinement root for file-reading modules (default: the open database's directory)")
}

// init wires internal/db.OpenDB's registration hook to vtab.Register.
// internal/vtab depends on internal/seed, which depends on internal/db, so
// the hook indirection (rather than internal/db importing internal/vtab
// directly) is what avoids an import cycle while still guaranteeing every
// OpenDB call site registers modules exactly once before its first
// connection opens.
func init() {
	db.RegisterModulesHook = vtab.Register
}
