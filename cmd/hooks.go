package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/hooks"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// hookFlags holds the --hook-mode/--allow-net flags shared between root,
// `squad cli`, `squad sandbox`, and `squad hooks`, mirroring moduleFlags.
// --hooks itself (the on/off switch) is deliberately NOT part of this
// struct: root/cli/sandbox register it separately via
// registerHooksEnableFlag, but the standalone `squad hooks` subcommand does
// not register it at all — running `squad hooks ...` is itself explicit
// intent to manage hooks and always works regardless of the flag.
type hookFlags struct {
	HookMode string
	AllowNet bool
}

func registerHookFlags(fs *pflag.FlagSet, h *hookFlags) {
	fs.StringVarP(&h.HookMode, "hook-mode", "H", "sync", "Lua trigger hook execution mode: sync (blocking, before hooks can abort a write) or async (fire-and-forget, after hooks only)")
	fs.BoolVarP(&h.AllowNet, "allow-net", "n", false, "Allow hook scripts to make outbound HTTP requests via the Lua http module")
}

// registerHooksEnableFlag registers --hooks, the on/off switch for the whole
// Lua trigger hooks feature (web Hooks tab, /api/hooks* routes, and squad
// cli's .hooks dot-command). Without it, hooks.RegisterAll/Init are no-ops:
// __squad_invoke_hook is never registered, so even a pre-existing hook
// trigger fails its next write with "no such function" rather than quietly
// keeping working. Registered on root/cli/sandbox only — never on the
// standalone `squad hooks` subcommand itself, which always works.
func registerHooksEnableFlag(fs *pflag.FlagSet, enabled *bool) {
	fs.BoolVarP(enabled, "hooks", "k", false, "Enable Lua trigger hooks: web Hooks tab, /api/hooks* routes, and squad cli's .hooks dot-command (squad hooks always works without this flag)")
}

// init wires internal/db.OpenDB's hook-dispatcher registration to
// hooks.RegisterAll (mirrors cmd/functions.go). modernc.org/sqlite's function
// registry is process-global and must be populated before the first
// sql.Open, so this indirection guarantees every OpenDB call site registers
// __squad_invoke_hook exactly once, before any connection exists.
func init() {
	db.RegisterHooksHook = hooks.RegisterAll
	rootCmd.AddCommand(hooksCmd)
}

var hooksCfg struct {
	hookFlags
	Write          bool
	ReadOnlyPragma bool

	DBPath string
	Table  string
	Event  string
	Timing string
	Scope  string
	Name   string
	Desc   string
	File   string
	Old    string
	New    string
}

var hooksCmd = &cobra.Command{
	Use:   "hooks <db>",
	Short: "Manage Lua trigger hooks without starting the server",
	Long: `squad hooks manages a database's Lua trigger hooks directly against the
database file — no HTTP server and no interactive shell. It honors the same
--write/--hook-mode/--allow-net flags as the root command.`,
}

