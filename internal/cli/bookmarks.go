package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// bookmarkProfile is a saved snapshot of session display settings, persisted
// to ~/.squad_bookmarks by ".bookmark save".
type bookmarkProfile struct {
	Mode       OutputMode `json:"mode"`
	Headers    bool       `json:"headers"`
	NullValue  string     `json:"nullValue"`
	Prompt     string     `json:"prompt"`
	OutputFile string     `json:"outputFile,omitempty"`
}

// bookmarksFilePath returns ~/.squad_bookmarks, or "" if the home dir can't
// be resolved -- same silent-skip-on-failure convention as historyFilePath.
func bookmarksFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".squad_bookmarks")
}

func (s *State) loadBookmarks() map[string]bookmarkProfile {
	if s.Bookmarks != nil {
		return s.Bookmarks
	}
	s.Bookmarks = map[string]bookmarkProfile{}
	path := bookmarksFilePath()
	if path == "" {
		return s.Bookmarks
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s.Bookmarks
	}
	_ = json.Unmarshal(data, &s.Bookmarks)
	return s.Bookmarks
}

func (s *State) saveBookmarksToDisk() {
	path := bookmarksFilePath()
	if path == "" {
		return
	}
	data, err := json.MarshalIndent(s.Bookmarks, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

// cmdBookmark implements ".bookmark ?save|load? ?NAME?": explicit subverbs
// disambiguate save-vs-restore (default verb "save", default name "default").
func (s *State) cmdBookmark(args []string) {
	s.loadBookmarks()
	name := "default"
	verb := "save"
	switch len(args) {
	case 0:
		// bare ".bookmark" -> save "default"
	case 1:
		if args[0] == "save" || args[0] == "load" {
			verb = args[0]
		} else {
			name = args[0]
		}
	case 2:
		verb, name = args[0], args[1]
	default:
		s.shellError(fmt.Errorf("usage: .bookmark ?save|load? ?NAME?"))
		return
	}

	switch verb {
	case "save":
		s.Bookmarks[name] = bookmarkProfile{
			Mode:       s.Mode,
			Headers:    s.Headers,
			NullValue:  s.NullValue,
			Prompt:     s.Prompt,
			OutputFile: s.outputFilePath,
		}
		s.saveBookmarksToDisk()
		fmt.Fprintf(s.Out, "saved bookmark %q\n", name)
	case "load":
		prof, ok := s.Bookmarks[name]
		if !ok {
			s.shellError(fmt.Errorf("no such bookmark: %s", name))
			return
		}
		s.Mode, s.Headers, s.NullValue, s.Prompt = prof.Mode, prof.Headers, prof.NullValue, prof.Prompt
		if prof.OutputFile != "" {
			s.cmdOutput([]string{prof.OutputFile}, false)
		} else {
			s.cmdOutput(nil, false)
		}
		fmt.Fprintf(s.Out, "loaded bookmark %q\n", name)
	default:
		s.shellError(fmt.Errorf("usage: .bookmark save|load ?NAME?"))
	}
}

func (s *State) cmdBookmarks() {
	s.loadBookmarks()
	names := make([]string, 0, len(s.Bookmarks))
	for k := range s.Bookmarks {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintln(s.Out, n)
	}
}
