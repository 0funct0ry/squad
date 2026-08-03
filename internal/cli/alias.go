package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// aliasKeywordDenylist blocks defining an alias whose name would shadow a
// core SQL keyword and silently break normal statement execution (aliases
// only -- abbreviations never fire on a bare leading token, so they don't
// need this check).
var aliasKeywordDenylist = map[string]bool{
	"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true,
	"CREATE": true, "DROP": true, "ALTER": true, "WITH": true, "EXPLAIN": true,
}

// aliasesFilePath returns ~/.squad_aliases, or "" if the home dir can't be
// resolved -- same silent-skip-on-failure convention as bookmarksFilePath.
func aliasesFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".squad_aliases")
}

// abbrsFilePath returns ~/.squad_abbrs, or "" if the home dir can't be
// resolved.
func abbrsFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".squad_abbrs")
}

func (s *State) loadAliases() map[string]string {
	if s.Aliases != nil {
		return s.Aliases
	}
	s.Aliases = map[string]string{}
	path := aliasesFilePath()
	if path == "" {
		return s.Aliases
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s.Aliases
	}
	_ = json.Unmarshal(data, &s.Aliases)
	return s.Aliases
}

func (s *State) saveAliasesToDisk() {
	path := aliasesFilePath()
	if path == "" {
		return
	}
	data, err := json.MarshalIndent(s.Aliases, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

func (s *State) loadAbbrs() map[string]string {
	if s.Abbrs != nil {
		return s.Abbrs
	}
	s.Abbrs = map[string]string{}
	path := abbrsFilePath()
	if path == "" {
		return s.Abbrs
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s.Abbrs
	}
	_ = json.Unmarshal(data, &s.Abbrs)
	return s.Abbrs
}

func (s *State) saveAbbrsToDisk() {
	path := abbrsFilePath()
	if path == "" {
		return
	}
	data, err := json.MarshalIndent(s.Abbrs, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

// validAliasOrAbbrName rejects a NAME that would shadow an existing
// dot-command (both alias and abbr), and, when checkSQLKeyword is true
// (aliases only), a core SQL keyword that would break normal statement
// execution if silently rewritten.
func validAliasOrAbbrName(cmdLabel, name string, checkSQLKeyword bool) error {
	lower := strings.ToLower(name)
	for _, dc := range dotCommandNames {
		if strings.ToLower(strings.TrimPrefix(dc, ".")) == lower {
			return fmt.Errorf("%s: %q would shadow a dot-command", cmdLabel, name)
		}
	}
	if checkSQLKeyword && aliasKeywordDenylist[strings.ToUpper(name)] {
		return fmt.Errorf("%s: %q would shadow a SQL keyword", cmdLabel, name)
	}
	return nil
}

// cmdAlias implements ".alias" (list), ".alias NAME=EXPANSION" (define),
// and ".alias -d NAME" (delete). rest is the raw text after ".alias",
// unsplit, so EXPANSION can contain spaces.
func (s *State) cmdAlias(rest string) {
	s.loadAliases()
	s.genericAliasCmd(".alias", rest, s.Aliases, s.saveAliasesToDisk, true)
}

// cmdUnalias implements ".unalias NAME".
func (s *State) cmdUnalias(args []string) {
	s.loadAliases()
	s.genericUnaliasCmd(".unalias", args, s.Aliases, s.saveAliasesToDisk)
}

// cmdAliases implements ".aliases": list all defined aliases.
func (s *State) cmdAliases() {
	s.loadAliases()
	listNamedExpansions(s.Out, s.Aliases, "(no aliases defined)")
}

// cmdAbbr implements ".abbr" (list), ".abbr NAME=EXPANSION" (define), and
// ".abbr -d NAME" (delete).
func (s *State) cmdAbbr(rest string) {
	s.loadAbbrs()
	s.genericAliasCmd(".abbr", rest, s.Abbrs, s.saveAbbrsToDisk, false)
}

// cmdUnabbr implements ".unabbr NAME".
func (s *State) cmdUnabbr(args []string) {
	s.loadAbbrs()
	s.genericUnaliasCmd(".unabbr", args, s.Abbrs, s.saveAbbrsToDisk)
}

// cmdAbbrs implements ".abbrs": list all defined abbreviations.
func (s *State) cmdAbbrs() {
	s.loadAbbrs()
	listNamedExpansions(s.Out, s.Abbrs, "(no abbreviations defined)")
}

// genericAliasCmd handles the shared "list / define NAME=EXPANSION / delete
// -d NAME" shape for both .alias and .abbr.
func (s *State) genericAliasCmd(cmdLabel, rest string, table map[string]string, save func(), checkSQLKeyword bool) {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		listNamedExpansions(s.Out, table, fmt.Sprintf("(no %s defined)", pluralLabel(cmdLabel)))
		return
	}
	if rest == "-d" || strings.HasPrefix(rest, "-d ") {
		name := strings.TrimSpace(strings.TrimPrefix(rest, "-d"))
		s.deleteNamedExpansion(cmdLabel, name, table, save)
		return
	}
	eq := strings.IndexByte(rest, '=')
	if eq < 0 {
		s.shellError(fmt.Errorf("usage: %s NAME=EXPANSION", cmdLabel))
		return
	}
	name := strings.TrimSpace(rest[:eq])
	expansion := strings.TrimSpace(rest[eq+1:])
	if name == "" || expansion == "" {
		s.shellError(fmt.Errorf("usage: %s NAME=EXPANSION", cmdLabel))
		return
	}
	if err := validAliasOrAbbrName(cmdLabel, name, checkSQLKeyword); err != nil {
		s.shellError(err)
		return
	}
	table[name] = expansion
	save()
	fmt.Fprintf(s.Out, "%s '%s' defined\n", strings.TrimPrefix(cmdLabel, "."), name)
}

// genericUnaliasCmd handles ".unalias NAME" / ".unabbr NAME".
func (s *State) genericUnaliasCmd(cmdLabel string, args []string, table map[string]string, save func()) {
	if len(args) != 1 {
		s.shellError(fmt.Errorf("usage: %s NAME", cmdLabel))
		return
	}
	s.deleteNamedExpansion(cmdLabel, args[0], table, save)
}

func (s *State) deleteNamedExpansion(cmdLabel, name string, table map[string]string, save func()) {
	if name == "" {
		s.shellError(fmt.Errorf("usage: %s -d NAME", cmdLabel))
		return
	}
	noun := "alias"
	if strings.Contains(cmdLabel, "abbr") {
		noun = "abbreviation"
	}
	if _, ok := table[name]; !ok {
		s.shellError(fmt.Errorf("no such %s: %s", noun, name))
		return
	}
	delete(table, name)
	save()
	fmt.Fprintf(s.Out, "%s '%s' removed\n", noun, name)
}

func pluralLabel(cmdLabel string) string {
	if cmdLabel == ".alias" {
		return "aliases"
	}
	return "abbreviations"
}

func listNamedExpansions(w interface{ Write([]byte) (int, error) }, table map[string]string, emptyMsg string) {
	if len(table) == 0 {
		fmt.Fprintln(w, emptyMsg)
		return
	}
	names := make([]string, 0, len(table))
	for k := range table {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(w, "%-20s %s\n", n, table[n])
	}
}

// expandAlias splits trimmed on the first whitespace boundary and, if the
// leading token matches a defined alias, splices in that alias's expansion,
// preserving everything after it (including whitespace) verbatim. Expansion
// is single-pass: the replacement text is never itself re-checked against
// the alias table.
func (s *State) expandAlias(trimmed string) string {
	s.loadAliases()
	if len(s.Aliases) == 0 {
		return trimmed
	}
	idx := strings.IndexAny(trimmed, " \t")
	var head, tail string
	if idx < 0 {
		head, tail = trimmed, ""
	} else {
		head, tail = trimmed[:idx], trimmed[idx:]
	}
	if expansion, ok := s.Aliases[head]; ok {
		return expansion + tail
	}
	return trimmed
}

// abbrNames returns every defined abbreviation's name, prefixed with the
// configured abbr trigger, for trigger-prefix completion.
func (s *State) abbrNames() []string {
	s.loadAbbrs()
	names := make([]string, 0, len(s.Abbrs))
	for k := range s.Abbrs {
		names = append(names, s.abbrTrigger()+k)
	}
	sort.Strings(names)
	return names
}
