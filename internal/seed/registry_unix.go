package seed

import (
	"fmt"

	"github.com/0funct0ry/squad/internal/seed/data"
	"github.com/brianvoe/gofakeit/v7"
)

// genUnixFilePath builds a "/<dir1>/<dir2>/<filename>.<ext>" style path from
// the curated directory/filename/extension pools.
func genUnixFilePath() string {
	dir1 := pickFrom(data.UnixDirPool)
	dir2 := pickFrom(data.UnixDirPool)
	name := pickFrom(data.UnixFilenameWords)
	ext := pickFrom(data.UnixFileExtensions)
	return fmt.Sprintf("/%s/%s/%s.%s", dir1, dir2, name, ext)
}

// genUnixPermissionString returns an octal ("0755") or symbolic ("rwxr-xr-x")
// style Unix file permission string.
func genUnixPermissionString(symbolic bool) string {
	octals := []string{"0755", "0644", "0600", "0700", "0666"}
	symbolics := []string{"rwxr-xr-x", "rw-r--r--", "rw-------", "rwx------", "rw-rw-rw-"}
	idx := gofakeit.Number(0, len(octals)-1)
	if symbolic {
		return symbolics[idx]
	}
	return octals[idx]
}

// genUnixEnvironmentVariable builds a "KEY=value" environment variable
// string from the curated name/value pools.
func genUnixEnvironmentVariable() string {
	return pickFrom(data.UnixEnvVarNames) + "=" + pickFrom(data.UnixEnvVarValues)
}

// genUnixCrontabEntry builds a valid 5-field cron expression followed by a
// shell-ish command.
func genUnixCrontabEntry() string {
	minute := gofakeit.Number(0, 59)
	hour := weightedPick([]string{"0", "6", "12", "18", "*"}, []int{20, 20, 20, 20, 20})
	return fmt.Sprintf("%d %s * * * %s", minute, hour, pickFrom(data.UnixCronCommands))
}

// genUnixLogFilePath builds a "/var/log/<slug>.log" path.
func genUnixLogFilePath() string {
	return "/var/log/" + kebabSlug(data.UnixFilenameWords, 1, 1) + ".log"
}

// genUnixMountPoint builds a weighted Unix mount point, appending a slug for
// the "/mnt" and "/media" cases.
func genUnixMountPoint() string {
	prefix := weightedPick(data.UnixMountPointPrefixes, data.UnixMountPointWeights)
	switch prefix {
	case "/mnt", "/media":
		return prefix + "/" + kebabSlug(data.UnixFilenameWords, 1, 1)
	default:
		return prefix
	}
}

// genUnixFileHash returns an md5 (32 hex chars) or sha256 (64 hex chars) file
// hash string, depending on algo.
func genUnixFileHash(algo string) string {
	if algo == "sha256" {
		return hexString(64)
	}
	return hexString(32)
}

func unixGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "unix.filePath", Group: "unix", Description: "Unix filesystem path", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return genUnixFilePath(), nil
		}},

		{Name: "unix.permissionString", Group: "unix", Description: "Unix file permission string (octal or symbolic)", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "symbolic", Label: "Symbolic (rwxr-xr-x)", Kind: OptKindBool, Default: false},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			return genUnixPermissionString(optBool(opts, "symbolic", false)), nil
		}},

		{Name: "unix.groupName", Group: "unix", Description: "Unix group name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return weightedPick(data.UnixGroupNames, data.UnixGroupWeights), nil
		}},

		{Name: "unix.processName", Group: "unix", Description: "Unix daemon/process name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return weightedPick(data.UnixProcessNames, data.UnixProcessWeights), nil
		}},

		{Name: "unix.pid", Group: "unix", Description: "Unix process ID", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Number(1, 99999), nil
		}},

		{Name: "unix.environmentVariable", Group: "unix", Description: "Unix environment variable (KEY=value)", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return genUnixEnvironmentVariable(), nil
		}},

		{Name: "unix.crontabEntry", Group: "unix", Description: "Unix crontab entry", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return genUnixCrontabEntry(), nil
		}},

		{Name: "unix.logFilePath", Group: "unix", Description: "Unix log file path", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return genUnixLogFilePath(), nil
		}},

		{Name: "unix.mountPoint", Group: "unix", Description: "Unix mount point", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return genUnixMountPoint(), nil
		}},

		{Name: "unix.deviceName", Group: "unix", Description: "Unix device node name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return weightedPick(data.UnixDeviceNames, data.UnixDeviceWeights), nil
		}},

		{Name: "unix.kernelModuleName", Group: "unix", Description: "Linux kernel module name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return weightedPick(data.UnixKernelModules, data.UnixKernelModuleWeights), nil
		}},

		{Name: "unix.signalName", Group: "unix", Description: "POSIX signal name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return weightedPick(data.UnixSignalNames, data.UnixSignalWeights), nil
		}},

		{Name: "unix.fileHash", Group: "unix", Description: "File hash (md5 or sha256)", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "algo", Label: "Algorithm", Kind: OptKindSelect, Choices: []string{"md5", "sha256"}, Default: "md5"},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			return genUnixFileHash(optString(opts, "algo", "md5")), nil
		}},
	}
}
