package vtab

import (
	"sync"

	"github.com/0funct0ry/squad/internal/vtab/modules"
	vtabdriver "modernc.org/sqlite/vtab"
)

var (
	enabled     bool
	registerOne sync.Once
)

// Configure records whether --modules was passed and the confinement root
// for file-reading modules. Must be called before the first OpenDB call, and
// before Register.
func Configure(modulesEnabled bool, modulesRoot string) {
	enabled = modulesEnabled
	modules.SetModulesRoot(modulesRoot)
}

// Enabled reports whether --modules was passed for this process.
func Enabled() bool {
	return enabled
}

// ModulesRoot returns the configured file confinement root.
func ModulesRoot() string {
	return modules.ModulesRoot()
}

// Register registers every module in the catalog with the driver, exactly
// once per process. It is a no-op unless Configure(true, ...) was called
// first. Called from internal/db.OpenDB, the one chokepoint every OpenDB
// call site (single-DB, squad cli, sandbox registry) funnels through, so a
// single call here covers every connection the process opens.
func Register() error {
	if !enabled {
		return nil
	}
	var regErr error
	registerOne.Do(func() {
		for _, name := range sortedNames() {
			def := registry[name]
			if err := vtabdriver.RegisterModule(nil, def.Name, def.factory()); err != nil {
				regErr = err
				return
			}
		}
	})
	return regErr
}

func sortedNames() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	// Deterministic order for reproducible registration-failure messages;
	// registration itself doesn't depend on order.
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}
