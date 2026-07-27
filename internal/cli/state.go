// Package cli implements `squad cli <db>`, an interactive terminal REPL that
// behaves like the stock sqlite3 shell. It reuses internal/db's statement
// classifier/splitter/schema introspection and internal/seed's generator
// registry rather than reimplementing any of them.
package cli

import (
	"database/sql"
	"io"
	"os"

	"github.com/0funct0ry/squad/internal/restserver"
)

// OutputMode names one of the .mode render formats.
type OutputMode string

const (
	ModeAscii    OutputMode = "ascii"
	ModeBox      OutputMode = "box"
	ModeColumn   OutputMode = "column"
	ModeCSV      OutputMode = "csv"
	ModeJSON     OutputMode = "json"
	ModeList     OutputMode = "list"
	ModeMarkdown OutputMode = "markdown"
	ModeTable    OutputMode = "table"
	ModeTabs     OutputMode = "tabs"
)

// ValidModes lists every supported .mode value, in the order presented by
// .help and used for .mode completion.
var ValidModes = []OutputMode{ModeAscii, ModeBox, ModeColumn, ModeCSV, ModeJSON, ModeList, ModeMarkdown, ModeTable, ModeTabs}

func IsValidMode(m string) bool {
	for _, v := range ValidModes {
		if string(v) == m {
			return true
		}
	}
	return false
}

// State holds all mutable REPL/session state: the DB handle, --write gate,
// current .mode/.headers/.nullvalue/.output settings, and the completion
// cache. It is shared by the REPL loop, the non-interactive paths, and the
// dot-command dispatcher (all funnel through Execute in executor.go).
type State struct {
	DB       *sql.DB
	Path     string
	Write    bool
	ReadOnly bool // the resolved readOnly bool cmd/cli.go computed at startup; reused verbatim by .open

	Interactive bool
	Colorized   bool // stdout is a TTY; suppress color when piped

	Mode      OutputMode
	Headers   bool
	NullValue string

	// Prompt/ContinuationPrompt are templates (see prompt.go's RenderPrompt):
	// {db} expands to the database's display name, and color tags like
	// {green}/{bold}/{reset} expand to ANSI escapes (or are stripped when not
	// Colorized). Settable via ".prompt"/".prompt continuation".
	Prompt             string
	ContinuationPrompt string

	// Out is where query results/dot-command output are written. Defaults to
	// os.Stdout; redirected by .output/.once.
	Out            io.Writer
	onceFile       *os.File // closed after the next statement if set via .once
	outputFilePath string   // path passed to the active persistent .output redirect; "" = stdout, set/cleared by cmdOutput

	// schema completion cache, invalidated after DDL (see executor.go)
	tablesCache  []string
	columnsCache map[string][]string
	cacheValid   bool

	// History records each top-level line/statement entered in the
	// interactive REPL, in order, 1-indexed by position for .history and
	// .history N (see repl.go/dotcommands.go). Not populated by the
	// non-interactive inline-SQL/stdin/.read paths.
	History []string

	// pendingDefault is set by ".edit" with the edited text; RunREPL consumes
	// it once, prefilling the next Readline() call via rl.SetDefault, then
	// clears it. Only meaningful in the interactive REPL.
	pendingDefault string

	// LastColumns/LastRows retain the most recent SELECT-shaped result set
	// for ".grep" to filter/re-render without re-querying the DB. A write
	// statement deliberately does NOT clear these (see executor.go).
	LastColumns []string
	LastRows    [][]any

	// REST control (".rest"/".listener"/".token") -- lazily constructed on
	// first use; RestPort/RestBindAddr come from cmd/cli.go's --rest-port/
	// --rest-bind-addr flags (shared with `squad serve`/`squad sandbox`).
	RestManager  *restserver.Manager
	RestConfigs  *restserver.ConfigStore
	RestPort     int
	RestBindAddr string
	Token        string

	// Session ergonomics toggles.
	TimerOn bool
	StatsOn bool

	// Bookmarks is lazily loaded from ~/.squad_bookmarks on first
	// .bookmark/.bookmarks use (see bookmarks.go).
	Bookmarks map[string]bookmarkProfile

	Quit bool
}

func NewState(database *sql.DB, path string, write, interactive, readOnly bool, restPort int, restBindAddr string) *State {
	mode := ModeList
	headers := false
	if interactive {
		mode = ModeColumn
		headers = true
	}
	return &State{
		DB:                 database,
		Path:               path,
		Write:              write,
		ReadOnly:           readOnly,
		Interactive:        interactive,
		Mode:               mode,
		Headers:            headers,
		NullValue:          "",
		Prompt:             DefaultPrompt,
		ContinuationPrompt: DefaultContinuationPrompt,
		Out:                os.Stdout,
		RestPort:           restPort,
		RestBindAddr:       restBindAddr,
	}
}

// closeOnce closes and clears any .once redirect, restoring Out to stdout.
func (s *State) closeOnceIfSet() {
	if s.onceFile != nil {
		s.onceFile.Close()
		s.onceFile = nil
		s.Out = os.Stdout
	}
}

func (s *State) invalidateSchemaCache() {
	s.cacheValid = false
	s.tablesCache = nil
	s.columnsCache = nil
}