func init() {
	registerHookFlags(hooksCmd.PersistentFlags(), &hooksCfg.hookFlags)
	hooksCmd.PersistentFlags().BoolVarP(&hooksCfg.Write, "write", "w", false, "Enable mutations (creating/editing/deleting hooks)")
	hooksCmd.PersistentFlags().BoolVarP(&hooksCfg.ReadOnlyPragma, "read-only-pragma", "R", true, "Open SQLite with mode=ro when not --write")

	listCmd := &cobra.Command{
		Use:   "list <db>",
		Short: "List hook definitions",
		Args:  cobra.ExactArgs(1),
		RunE:  runHooksList,
	}
	listCmd.Flags().StringVar(&hooksCfg.Table, "table", "", "Only list hooks for this table")

	createCmd := &cobra.Command{
		Use:   "create <db>",
		Short: "Create a hook (Lua source from --file or stdin)",
		Args:  cobra.ExactArgs(1),
		RunE:  runHooksCreate,
	}
	createCmd.Flags().StringVar(&hooksCfg.Table, "table", "", "Table the hook fires on (required)")
	createCmd.Flags().StringVar(&hooksCfg.Event, "event", "insert", "insert|update|delete")
	createCmd.Flags().StringVar(&hooksCfg.Timing, "timing", "after", "before|after")
	createCmd.Flags().StringVar(&hooksCfg.Scope, "scope", "row", "row|statement")
	createCmd.Flags().StringVar(&hooksCfg.Name, "name", "", "Human-readable hook name")
	createCmd.Flags().StringVar(&hooksCfg.Desc, "description", "", "Hook description")
	createCmd.Flags().StringVar(&hooksCfg.File, "file", "", "Path to the hook's Lua source (default: stdin)")

	editCmd := &cobra.Command{
		Use:   "edit <db> <id>",
		Short: "Replace a hook's Lua source",
		Args:  cobra.ExactArgs(2),
		RunE:  runHooksEdit,
	}
	editCmd.Flags().StringVar(&hooksCfg.File, "file", "", "Path to the hook's new Lua source (default: stdin)")

	testCmd := &cobra.Command{
		Use:   "test <db> <id>",
		Short: "Dry-run a hook against sample row JSON (no trigger fires)",
		Args:  cobra.ExactArgs(2),
		RunE:  runHooksTest,
	}
	testCmd.Flags().StringVar(&hooksCfg.Old, "old", "", `Sample OLD row as JSON, e.g. '{"id":1}'`)
	testCmd.Flags().StringVar(&hooksCfg.New, "new", "", `Sample NEW row as JSON, e.g. '{"email":"a@b.com"}'`)

	hooksCmd.AddCommand(listCmd, createCmd, editCmd, testCmd,
		&cobra.Command{Use: "enable <db> <id>", Short: "Enable a hook", Args: cobra.ExactArgs(2), RunE: runHooksEnable},
		&cobra.Command{Use: "disable <db> <id>", Short: "Disable a hook", Args: cobra.ExactArgs(2), RunE: runHooksDisable},
		&cobra.Command{Use: "rm <db> <id>", Short: "Delete a hook and drop its trigger", Args: cobra.ExactArgs(2), RunE: runHooksRemove},
		&cobra.Command{Use: "log <db> <id>", Short: "Show a hook's execution log", Args: cobra.ExactArgs(2), RunE: runHooksLog},
	)
}

// dbHandle bundles the opened connection so every subcommand can `defer
// h.Close()` uniformly.
type dbHandle struct{ DB *sql.DB }

func (h dbHandle) Close() {
	if h.DB != nil {
		hooks.Drain()
		h.DB.Close()
	}
}

// openForHooks resolves the db path, configures internal/hooks from the
// resolved flags, opens the database and attaches hooks to it — the same
// sequence cmd/root.go performs before starting the server.
func openForHooks(path string) (dbHandle, error) {
	resolved := path
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if abs, err := filepath.Abs(path); err == nil {
			resolved = abs
		}
	}
	readOnly := !hooksCfg.Write && hooksCfg.ReadOnlyPragma
	// enabled is always true here: running `squad hooks ...` is itself
	// explicit intent, independent of --hooks (which this subcommand
	// doesn't even register — see registerHooksEnableFlag's doc comment).
	hooks.Configure(hooksCfg.HookMode, hooksCfg.AllowNet, hooksCfg.Write, true)

	d, err := db.OpenDB(resolved, readOnly)
	if err != nil {
		return dbHandle{}, err
	}
	if err := hooks.Init(d); err != nil {
		d.Close()
		return dbHandle{}, err
	}
	return dbHandle{DB: d}, nil
}

func requireHookWrite() error {
	if !hooksCfg.Write {
		return fmt.Errorf("READ_ONLY: managing hooks requires --write mode")
	}
	return nil
}

func parseHookID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid hook id %q", s)
	}
	return id, nil
}

func runHooksList(cmd *cobra.Command, args []string) error {
	h, err := openForHooks(args[0])
	if err != nil {
		return err
	}
	defer h.Close()
	list, err := hooks.List(h.DB, hooksCfg.Table)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no hooks defined")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%-5s %-20s %-8s %-7s %-10s %-8s %s\n", "ID", "TABLE", "EVENT", "TIMING", "SCOPE", "ENABLED", "NAME")
	for _, hk := range list {
		fmt.Fprintf(cmd.OutOrStdout(), "%-5d %-20s %-8s %-7s %-10s %-8v %s\n", hk.ID, hk.Table, hk.Event, hk.Timing, hk.Scope, hk.Enabled, hk.Name)
	}
	return nil
}

