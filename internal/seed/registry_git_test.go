package seed

import (
	"regexp"
	"testing"
)

func TestGitGenerators(t *testing.T) {
	cases := []struct {
		name string
		re   *regexp.Regexp
	}{
		{"git.branchName", regexp.MustCompile(`^[a-z]+/[a-z]+(-[a-z]+){1,3}$`)},
		{"git.commitMessage", nil},
		{"git.commitShaShort", regexp.MustCompile(`^[0-9a-f]{7}$`)},
		{"git.commitShaLong", regexp.MustCompile(`^[0-9a-f]{40}$`)},
		{"git.commitAuthorName", nil},
		{"git.commitAuthorEmail", regexp.MustCompile(`^\S+@\S+$`)},
		{"git.tagName", regexp.MustCompile(`^v\d+\.\d+\.\d+$`)},
		{"git.tagMessage", nil},
		{"git.remoteName", nil},
		{"git.repoPath", nil},
		{"git.gitignoreEntry", nil},
		{"git.mergeCommitMessage", regexp.MustCompile(`^Merge branch '.+' into `)},
		{"git.conflictMarkerBlock", regexp.MustCompile(`(?s)^<<<<<<< HEAD\n.*=======\n.*>>>>>>> `)},
		{"git.configUserName", nil},
		{"git.stashMessage", regexp.MustCompile(`^WIP on `)},
		{"git.diffHunkHeader", regexp.MustCompile(`^@@ -\d+,\d+ \+\d+,\d+ @@$`)},
		{"git.fileModeChange", regexp.MustCompile(`^old mode \d+\nnew mode \d+$`)},
		{"git.commitTimestamp", nil},
		{"git.blameLine", nil},
		{"git.submoduleName", regexp.MustCompile(`^[a-z]+(-[a-z]+)?$`)},
		{"git.hookName", nil},
	}

	if !Exists("git.branchName") {
		t.Fatal("expected git.branchName to be registered")
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !Exists(tc.name) {
				t.Fatalf("generator %s not registered", tc.name)
			}
			for i := 0; i < 20; i++ {
				v, err := Generate(tc.name, "TEXT", nil)
				if err != nil {
					t.Fatalf("Generate(%s) error: %v", tc.name, err)
				}
				s, ok := v.(string)
				if !ok || s == "" {
					t.Fatalf("Generate(%s) returned empty/non-string value: %#v", tc.name, v)
				}
				if tc.re != nil && !tc.re.MatchString(s) {
					t.Fatalf("Generate(%s) = %q does not match %s", tc.name, s, tc.re.String())
				}
			}
		})
	}
}
