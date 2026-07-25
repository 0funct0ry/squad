package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	Addr           string
	Port           int
	Write          bool
	Rest           bool
	Open           bool
	ReadOnlyPragma bool
	Token          string
	LogLevel       string
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

		// Start the server
		srv := server.NewServer(database, resolvedPath, cfg.Write)
		addr := fmt.Sprintf("%s:%d", cfg.Addr, cfg.Port)
		fmt.Printf("  address  : http://%s\n", addr)
		fmt.Println("  press Ctrl+C to stop")

		// If Open is true, open default browser after a short delay
		if cfg.Open {
			go func() {
				time.Sleep(500 * time.Millisecond)
				openBrowser(fmt.Sprintf("http://%s", addr))
			}()
		}

		if err := srv.Start(addr); err != nil {
			fmt.Printf("Error starting server: %v\n", err)
			os.Exit(1)
		}
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
	rootCmd.Flags().StringVarP(&cfg.Addr, "addr", "a", "127.0.0.1", "Bind address")
	rootCmd.Flags().IntVarP(&cfg.Port, "port", "p", 7071, "Port to listen on")
	rootCmd.Flags().BoolVarP(&cfg.Write, "write", "w", false, "Enable mutations (DDL, DML, write operations)")
	rootCmd.Flags().BoolVarP(&cfg.Rest, "rest", "r", false, "Enable auto REST endpoints for tables")
	rootCmd.Flags().BoolVarP(&cfg.Open, "open", "o", true, "Auto-open default browser on start")
	rootCmd.Flags().BoolVarP(&cfg.ReadOnlyPragma, "read-only-pragma", "R", true, "Open SQLite with mode=ro when not --write")
	rootCmd.Flags().StringVarP(&cfg.Token, "token", "t", "", "Optional bearer token gate for the API")
	rootCmd.Flags().StringVarP(&cfg.LogLevel, "log-level", "l", "info", "Log level (debug/info/warn/error)")
}