func readHookSource() (string, error) {
	if hooksCfg.File != "" {
		b, err := os.ReadFile(hooksCfg.File)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func runHooksCreate(cmd *cobra.Command, args []string) error {
	if err := requireHookWrite(); err != nil {
		return err
	}
	src, err := readHookSource()
	if err != nil {
		return err
	}
	h, err := openForHooks(args[0])
	if err != nil {
		return err
	}
	defer h.Close()
	created, err := hooks.Create(h.DB, hooks.Hook{
		Table: hooksCfg.Table, Event: hooksCfg.Event, Timing: hooksCfg.Timing,
		Scope: hooksCfg.Scope, Name: hooksCfg.Name, Description: hooksCfg.Desc,
		Source: src, Enabled: true,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "created hook %d on %s (%s %s)\n", created.ID, created.Table, created.Timing, created.Event)
	return nil
}

func runHooksEdit(cmd *cobra.Command, args []string) error {
	if err := requireHookWrite(); err != nil {
		return err
	}
	id, err := parseHookID(args[1])
	if err != nil {
		return err
	}
	src, err := readHookSource()
	if err != nil {
		return err
	}
	h, err := openForHooks(args[0])
	if err != nil {
		return err
	}
	defer h.Close()
	if _, err := hooks.Update(h.DB, id, hooks.Patch{Source: &src}); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "updated hook %d\n", id)
	return nil
}

func setHookEnabled(cmd *cobra.Command, args []string, enabled bool) error {
	if err := requireHookWrite(); err != nil {
		return err
	}
	id, err := parseHookID(args[1])
	if err != nil {
		return err
	}
	h, err := openForHooks(args[0])
	if err != nil {
		return err
	}
	defer h.Close()
	if _, err := hooks.Update(h.DB, id, hooks.Patch{Enabled: &enabled}); err != nil {
		return err
	}
	state := "disabled"
	if enabled {
		state = "enabled"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "hook %d %s\n", id, state)
	return nil
}

func runHooksEnable(cmd *cobra.Command, args []string) error { return setHookEnabled(cmd, args, true) }
func runHooksDisable(cmd *cobra.Command, args []string) error {
	return setHookEnabled(cmd, args, false)
}

func runHooksRemove(cmd *cobra.Command, args []string) error {
	if err := requireHookWrite(); err != nil {
		return err
	}
	id, err := parseHookID(args[1])
	if err != nil {
		return err
	}
	h, err := openForHooks(args[0])
	if err != nil {
		return err
	}
	defer h.Close()
	if err := hooks.Delete(h.DB, id); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "removed hook %d\n", id)
	return nil
}

func runHooksTest(cmd *cobra.Command, args []string) error {
	id, err := parseHookID(args[1])
	if err != nil {
		return err
	}
	oldRow, err := parseRowJSON(hooksCfg.Old)
	if err != nil {
		return fmt.Errorf("--old: %w", err)
	}
	newRow, err := parseRowJSON(hooksCfg.New)
	if err != nil {
		return fmt.Errorf("--new: %w", err)
	}
	h, err := openForHooks(args[0])
	if err != nil {
		return err
	}
	defer h.Close()
	hk, err := hooks.Get(h.DB, id)
	if err != nil {
		return err
	}
	res := hooks.Run(hk, oldRow, newRow, hooks.RunConfig{
		DB: h.DB, Write: hooksCfg.Write, AllowNet: hooksCfg.AllowNet, Record: true,
	})
	hooks.Drain()
	printHookResult(cmd.OutOrStdout(), res)
	return nil
}

func printHookResult(w io.Writer, res hooks.Result) {
	result := "nil"
	if res.Result != nil {
		result = strconv.FormatBool(*res.Result)
	}
	fmt.Fprintf(w, "result:   %s\n", result)
	fmt.Fprintf(w, "message:  %s\n", res.Message)
	fmt.Fprintf(w, "duration: %dms\n", res.DurationMs)
	if res.Error != "" {
		fmt.Fprintf(w, "error:    %s\n", res.Error)
	}
	for _, l := range res.Logs {
		fmt.Fprintf(w, "log:      %s\n", l)
	}
}

func runHooksLog(cmd *cobra.Command, args []string) error {
	id, err := parseHookID(args[1])
	if err != nil {
		return err
	}
	h, err := openForHooks(args[0])
	if err != nil {
		return err
	}
	defer h.Close()
	runs, err := hooks.Logs(h.DB, id, 50, 0)
	if err != nil {
		return err
	}
	for _, r := range runs {
		fmt.Fprintf(cmd.OutOrStdout(), "%s  %-14s success=%v  %dms  %s\n", r.RanAt, r.Event, r.Success, r.DurationMs, r.Error)
	}
	return nil
}

func parseRowJSON(s string) (map[string]any, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, err
	}
	return m, nil
}
