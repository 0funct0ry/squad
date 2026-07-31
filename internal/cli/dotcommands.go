package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
)

// dotCommandNames lists every supported dot-command, in .help order and used
// for top-level dot-command completion.
var dotCommandNames = []string{
	".help", ".tables", ".schema", ".indexes", ".databases", ".mode",
	".headers", ".nullvalue", ".output", ".once", ".import", ".dump",
	".read", ".templates", ".history", ".echo", ".prompt", ".quit", ".exit",
	".edit", ".save", ".grep", ".rest", ".listener", ".token", ".timer",
	".stats", ".explain", ".plan", ".bookmark", ".bookmarks", ".shell", ".sh",
	".watch", ".open", ".backup", ".clone", ".seed", ".diff", ".constraints",
	".size", ".stat", ".repeat", ".modules", ".mounts", ".mount", ".unmount",
}

const helpText = `.help                     show this message
.tables ?PATTERN?         list tables matching PATTERN
.schema ?PATTERN?         show CREATE statements for matching tables
.schema -t ?PATTERN?      show matching tables' columns as a table
.indexes ?TABLE?          list indexes, optionally for one table
.databases                list attached databases
.mode MODE                set output mode: ascii box column csv json list markdown table tabs
.headers on|off           turn column headers on or off
.nullvalue STRING         set the text rendered for NULL values
.output ?FILE?            redirect output to FILE, or back to stdout
.once FILE                redirect only the next statement's output to FILE
.import FILE TABLE        import CSV data from FILE into TABLE (--write)
.dump ?PATTERN?           dump matching tables as SQL
.read FILE                execute SQL statements from FILE
.templates                list functions callable inside {{ }} template blocks
.history                  list this session's history
.history N                re-execute history entry N
.echo TEXT                expand {{ }} template functions in TEXT and print the result, without executing it as SQL
.prompt                   show the current prompt/continuation-prompt templates
.prompt TEXT              set the main prompt; {db} expands to the db name, {red}/{green}/.../{bold}/{dim}/{reset} add color
.prompt continuation TEXT set the continuation prompt (same {db}/color tags)
.quit / .exit             exit the shell
.edit -h N                open $EDITOR on history entry N (interactive only)
.edit -c                  open $EDITOR seeded from the OS clipboard (interactive only)
.save FILE "QUERY"        execute QUERY and write its rendered output to FILE
.grep ?-r|--regex? PATTERN  filter the last result set's rows by PATTERN
.rest ?--r|--rw|--rwd? TABLE   configure TABLE's REST exposure (--rw/--rwd need --write)
.listener start|stop     start/stop the REST listener configured via .rest
.token ?VALUE?            get/set a stored bearer token (not yet enforced)
.timer on|off             print statement run time after each statement
.stats on|off             print row/duration/schema-change stats after each statement
.explain QUERY / .plan QUERY  show QUERY's EXPLAIN QUERY PLAN as an indented tree
.bookmark ?save|load? ?NAME?  save/restore mode/headers/nullvalue/prompt/output as NAME
.bookmarks                list saved bookmark names
.shell CMD / .sh CMD      run CMD via $SHELL -c, inheriting stdio
.watch SECONDS QUERY      re-run QUERY every SECONDS until Ctrl-C
.open DB                  close the current db and reopen DB
.backup FILE              VACUUM INTO FILE (errors if FILE exists)
.clone TABLE NEW ?--data? recreate TABLE's DDL as NEW (--write); ?--data? copies rows too
.seed TABLE N             insert N generated rows into TABLE (--write)
.diff TABLE_A TABLE_B     compare two tables' columns
.constraints TABLE        show PK/FK/NOT NULL/UNIQUE/CHECK constraints
.size / .stat db          show database file/meta info
.repeat N "QUERY"         run QUERY N times, re-expanding {{ }} templates fresh each time
.modules ?NAME?           list virtual table modules, or one module's flags/columns (--modules)
.mounts                   list active virtual table mounts
.mount MODULE ALIAS FLAGS...  mount MODULE under ALIAS, e.g. .mount csv x --file data.csv (--modules)
.unmount ALIAS            drop an active mount
`

