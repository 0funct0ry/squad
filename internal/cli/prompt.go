package cli

import (
	"path/filepath"
	"strings"
)

// Default prompt templates. {db} expands to the database's display name;
// the color tags below expand to ANSI escapes (or are stripped when the
// session isn't colorized) -- see RenderPrompt.
const (
	DefaultPrompt             = "squad[{db}]> "
	DefaultContinuationPrompt = "   ...> "
)

// promptColorTags maps a {tagname} placeholder usable inside a prompt
// template to its ANSI escape code, so ".prompt" can support color without
// requiring the user to type raw escape sequences.
var promptColorTags = map[string]string{
	"reset":   "\x1b[0m",
	"bold":    "\x1b[1m",
	"dim":     "\x1b[2m",
	"black":   "\x1b[30m",
	"red":     "\x1b[31m",
	"green":   "\x1b[32m",
	"yellow":  "\x1b[33m",
	"blue":    "\x1b[34m",
	"magenta": "\x1b[35m",
	"cyan":    "\x1b[36m",
	"white":   "\x1b[37m",
}

// dbLabel is the {db} expansion for a prompt template: the database's base
// filename, or its raw path for special forms like ":memory:"/URIs.
func dbLabel(s *State) string {
	if s.Path == "" || s.Path == ":memory:" || strings.HasPrefix(s.Path, "file:") {
		return s.Path
	}
	return filepath.Base(s.Path)
}

// RenderPrompt expands {db} and the color tags in a prompt template. Color
// tags are stripped (not left as literal text) when the session isn't
// colorized, so a colored prompt degrades to clean plain text when piped.
func RenderPrompt(s *State, tmpl string) string {
	out := strings.ReplaceAll(tmpl, "{db}", dbLabel(s))
	for tag, code := range promptColorTags {
		placeholder := "{" + tag + "}"
		if s.Colorized {
			out = strings.ReplaceAll(out, placeholder, code)
		} else {
			out = strings.ReplaceAll(out, placeholder, "")
		}
	}
	return out
}
