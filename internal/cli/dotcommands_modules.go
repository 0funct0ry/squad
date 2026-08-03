package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/0funct0ry/squad/internal/seed"
	"github.com/0funct0ry/squad/internal/vtab"
)

// tokenizeMountArgs splits a `.mount` argument string into shell-like
// tokens, honoring single/double quotes so values containing spaces or
// shell metacharacters (e.g. --regex '[,;]\s*') survive intact. Simpler
// than splitFileAndQuery's bracket/backtick handling (mount flag values
// never need those), but the same quote-aware approach.
func tokenizeMountArgs(text string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	var inSingle, inDouble bool
	haveToken := false

	flush := func() {
		if haveToken {
			tokens = append(tokens, cur.String())
			cur.Reset()
			haveToken = false
		}
	}

	i := 0
	n := len(text)
	for i < n {
		c := text[i]
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
			haveToken = true
		case c == '"' && !inSingle:
			inDouble = !inDouble
			haveToken = true
		case (c == ' ' || c == '\t') && !inSingle && !inDouble:
			flush()
		default:
			cur.WriteByte(c)
			haveToken = true
		}
		i++
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quote in .mount arguments")
	}
	flush()
	return tokens, nil
}

// kebabFlagName converts a snake_case (or already-kebab) argument key into
// its --flag-name form, e.g. multi_doc -> --multi-doc.
func kebabFlagName(key string) string {
	return "--" + strings.ReplaceAll(key, "_", "-")
}

// parseMountFlags binds ordinary CLI flag tokens (--flag value, --flag=value,
// bare/--no- booleans, repeatable --column for the `fake` module) against a
// module's declared argument schema, translating them into the SQL
// key=value form CREATE VIRTUAL TABLE ... USING expects. Errors name the
// offending flag and list the module's accepted flags.
func parseMountFlags(def vtab.ModuleDef, tokens []string) (map[string]string, error) {
	byFlag := make(map[string]seed.OptionField, len(def.Args))
	for _, f := range def.Args {
		byFlag[kebabFlagName(f.Key)] = f
	}

	accepted := func() string {
		names := make([]string, 0, len(def.Args))
		for _, f := range def.Args {
			names = append(names, kebabFlagName(f.Key))
		}
		sort.Strings(names)
		return strings.Join(names, ", ")
	}

	args := make(map[string]string)
	var columnPairs []string

	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		if !strings.HasPrefix(tok, "--") {
			return nil, fmt.Errorf("unexpected argument %q (expected a --flag); accepted flags: %s", tok, accepted())
		}

		name := tok
		var inlineVal string
		hasInline := false
		if eq := strings.IndexByte(tok, '='); eq >= 0 {
			name = tok[:eq]
			inlineVal = tok[eq+1:]
			hasInline = true
		}

		// `fake`'s repeatable --column <name>=<generator> pair is the one
		// dynamic-argument module; it isn't part of the declared schema.
		if def.Name == "fake" && name == "--column" {
			var val string
			if hasInline {
				val = inlineVal
			} else {
				i++
				if i >= len(tokens) {
					return nil, fmt.Errorf("--column requires a value, e.g. --column email=email")
				}
				val = tokens[i]
			}
			columnPairs = append(columnPairs, val)
			i++
			continue
		}

		if strings.HasPrefix(name, "--no-") {
			base := "--" + strings.TrimPrefix(name, "--no-")
			field, ok := byFlag[base]
			if !ok || field.Kind != seed.OptKindBool {
				return nil, fmt.Errorf("unknown flag %q; accepted flags: %s", tok, accepted())
			}
			args[field.Key] = "false"
			i++
			continue
		}

		field, ok := byFlag[name]
		if !ok {
			return nil, fmt.Errorf("unknown flag %q; accepted flags: %s", tok, accepted())
		}

		if field.Kind == seed.OptKindBool && !hasInline {
			// Bare boolean flag: presence means true, unless the next token
			// is itself an explicit value (rare, but --header true should
			// still work rather than erroring).
			if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "--") {
				args[field.Key] = tokens[i+1]
				i += 2
				continue
			}
			args[field.Key] = "true"
			i++
			continue
		}

		var val string
		if hasInline {
			val = inlineVal
			i++
		} else {
			if i+1 >= len(tokens) {
				return nil, fmt.Errorf("flag %s requires a value", name)
			}
			val = tokens[i+1]
			i += 2
		}
		args[field.Key] = val
	}

	for _, f := range def.Args {
		if f.Required {
			if _, ok := args[f.Key]; !ok {
				return nil, fmt.Errorf("missing required flag %s; accepted flags: %s", kebabFlagName(f.Key), accepted())
			}
		}
	}

	if def.Name == "fake" {
		for _, pair := range columnPairs {
			eq := strings.IndexByte(pair, '=')
			if eq < 0 {
				return nil, fmt.Errorf("--column value must be name=generator, got %q", pair)
			}
			args[pair[:eq]] = pair[eq+1:]
		}
	}

	return args, nil
}