// dispatchDotCommand parses and executes a dot-command line. ".echo" and
// ".prompt" are special-cased ahead of the generic whitespace-tokenized
// dispatch below because their argument is arbitrary text (spaces, braces,
// color-tag syntax all significant) rather than a small fixed set of
// flag/name tokens like every other dot-command takes.
func (s *State) dispatchDotCommand(line string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == ".echo" || strings.HasPrefix(trimmed, ".echo ") {
		s.cmdEcho(strings.TrimSpace(strings.TrimPrefix(trimmed, ".echo")))
		return
	}
	if trimmed == ".prompt" || strings.HasPrefix(trimmed, ".prompt ") {
		s.cmdPrompt(strings.TrimSpace(strings.TrimPrefix(trimmed, ".prompt")))
		return
	}
	if trimmed == ".save" || strings.HasPrefix(trimmed, ".save ") {
		s.cmdSave(strings.TrimSpace(strings.TrimPrefix(trimmed, ".save")))
		return
	}
	if trimmed == ".explain" || strings.HasPrefix(trimmed, ".explain ") {
		s.cmdExplain(strings.TrimSpace(strings.TrimPrefix(trimmed, ".explain")))
		return
	}
	if trimmed == ".plan" || strings.HasPrefix(trimmed, ".plan ") {
		s.cmdExplain(strings.TrimSpace(strings.TrimPrefix(trimmed, ".plan")))
		return
	}
	if trimmed == ".shell" || strings.HasPrefix(trimmed, ".shell ") {
		s.cmdShell(strings.TrimSpace(strings.TrimPrefix(trimmed, ".shell")))
		return
	}
	if trimmed == ".sh" || strings.HasPrefix(trimmed, ".sh ") {
		s.cmdShell(strings.TrimSpace(strings.TrimPrefix(trimmed, ".sh")))
		return
	}
	if trimmed == ".watch" || strings.HasPrefix(trimmed, ".watch ") {
		s.cmdWatch(strings.TrimSpace(strings.TrimPrefix(trimmed, ".watch")))
		return
	}
	if trimmed == ".repeat" || strings.HasPrefix(trimmed, ".repeat ") {
		s.cmdRepeat(strings.TrimSpace(strings.TrimPrefix(trimmed, ".repeat")))
		return
	}
	if trimmed == ".mount" || strings.HasPrefix(trimmed, ".mount ") {
		s.cmdMount(strings.TrimSpace(strings.TrimPrefix(trimmed, ".mount")))
		return
	}

	fields := splitDotCommand(line)
	if len(fields) == 0 {
		return
	}
	cmd := fields[0]
	args := fields[1:]

	switch cmd {
	case ".help":
		fmt.Fprint(s.Out, helpText)
	case ".quit", ".exit":
		s.Quit = true
	case ".tables":
		s.cmdTables(args)
	case ".schema":
		s.cmdSchema(args)
	case ".indexes":
		s.cmdIndexes(args)
	case ".databases":
		s.cmdDatabases()
	case ".mode":
		s.cmdMode(args)
	case ".headers":
		s.cmdHeaders(args)
	case ".nullvalue":
		if len(args) != 1 {
			s.shellError(fmt.Errorf("usage: .nullvalue STRING"))
			return
		}
		s.NullValue = args[0]
	case ".output":
		s.cmdOutput(args, false)
	case ".once":
		s.cmdOutput(args, true)
	case ".import":
		s.cmdImport(args)
	case ".dump":
		s.cmdDump(args)
	case ".read":
		s.cmdRead(args)
	case ".templates":
		s.cmdTemplates(args)
	case ".history":
		s.cmdHistory(args)
	case ".edit":
		s.cmdEdit(args)
	case ".grep":
		s.cmdGrep(args)
	case ".rest":
		s.cmdRest(args)
	case ".listener":
		s.cmdListener(args)
	case ".token":
		s.cmdToken(args)
	case ".timer":
		s.cmdTimer(args)
	case ".stats":
		s.cmdStats(args)
	case ".bookmark":
		s.cmdBookmark(args)
	case ".bookmarks":
		s.cmdBookmarks()
	case ".open":
		s.cmdOpen(args)
	case ".backup":
		s.cmdBackup(args)
	case ".clone":
		s.cmdClone(args)
	case ".seed":
		s.cmdSeed(args)
	case ".diff":
		s.cmdDiff(args)
	case ".constraints":
		s.cmdConstraints(args)
	case ".size":
		s.cmdSize(args)
	case ".stat":
		if len(args) != 1 || args[0] != "db" {
			s.shellError(fmt.Errorf("usage: .stat db"))
			return
		}
		s.cmdSize(nil)
	case ".modules":
		s.cmdModules(args)
	case ".mounts":
		s.cmdMounts()
	case ".unmount":
		s.cmdUnmount(args)
	default:
		s.shellError(fmt.Errorf("unknown command or invalid arguments: %q. Enter \".help\" for help", cmd))
	}
}

