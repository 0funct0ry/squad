package seed

import (
	"fmt"
	"strings"
	"time"

	"github.com/0funct0ry/squad/internal/seed/data"
	"github.com/brianvoe/gofakeit/v7"
)

// genGitBranchName builds a "<prefix>/<slug>" branch name; forcedPrefix
// overrides the weighted pick when non-empty.
func genGitBranchName(forcedPrefix string) string {
	prefix := forcedPrefix
	if prefix == "" {
		prefix = weightedPick(data.GitBranchPrefixes, data.GitBranchPrefixWeights)
	}
	slug := kebabSlug(data.GitDevWords, 2, 4)
	return prefix + "/" + slug
}

// genGitCommitMessageSubject builds the "<type>(<scope>)?: <sentence>" part
// of a conventional commit message, without any trailing composition.
func genGitCommitMessageSubject() string {
	commitType := weightedPick(data.GitCommitTypes, data.GitCommitTypeWeights)
	scope := ""
	if gofakeit.Number(0, 99) < 40 {
		scope = "(" + pickFrom(data.GitComponentWords) + ")"
	}
	sentence := strings.ToLower(sentenceWithWordCount(6))
	sentence = strings.TrimSuffix(sentence, ".")
	return fmt.Sprintf("%s%s: %s", commitType, scope, sentence)
}

func genGitTagName(prefix string) string {
	if prefix == "" {
		prefix = "v"
	}
	major := weightedPick([]string{"0", "1", "2"}, []int{20, 60, 20})
	return fmt.Sprintf("%s%s.%d.%d", prefix, major, gofakeit.Number(0, 20), gofakeit.Number(0, 30))
}

func gitGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "git.branchName", Group: "git", Description: "Git branch name", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "prefix", Label: "Prefix", Kind: OptKindSelect, Choices: data.GitBranchPrefixes, Description: "Force a specific branch prefix"},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			return genGitBranchName(optString(opts, "prefix", "")), nil
		}},

		{Name: "git.commitMessage", Group: "git", Description: "Conventional commit message", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return genGitCommitMessageSubject(), nil
		}},

		{Name: "git.commitShaShort", Group: "git", Description: "7-char git commit SHA", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return hexString(7), nil
		}},

		{Name: "git.commitShaLong", Group: "git", Description: "40-char git commit SHA", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return hexString(40), nil
		}},

		{Name: "git.commitAuthorName", Group: "git", Description: "Commit author name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Name(), nil
		}},

		{Name: "git.commitAuthorEmail", Group: "git", Description: "Commit author email", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Email(), nil
		}},

		{Name: "git.tagName", Group: "git", Description: "Semver git tag", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "prefix", Label: "Prefix", Kind: OptKindString, Default: "v"},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			return genGitTagName(optString(opts, "prefix", "v")), nil
		}},

		{Name: "git.tagMessage", Group: "git", Description: "Tag / release message", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			tag := genGitTagName("v")
			if gofakeit.Number(0, 1) == 0 {
				return "Release " + tag, nil
			}
			return sentenceWithWordCount(5), nil
		}},

		{Name: "git.remoteName", Group: "git", Description: "Git remote name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return weightedPick(data.GitRemoteNames, data.GitRemoteNameWeights), nil
		}},

		{Name: "git.repoPath", Group: "git", Description: "Local repository filesystem path", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "base", Label: "Base directory", Kind: OptKindString},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			repoSlug := kebabSlug(data.GitDevWords, 1, 2)
			base := optString(opts, "base", "")
			if base != "" {
				return base + "/" + repoSlug, nil
			}
			if gofakeit.Number(0, 1) == 0 {
				return "/home/" + gofakeit.Username() + "/projects/" + repoSlug, nil
			}
			return "~/code/" + repoSlug, nil
		}},

		{Name: "git.gitignoreEntry", Group: "git", Description: "A common .gitignore pattern", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return pickFrom(data.GitignoreEntries), nil
		}},

		{Name: "git.mergeCommitMessage", Group: "git", Description: "Merge commit message", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			branch := genGitBranchName("")
			base := weightedPick(data.GitBaseBranches, data.GitBaseBranchWeights)
			return fmt.Sprintf("Merge branch '%s' into %s", branch, base), nil
		}},

		{Name: "git.conflictMarkerBlock", Group: "git", Description: "Multi-line merge conflict marker block", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			branch := genGitBranchName("")
			ours := sentenceWithWordCount(4)
			theirs := sentenceWithWordCount(4)
			return fmt.Sprintf("<<<<<<< HEAD\n%s\n=======\n%s\n>>>>>>> %s", ours, theirs, branch), nil
		}},

		{Name: "git.configUserName", Group: "git", Description: "Git config user.name value", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Name(), nil
		}},

		{Name: "git.stashMessage", Group: "git", Description: "Git stash entry message", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			branch := genGitBranchName("")
			sha := hexString(7)
			subject := genGitCommitMessageSubject()
			if idx := strings.Index(subject, ": "); idx != -1 {
				subject = subject[idx+2:]
			}
			return fmt.Sprintf("WIP on %s: %s %s", branch, sha, subject), nil
		}},

		{Name: "git.diffHunkHeader", Group: "git", Description: "Unified diff hunk header", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			a := gofakeit.Number(1, 200)
			b := gofakeit.Number(1, 20)
			c := gofakeit.Number(1, 200)
			d := gofakeit.Number(1, 20)
			return fmt.Sprintf("@@ -%d,%d +%d,%d @@", a, b, c, d), nil
		}},

		{Name: "git.fileModeChange", Group: "git", Description: "File mode change block", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			if gofakeit.Number(0, 1) == 0 {
				return "old mode 100644\nnew mode 100755", nil
			}
			return "old mode 100755\nnew mode 100644", nil
		}},

		{Name: "git.commitTimestamp", Group: "git", Description: "Commit timestamp (git %ad format)", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "from", Label: "From", Kind: OptKindDateTime},
			{Key: "to", Label: "To", Kind: OptKindDateTime},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			from := optTime(opts, "from", time.Now().AddDate(-5, 0, 0))
			to := optTime(opts, "to", time.Now())
			t := gofakeit.DateRange(from, to)
			return t.Format("Mon Jan 2 15:04:05 2006 -0700"), nil
		}},

		{Name: "git.blameLine", Group: "git", Description: "Git blame output line", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			sha := hexString(7)
			author := gofakeit.Name()
			date := gofakeit.DateRange(time.Now().AddDate(-2, 0, 0), time.Now()).Format("2006-01-02")
			lineNo := gofakeit.Number(1, 500)
			source := sentenceWithWordCount(4)
			return fmt.Sprintf("%s (%s %s %d) %s", sha, author, date, lineNo, source), nil
		}},

		{Name: "git.submoduleName", Group: "git", Description: "Submodule name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return kebabSlug(data.GitSubmoduleWords, 1, 2), nil
		}},

		{Name: "git.hookName", Group: "git", Description: "Git hook name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return pickFrom(data.GitHookNames), nil
		}},
	}
}