// cmdMount implements ".mount MODULE ALIAS --flag value ...". Called from
// dispatchDotCommand's free-text prefix phase (like .save/.shell) because
// flag values legitimately contain spaces/globs that a tokenized-phase
// strings.Fields split would shred.
func (s *State) cmdMount(rest string) {
	tokens, err := tokenizeMountArgs(rest)
	if err != nil {
		s.shellError(err)
		return
	}
	if len(tokens) < 2 {
		s.shellError(fmt.Errorf("usage: .mount MODULE ALIAS --flag value ..."))
		return
	}
	moduleName, alias := tokens[0], tokens[1]
	flagTokens := tokens[2:]

	if !s.ModulesEnabled {
		s.shellError(fmt.Errorf("virtual table modules are off; relaunch with --modules to enable them"))
		return
	}

	def, ok := vtab.Get(moduleName)
	if !ok {
		s.shellError(fmt.Errorf("unknown module %q; run .modules to list available modules", moduleName))
		return
	}

	args, err := parseMountFlags(def, flagTokens)
	if err != nil {
		s.shellError(err)
		return
	}

	m, err := vtab.CreateMount(context.Background(), s.DB, s.MountStore, moduleName, alias, args)
	if err != nil {
		s.shellError(err)
		return
	}

	s.invalidateSchemaCache()
	fmt.Fprintf(s.Out, "mounted %s as %s (%d columns)\n", moduleName, alias, len(m.DeclaredColumns))
}

// cmdUnmount implements ".unmount ALIAS".
func (s *State) cmdUnmount(args []string) {
	if len(args) != 1 {
		s.shellError(fmt.Errorf("usage: .unmount ALIAS"))
		return
	}
	if !vtab.DropMount(s.MountStore, args[0]) {
		s.shellError(fmt.Errorf("no active mount named %q", args[0]))
		return
	}
	s.invalidateSchemaCache()
	fmt.Fprintf(s.Out, "unmounted %s\n", args[0])
}

// cmdMounts implements ".mounts": list active mounts.
func (s *State) cmdMounts() {
	mounts := s.MountStore.List()
	if len(mounts) == 0 {
		fmt.Fprintln(s.Out, "(no active mounts)")
		return
	}
	for _, m := range mounts {
		fmt.Fprintf(s.Out, "%-20s %-12s %d columns\n", m.Alias, m.Module, len(m.DeclaredColumns))
	}
}

// cmdModules implements ".modules" (list the catalog) and ".modules NAME"
// (print one module's flags and columns), sharing vtab.Catalog() with the
// GET /api/modules web endpoint.
func (s *State) cmdModules(args []string) {
	if len(args) == 0 {
		if !s.ModulesEnabled {
			fmt.Fprintln(s.Out, "virtual table modules are off; relaunch with --modules to enable them")
		}
		for _, mod := range vtab.Catalog() {
			fmt.Fprintf(s.Out, "%-12s %-12s %s\n", mod.Name, mod.Group, mod.Description)
		}
		return
	}

	def, ok := vtab.Get(args[0])
	if !ok {
		s.shellError(fmt.Errorf("unknown module %q; run .modules to list available modules", args[0]))
		return
	}
	fmt.Fprintf(s.Out, "%s — %s\n", def.Name, def.Description)
	for _, f := range def.Args {
		req := ""
		if f.Required {
			req = " (required)"
		}
		fmt.Fprintf(s.Out, "  %-20s %s%s\n", kebabFlagName(f.Key), f.Description, req)
	}
}

// moduleNames returns every registered module's name, for .modules/.mount
// completion.
func moduleNames() []string {
	cat := vtab.Catalog()
	names := make([]string, len(cat))
	for i, m := range cat {
		names[i] = m.Name
	}
	return names
}

// mountAliasNames returns every active mount's alias, for .unmount
// completion.
func (s *State) mountAliasNames() []string {
	mounts := s.MountStore.List()
	names := make([]string, len(mounts))
	for i, m := range mounts {
		names[i] = m.Alias
	}
	return names
}

// moduleFlagNames returns a module's declared --flag names, for .mount's
// third completion level.
func moduleFlagNames(moduleName string) []string {
	def, ok := vtab.Get(moduleName)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(def.Args))
	for _, f := range def.Args {
		names = append(names, kebabFlagName(f.Key))
		if f.Kind == seed.OptKindBool {
			names = append(names, "--no-"+strings.TrimPrefix(kebabFlagName(f.Key), "--"))
		}
	}
	sort.Strings(names)
	return names
}
