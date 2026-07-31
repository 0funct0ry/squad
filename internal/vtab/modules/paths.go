package modules

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// modulesRoot is the confinement root for every file-reading module's
// file=/root= argument, set once by Register (from --modules-root, default:
// the directory holding the open database file).
var modulesRoot string

// SetModulesRoot records the confinement root. Called once by cmd/ before
// registration; a mount error, not a silent empty table, is what a
// traversal/symlink escape produces afterward (see ResolvePath).
func SetModulesRoot(root string) {
	modulesRoot = root
}

// ModulesRoot returns the currently configured confinement root.
func ModulesRoot() string {
	return modulesRoot
}

// ResolvePath resolves a file=/root= argument inside the configured
// --modules-root, rejecting traversal and symlink escape. Returns an
// absolute path safe to open.
func ResolvePath(rel string) (string, error) {
	if modulesRoot == "" {
		return "", errors.New("modules root is not configured")
	}
	root, err := filepath.Abs(modulesRoot)
	if err != nil {
		return "", err
	}
	root, err = evalSymlinksBestEffort(root)
	if err != nil {
		return "", err
	}

	joined := filepath.Join(root, rel)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}

	resolved, err := evalSymlinksBestEffort(abs)
	if err != nil {
		return "", err
	}

	if resolved != root && !strings.HasPrefix(resolved, root+string(filepath.Separator)) {
		return "", errors.New("path escapes --modules-root: " + rel)
	}
	return resolved, nil
}

// evalSymlinksBestEffort resolves symlinks in path, falling back to the
// (already filepath.Abs'd) input when the path doesn't exist yet — a mount
// argument may name a file, not a directory, and non-existence is reported
// by the caller opening it, not here.
func evalSymlinksBestEffort(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, nil
		}
		return "", err
	}
	return resolved, nil
}
