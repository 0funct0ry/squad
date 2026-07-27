package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0funct0ry/squad/internal/cli"
	"github.com/0funct0ry/squad/internal/db"
	"github.com/spf13/cobra"
)

type cliConfig struct {
	Write          bool
	ReadOnlyPragma bool
	LogLevel       string
	restFlags
}

var cliCfg cliConfig

// cliCmd is `squad cli <db> [SQL]`: an interactive terminal REPL (or
// non-interactive inline-SQL/stdin-script runner) that behaves like the
// stock sqlite3 shell. It reuses db.OpenDB's same DB-open path and safety
// model as the root command, but starts no HTTP server and only honors
// --write, --read-only-pragma, --log-level -- server-only flags
// (--addr/--port/--rest/--open/--token) are intentionally not registered.
var cliCmd = &cobra.Command{
	Use:   "cli <db> [SQL]",
	Short: "Open a SQLite database in an interactive terminal shell",
	Long:  `squad cli opens a SQLite database and starts a readline-based REPL that behaves like the stock sqlite3 shell. No HTTP server or web UI is started.`,
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		dbPath := args[0]
		inlineSQL := ""
		if len(args) == 2 {
			inlineSQL = args[1]
		}

		resolvedPath := dbPath
		if dbPath != ":memory:" && !strings.HasPrefix(dbPath, "file:") {
			if abs, err := filepath.Abs(dbPath); err == nil {
				resolvedPath = abs
			}
		}

		readOnly := !cliCfg.Write && cliCfg.ReadOnlyPragma

		database, err := db.OpenDB(resolvedPath, readOnly)
		if err != nil {
			fmt.Printf("Error: failed to open database: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		interactive := inlineSQL == "" && cli.IsStdinTerminal()
		state := cli.NewState(database, resolvedPath, cliCfg.Write, interactive, readOnly, cliCfg.RestPort, cliCfg.RestBindAddr)

		if err := cli.Run(state, inlineSQL); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(cliCmd)
	cliCmd.Flags().BoolVarP(&cliCfg.Write, "write", "w", false, "Enable mutations (DDL, DML, write operations)")
	cliCmd.Flags().BoolVarP(&cliCfg.ReadOnlyPragma, "read-only-pragma", "R", true, "Open SQLite with mode=ro when not --write")
	cliCmd.Flags().StringVarP(&cliCfg.LogLevel, "log-level", "l", "info", "Log level (debug/info/warn/error)")
	registerRestFlags(cliCmd.Flags(), &cliCfg.restFlags)
}
