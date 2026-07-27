package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/server"
	"github.com/spf13/cobra"
)

// SandboxConfig holds flags specific to `squad sandbox`. --write and
// --read-only-pragma intentionally have no equivalent here: sandbox
// databases are always opened read-write and there is no READ_ONLY code
// path for them. --rest/--rest-port/--rest-bind-addr are supported, same as
// root mode, except /rest/* always resolves against whichever sandbox
// database is currently active (SPEC §5.7).
type SandboxConfig struct {
	commonFlags
	restFlags
	Dir         string
	MaxUploadMB int64
	Examples    bool
}

var sandboxCfg SandboxConfig

var sandboxCmd = &cobra.Command{
	Use:   "sandbox",
	Short: "Start squad with an ad-hoc, browser-managed set of SQLite databases",
	Long: `squad sandbox starts the same web UI with no fixed database. Upload,
create, and switch between one or more SQLite files entirely from the
browser instead of passing a <db> path on the command line. Every database
opened in sandbox mode is always read-write.`,
	Args: cobra.NoArgs,
	Run:  runSandbox,
}

func init() {
	registerCommonFlags(sandboxCmd.Flags(), &sandboxCfg.commonFlags)
	registerRestFlags(sandboxCmd.Flags(), &sandboxCfg.restFlags)
	sandboxCmd.Flags().StringVar(&sandboxCfg.Dir, "dir", "", "Directory to store sandbox database files (env SQUAD_SANDBOX_DIR); defaults to a fresh temp dir")
	sandboxCmd.Flags().Int64Var(&sandboxCfg.MaxUploadMB, "max-upload-size", 512, "Max upload size in MB for sandbox database files")
	sandboxCmd.Flags().BoolVarP(&sandboxCfg.Examples, "examples", "e", false, "Enable the canned example data-model library")
	rootCmd.AddCommand(sandboxCmd)
}

func runSandbox(cmd *cobra.Command, args []string) {
	applyExamplesEnvOverride(cmd.Flags().Changed("examples"), &sandboxCfg.Examples)
	applyRestEnvOverrides(cmd, &sandboxCfg.restFlags)

	dir := sandboxCfg.Dir
	dirExplicitlySet := cmd.Flags().Changed("dir")
	if dir == "" {
		if envDir := os.Getenv("SQUAD_SANDBOX_DIR"); envDir != "" {
			dir = envDir
			dirExplicitlySet = true
		}
	}

	var cleanupTemp bool
	if dir == "" {
		tmp, err := os.MkdirTemp(os.TempDir(), "squad-sandbox-")
		if err != nil {
			fmt.Printf("Error: failed to create sandbox temp dir: %v\n", err)
			os.Exit(1)
		}
		dir = tmp
		cleanupTemp = true
	} else {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Printf("Error: sandbox dir %q could not be created: %v\n", dir, err)
			os.Exit(1)
		}
		probe := filepath.Join(dir, ".squad-write-check")
		f, err := os.Create(probe)
		if err != nil {
			fmt.Printf("Error: sandbox dir %q is not writable: %v\n", dir, err)
			os.Exit(1)
		}
		f.Close()
		os.Remove(probe)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}

	registry := db.NewRegistry(absDir, sandboxCfg.MaxUploadMB*1024*1024)
	if dirExplicitlySet {
		for _, rescanErr := range registry.Rescan() {
			fmt.Printf("  warning: skipping invalid file during sandbox dir scan: %v\n", rescanErr)
		}
	}

	warnIfBroadcastBind("--addr", sandboxCfg.Addr)
	if sandboxCfg.Rest {
		warnIfBroadcastBind("--rest-bind-addr", sandboxCfg.RestBindAddr)
	}

	fmt.Printf("squad %s (sandbox mode)\n", Version)
	fmt.Printf("  sandbox dir : %s\n", absDir)

	srv := server.NewSandboxServer(registry, sandboxCfg.Examples, sandboxCfg.Rest, sandboxCfg.RestBindAddr, sandboxCfg.RestPort, sandboxCfg.LogLevel)
	addr := fmt.Sprintf("%s:%d", sandboxCfg.Addr, sandboxCfg.Port)
	fmt.Printf("  address     : http://%s\n", addr)
	if sandboxCfg.Rest {
		fmt.Printf("  rest        : capability enabled (start it from the REST tab) on %s:%d\n", sandboxCfg.RestBindAddr, sandboxCfg.RestPort)
	}
	fmt.Println("  press Ctrl+C to stop")

	if sandboxCfg.Open {
		go func() {
			time.Sleep(500 * time.Millisecond)
			openBrowser(fmt.Sprintf("http://%s", addr))
		}()
	}

	runServeWithGracefulShutdown(srv, addr, func() {
		registry.CloseAll()
		if cleanupTemp {
			_ = os.RemoveAll(registry.Dir())
		}
	})
}
