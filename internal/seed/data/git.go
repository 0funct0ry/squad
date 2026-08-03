package data

// GitDevWords is a curated wordlist of common development vocabulary used to
// build plausible branch-name and submodule slugs.
var GitDevWords = []string{
	"auth", "retry", "cache", "parser", "router", "session", "token", "queue",
	"worker", "config", "logger", "metrics", "schema", "migration", "index",
	"validator", "middleware", "handler", "client", "server", "socket", "buffer",
	"scheduler", "throttle", "webhook", "template", "renderer", "encoder",
	"decoder", "compressor",
}

// GitComponentWords is a curated wordlist of component/scope names used for
// conventional-commit scopes.
var GitComponentWords = []string{
	"api", "web", "cli", "db", "auth", "ui", "core", "build", "docs", "ci",
	"deps", "config", "test", "server", "client",
}

// GitBranchPrefixes and GitBranchPrefixWeights drive weighted branch-name
// prefix selection (feature/fix most common).
var GitBranchPrefixes = []string{"feature", "fix", "chore", "hotfix", "release", "docs"}
var GitBranchPrefixWeights = []int{35, 30, 15, 8, 7, 5}

// GitCommitTypes and GitCommitTypeWeights drive weighted conventional-commit
// type selection.
var GitCommitTypes = []string{"fix", "feat", "chore", "refactor", "docs", "test"}
var GitCommitTypeWeights = []int{28, 25, 20, 15, 7, 5}

// GitRemoteNames and GitRemoteNameWeights drive weighted remote-name
// selection (origin most common).
var GitRemoteNames = []string{"origin", "upstream", "fork", "backup"}
var GitRemoteNameWeights = []int{70, 15, 10, 5}

// GitBaseBranches and GitBaseBranchWeights drive weighted base-branch
// selection for merge commits.
var GitBaseBranches = []string{"main", "master", "develop"}
var GitBaseBranchWeights = []int{60, 30, 10}

// GitignoreEntries is a curated list of common .gitignore patterns.
var GitignoreEntries = []string{
	"node_modules/", "*.log", ".env", "dist/", "__pycache__/", "*.pyc",
	".DS_Store", "*.class", "target/", "build/", "vendor/", ".idea/",
	".vscode/", "*.swp", "coverage/", "*.o", "*.exe", ".env.local",
	"bin/", "*.tmp",
}

// GitSubmoduleWords is a curated wordlist of vendor/lib-style names used for
// submodule slugs.
var GitSubmoduleWords = []string{
	"vendor", "shared", "lib", "core", "ui", "sdk", "proto", "utils", "common",
	"kit",
}

// GitHookNames is the list of real git hook names.
var GitHookNames = []string{
	"pre-commit", "pre-push", "commit-msg", "post-checkout", "pre-rebase",
	"post-merge",
}
