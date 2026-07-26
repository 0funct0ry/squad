package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/server"
	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	CommitSHA = "none"
)

type Config struct {
	commonFlags
	restFlags
	Write          bool
	ReadOnlyPragma bool
	Examples       bool
}

var cfg Config

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "squad <db>",
	Version: Version,
	Short:   "squad is a single-binary web-based SQLite client",
	Long:    `squad opens a SQLite database and starts a web server for browsing, querying, and managing your SQLite databases.`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		applyExamplesEnvOverride(cmd.Flags().Changed("examples"), &cfg.Examples)
		applyRestEnvOverrides(cmd, &cfg.restFlags)

		dbPath := args[0]
		resolvedPath := dbPath
		if dbPath != ":memory:" && !strings.HasPrefix(dbPath, "file:") {
			if abs, err := filepath.Abs(dbPath); err == nil {
				resolvedPath = abs
			}
		}

		// Determine if database should be opened in read-only mode.
		// Read-only mode is active if Write is false.
		// Read-only pragma controls whether we pass mode=ro.
		readOnly := !cfg.Write && cfg.ReadOnlyPragma

		fmt.Printf("squad %s\n", Version)
		modeStr := "read-only"
		if cfg.Write {
			modeStr = "write"
		}
		fmt.Printf("  database : %s  (%s)\n", resolvedPath, modeStr)

		// Open the database
		database, err := db.OpenDB(resolvedPath, readOnly)
		if err != nil {
			fmt.Printf("Error: failed to open database: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		warnIfBroadcastBind("--addr", cfg.Addr)
		if cfg.Rest {
			warnIfBroadcastBind("--rest-bind-addr", cfg.RestBindAddr)
		}

		// Start the server
		srv := server.NewServer(database, resolvedPath, cfg.Write, cfg.Examples, cfg.Rest, cfg.RestBindAddr, cfg.RestPort)
		addr := fmt.Sprintf("%s:%d", cfg.Addr, cfg.Port)
		fmt.Printf("  address  : http://%s\n", addr)
		if cfg.Rest {
			fmt.Printf("  rest     : capability enabled (start it from the REST tab) on %s:%d\n", cfg.RestBindAddr, cfg.RestPort)
		}
		fmt.Println("  press Ctrl+C to stop")

		// If Open is true, open default browser after a short delay
		if cfg.Open {
			go func() {
				time.Sleep(500 * time.Millisecond)
				openBrowser(fmt.Sprintf("http://%s", addr))
			}()
		}

		runServeWithGracefulShutdown(srv, addr, nil)
	},
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	_ = err // Ignore error, don't crash CLI if browser fails to open
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("squad version %s (commit: %s)\n", Version, CommitSHA))

	// Bind flags to Config struct
	registerCommonFlags(rootCmd.Flags(), &cfg.commonFlags)
	registerRestFlags(rootCmd.Flags(), &cfg.restFlags)
	rootCmd.Flags().BoolVarP(&cfg.Write, "write", "w", false, "Enable mutations (DDL, DML, write operations)")
	rootCmd.Flags().BoolVarP(&cfg.ReadOnlyPragma, "read-only-pragma", "R", true, "Open SQLite with mode=ro when not --write")
	rootCmd.Flags().BoolVarP(&cfg.Examples, "examples", "e", false, "Enable the canned example data-model library")
}

// applyExamplesEnvOverride applies SQUAD_EXAMPLES when --examples was not
// explicitly passed on the command line, preserving flags > env > defaults.
func applyExamplesEnvOverride(changed bool, examples *bool) {
	if changed {
		return
	}
	if v := os.Getenv("SQUAD_EXAMPLES"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			*examples = b
		}
	}
}