func splitDotCommand(line string) []string {
	return strings.Fields(line)
}

func matchesPattern(name, pattern string) bool {
	if pattern == "" {
		return true
	}
	ok, err := filepath.Match(pattern, name)
	if err != nil {
		return strings.Contains(name, pattern)
	}
	return ok
}

func (s *State) cmdTables(args []string) {
	pattern := ""
	if len(args) > 0 {
		pattern = args[0]
	}
	tables, err := db.GetTables(s.DB)
	if err != nil {
		s.shellError(err)
		return
	}
	var names []string
	for _, t := range tables {
		if matchesPattern(t.Name, pattern) {
			names = append(names, t.Name)
		}
	}
	fmt.Fprintln(s.Out, strings.Join(names, "  "))
}

func (s *State) cmdSchema(args []string) {
	tabular := false
	pattern := ""
	for _, a := range args {
		if a == "-t" || a == "--table" {
			tabular = true
			continue
		}
		pattern = a
	}

	tables, err := db.GetTables(s.DB)
	if err != nil {
		s.shellError(err)
		return
	}

	for _, t := range tables {
		if !matchesPattern(t.Name, pattern) {
			continue
		}
		schema, err := db.GetTableSchema(s.DB, t.Name)
		if err != nil {
			s.shellError(err)
			continue
		}
		if tabular {
			cols := []string{"name", "type", "notnull", "dflt_value", "pk"}
			var rows [][]any
			for _, c := range schema.Columns {
				def := any(nil)
				if c.DefaultVal != nil {
					def = *c.DefaultVal
				}
				notnull := 0
				if c.NotNull {
					notnull = 1
				}
				rows = append(rows, []any{c.Name, c.Type, notnull, def, c.PK})
			}
			fmt.Fprintf(s.Out, "-- %s\n", t.Name)
			s.Render(cols, rows)
		} else {
			fmt.Fprintln(s.Out, schema.DDL+";")
		}
	}
}

func (s *State) cmdIndexes(args []string) {
	table := ""
	if len(args) > 0 {
		table = args[0]
	}
	tables, err := db.GetTables(s.DB)
	if err != nil {
		s.shellError(err)
		return
	}
	for _, t := range tables {
		if t.Type != "table" {
			continue
		}
		if table != "" && t.Name != table {
			continue
		}
		schema, err := db.GetTableSchema(s.DB, t.Name)
		if err != nil {
			continue
		}
		for _, idx := range schema.Indexes {
			fmt.Fprintln(s.Out, idx.Name)
		}
	}
}

func (s *State) cmdDatabases() {
	rows, err := s.DB.Query("PRAGMA database_list")
	if err != nil {
		s.shellError(err)
		return
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		s.shellError(err)
		return
	}
	var result [][]any
	for rows.Next() {
		dest := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range ptrs {
			ptrs[i] = &dest[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			s.shellError(err)
			return
		}
		result = append(result, dest)
	}
	s.Render(cols, result)
}

func (s *State) cmdMode(args []string) {
	if len(args) != 1 {
		s.shellError(fmt.Errorf("usage: .mode MODE (one of: %s)", strings.Join(modeNames(), " ")))
		return
	}
	if !IsValidMode(args[0]) {
		s.shellError(fmt.Errorf("unknown mode %q (one of: %s)", args[0], strings.Join(modeNames(), " ")))
		return
	}
	s.Mode = OutputMode(args[0])
}

func modeNames() []string {
	names := make([]string, len(ValidModes))
	for i, m := range ValidModes {
		names[i] = string(m)
	}
	return names
}

func (s *State) cmdHeaders(args []string) {
	if len(args) != 1 || (args[0] != "on" && args[0] != "off") {
		s.shellError(fmt.Errorf("usage: .headers on|off"))
		return
	}
	s.Headers = args[0] == "on"
}

func (s *State) cmdOutput(args []string, once bool) {
	s.closeOnceIfSet()
	if len(args) == 0 {
		s.Out = os.Stdout
		if !once {
			s.outputFilePath = ""
		}
		return
	}
	f, err := os.Create(args[0])
	if err != nil {
		s.shellError(err)
		return
	}
	s.Out = f
	if once {
		s.onceFile = f
	} else {
		s.outputFilePath = args[0]
	}
}

func (s *State) cmdImport(args []string) {
	if !s.Write {
		s.shellError(fmt.Errorf(".import is not allowed in read-only mode (READ_ONLY)"))
		return
	}
	if len(args) != 2 {
		s.shellError(fmt.Errorf("usage: .import FILE TABLE"))
		return
	}
	file, table := args[0], args[1]

	f, err := os.Open(file)
	if err != nil {
		s.shellError(err)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var header []string
	rowCount := 0
	tx, err := s.DB.Begin()
	if err != nil {
		s.shellError(err)
		return
	}
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ",")
		if header == nil {
			header = fields
			continue
		}
		placeholders := strings.TrimRight(strings.Repeat("?,", len(fields)), ",")
		quotedCols := make([]string, len(header))
		for i, h := range header {
			quotedCols[i] = db.QuoteIdentifier(h)
		}
		args := make([]any, len(fields))
		for i, v := range fields {
			args[i] = v
		}
		q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", db.QuoteIdentifier(table), strings.Join(quotedCols, ","), placeholders)
		if _, err := tx.Exec(q, args...); err != nil {
			tx.Rollback()
			s.shellError(err)
			return
		}
		rowCount++
	}
	if err := tx.Commit(); err != nil {
		s.shellError(err)
		return
	}
	if s.Interactive {
		fmt.Fprintf(s.Out, "imported %d rows into %s\n", rowCount, table)
	}
}

func (s *State) cmdDump(args []string) {
	pattern := ""
	if len(args) > 0 {
		pattern = args[0]
	}
	tables, err := db.GetTables(s.DB)
	if err != nil {
		s.shellError(err)
		return
	}
	fmt.Fprintln(s.Out, "BEGIN TRANSACTION;")
	for _, t := range tables {
		if !matchesPattern(t.Name, pattern) {
			continue
		}
		schema, err := db.GetTableSchema(s.DB, t.Name)
		if err != nil {
			continue
		}
		fmt.Fprintln(s.Out, schema.DDL+";")

		rows, err := s.DB.Query(fmt.Sprintf("SELECT * FROM %s", db.QuoteIdentifier(t.Name)))
		if err != nil {
			continue
		}
		cols, vals, err := scanRowsToValues(rows)
		rows.Close()
		if err != nil {
			continue
		}
		quotedCols := make([]string, len(cols))
		for i, c := range cols {
			quotedCols[i] = db.QuoteIdentifier(c)
		}
		for _, row := range vals {
			litVals := make([]string, len(row))
			for i, v := range row {
				lit, err := sqlLiteral(v)
				if err != nil {
					lit = "NULL"
				}
				litVals[i] = lit
			}
			fmt.Fprintf(s.Out, "INSERT INTO %s (%s) VALUES (%s);\n",
				db.QuoteIdentifier(t.Name), strings.Join(quotedCols, ","), strings.Join(litVals, ","))
		}
	}
	fmt.Fprintln(s.Out, "COMMIT;")
}

func (s *State) cmdRead(args []string) {
	if len(args) != 1 {
		s.shellError(fmt.Errorf("usage: .read FILE"))
		return
	}
	f, err := os.Open(args[0])
	if err != nil {
		s.shellError(err)
		return
	}
	defer f.Close()
	if err := RunScript(s, f); err != nil {
		s.shellError(err)
	}
}

// cmdTemplates lists every function callable inside a {{ }} template block
// (internal/seed's generator registry plus its formulaFuncs whitelist), with
// its name, description, usage, and whether the caller needs to add their
// own quotes around the call in the surrounding SQL.
func (s *State) cmdTemplates(args []string) {
	fns := ListTemplateFunctions()
	cols := []string{"name", "description", "usage", "quoting"}
	rows := make([][]string, len(fns))
	for i, fn := range fns {
		rows[i] = []string{fn.Name, fn.Description, fn.Usage, fn.Quoting}
	}
	renderColumn(s.Out, cols, rows, true, s.Colorized)
	fmt.Fprintln(s.Out, `note: functions return raw values, not pre-quoted SQL literals -- write your own '...' around "add quotes" calls (e.g. VALUES ('{{name}}', '{{firstName}}@{{lastName}}.in')); "bare" calls need no quotes; "self-quoted" calls (blob-returning) already include their own delimiters, so don't add quotes around those.`)
}

// cmdHistory lists the interactive session's history (".history"), or
// re-executes entry N (".history N", 1-based, as shown by the list).
func (s *State) cmdHistory(args []string) {
	if len(args) == 0 {
		for i, entry := range s.History {
			fmt.Fprintf(s.Out, "%4d  %s\n", i+1, entry)
		}
		return
	}
	if len(args) != 1 {
		s.shellError(fmt.Errorf("usage: .history ?N?"))
		return
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n < 1 || n > len(s.History) {
		s.shellError(fmt.Errorf("no such history entry: %s", args[0]))
		return
	}
	entry := s.History[n-1]
	fmt.Fprintf(s.Out, "%s\n", entry)
	s.Execute(entry)
}

// cmdEcho expands {{ }} template functions in text (the same
// preprocessTemplate step Execute runs ahead of Classify/execution) and
// prints the result, without ever running it as SQL or dispatching it as a
// dot-command. This lets a generator/formula function call be tried out in
// isolation -- e.g. ".echo {{firstName}}" or ".echo INSERT INTO users
// (name, email) VALUES ({{name}}, {{email}});" -- before using it in a real
// statement.
func (s *State) cmdEcho(text string) {
	if text == "" {
		s.shellError(fmt.Errorf("usage: .echo TEXT"))
		return
	}
	rendered, err := preprocessTemplate(unquoteDotCommandText(text))
	if err != nil {
		s.shellError(err)
		return
	}
	fmt.Fprintln(s.Out, rendered)
}

// cmdPrompt gets or sets the main/continuation prompt templates.
// ".prompt" alone shows both (raw template and rendered preview);
// ".prompt TEXT" sets the main prompt; ".prompt continuation TEXT" sets the
// continuation prompt. TEXT may use {db} and the {red}/{green}/.../{bold}/
// {dim}/{reset} color tags (see RenderPrompt in prompt.go).
func (s *State) cmdPrompt(rest string) {
	if rest == "" {
		const labelWidth = 21 // len("continuation prompt:"), the longer of the two labels
		fmt.Fprintf(s.Out, "%-*s %q  (rendered: %s)\n", labelWidth, "prompt:", s.Prompt, RenderPrompt(s, s.Prompt))
		fmt.Fprintf(s.Out, "%-*s %q  (rendered: %s)\n", labelWidth, "continuation prompt:", s.ContinuationPrompt, RenderPrompt(s, s.ContinuationPrompt))
		return
	}
	if rest == "continuation" || strings.HasPrefix(rest, "continuation ") {
		text := strings.TrimSpace(strings.TrimPrefix(rest, "continuation"))
		if text == "" {
			s.shellError(fmt.Errorf("usage: .prompt continuation TEXT"))
			return
		}
		s.ContinuationPrompt = unquoteDotCommandText(text)
		return
	}
	s.Prompt = unquoteDotCommandText(rest)
}

// unquoteDotCommandText strips one layer of surrounding "..."/'...' quoting
// from a dot-command's raw text argument, so e.g. `.prompt "{bold}...{reset}$
// "` or `.echo "{{weekday}}"` sets/expands the unquoted text rather than
// embedding the literal quote characters in the result (which would
// otherwise show up doubled, e.g. around a generator's own quoted SQL
// literal). Text without matching surrounding quotes is returned unchanged.
func unquoteDotCommandText(text string) string {
	if len(text) >= 2 {
		first, last := text[0], text[len(text)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return text[1 : len(text)-1]
		}
	}
	return text
}
