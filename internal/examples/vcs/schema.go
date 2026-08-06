package vcs

const Schema = `-- Version Control System Schema

PRAGMA foreign_keys = ON;

CREATE TABLE users (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    username        TEXT NOT NULL UNIQUE,
    email           TEXT NOT NULL UNIQUE,
    full_name       TEXT,
    avatar_url      TEXT,
    bio             TEXT,
    password_hash   TEXT NOT NULL,
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    is_site_admin   INTEGER NOT NULL DEFAULT 0 CHECK (is_site_admin IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE organizations (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL UNIQUE,
    display_name    TEXT,
    description     TEXT,
    avatar_url      TEXT,
    owner_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE organization_members (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner','admin','member')),
    joined_at       TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (organization_id, user_id)
);

CREATE TABLE repositories (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_user_id           INTEGER REFERENCES users(id) ON DELETE CASCADE,
    owner_org_id            INTEGER REFERENCES organizations(id) ON DELETE CASCADE,
    name                    TEXT NOT NULL,
    description             TEXT,
    is_private              INTEGER NOT NULL DEFAULT 0 CHECK (is_private IN (0,1)),
    is_archived             INTEGER NOT NULL DEFAULT 0 CHECK (is_archived IN (0,1)),
    is_fork                 INTEGER NOT NULL DEFAULT 0 CHECK (is_fork IN (0,1)),
    forked_from_repo_id     INTEGER REFERENCES repositories(id) ON DELETE SET NULL,
    default_branch_name     TEXT NOT NULL DEFAULT 'main',
    primary_language        TEXT,
    star_count              INTEGER NOT NULL DEFAULT 0,
    fork_count              INTEGER NOT NULL DEFAULT 0,
    watch_count             INTEGER NOT NULL DEFAULT 0,
    open_issue_count        INTEGER NOT NULL DEFAULT 0,
    size_kb                 INTEGER NOT NULL DEFAULT 0,
    created_at              TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at              TEXT NOT NULL DEFAULT (datetime('now')),
    pushed_at               TEXT,
    CHECK (
        (owner_user_id IS NOT NULL AND owner_org_id IS NULL)
        OR (owner_user_id IS NULL AND owner_org_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX idx_repos_user_owner_name
    ON repositories(owner_user_id, name) WHERE owner_user_id IS NOT NULL;
CREATE UNIQUE INDEX idx_repos_org_owner_name
    ON repositories(owner_org_id, name) WHERE owner_org_id IS NOT NULL;
CREATE INDEX idx_repos_forked_from ON repositories(forked_from_repo_id);

CREATE TABLE repository_collaborators (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    repository_id   INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission      TEXT NOT NULL DEFAULT 'read'
                    CHECK (permission IN ('read','triage','write','maintain','admin')),
    added_at        TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (repository_id, user_id)
);

CREATE TABLE deploy_keys (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    repository_id   INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    public_key      TEXT NOT NULL UNIQUE,
    is_read_only    INTEGER NOT NULL DEFAULT 1 CHECK (is_read_only IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE webhooks (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    repository_id   INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    url             TEXT NOT NULL,
    secret          TEXT,
    events          TEXT NOT NULL DEFAULT '[]',   -- JSON array, e.g. '["push","pull_request"]'
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE commits (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    repository_id   INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    sha             TEXT NOT NULL,
    author_id       INTEGER REFERENCES users(id) ON DELETE SET NULL,
    author_name     TEXT NOT NULL,
    author_email    TEXT NOT NULL,
    committer_id    INTEGER REFERENCES users(id) ON DELETE SET NULL,
    committer_name  TEXT NOT NULL,
    committer_email TEXT NOT NULL,
    message         TEXT NOT NULL,
    authored_at     TEXT NOT NULL,
    committed_at    TEXT NOT NULL,
    additions       INTEGER NOT NULL DEFAULT 0,
    deletions       INTEGER NOT NULL DEFAULT 0,
    changed_files   INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (repository_id, sha)
);

CREATE INDEX idx_commits_repo ON commits(repository_id);
CREATE INDEX idx_commits_author ON commits(author_id);
CREATE INDEX idx_commits_committed_at ON commits(committed_at);

CREATE TABLE commit_parents (
    commit_id        INTEGER NOT NULL REFERENCES commits(id) ON DELETE CASCADE,
    parent_commit_id INTEGER NOT NULL REFERENCES commits(id) ON DELETE CASCADE,
    parent_order     INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (commit_id, parent_commit_id),
    CHECK (commit_id <> parent_commit_id)
);
CREATE INDEX idx_commit_parents_parent ON commit_parents(parent_commit_id);

CREATE TABLE commit_files (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    commit_id       INTEGER NOT NULL REFERENCES commits(id) ON DELETE CASCADE,
    file_path       TEXT NOT NULL,
    previous_path   TEXT,
    change_type     TEXT NOT NULL CHECK (change_type IN ('added','modified','deleted','renamed')),
    additions       INTEGER NOT NULL DEFAULT 0,
    deletions       INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_commit_files_commit ON commit_files(commit_id);
CREATE INDEX idx_commit_files_path ON commit_files(file_path);

CREATE TABLE commit_statuses (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    commit_id       INTEGER NOT NULL REFERENCES commits(id) ON DELETE CASCADE,
    context         TEXT NOT NULL,
    state           TEXT NOT NULL CHECK (state IN ('pending','success','failure','error')),
    description     TEXT,
    target_url      TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (commit_id, context)
);

CREATE TABLE branches (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    repository_id   INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    head_commit_id  INTEGER REFERENCES commits(id) ON DELETE SET NULL,
    is_default      INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0,1)),
    is_protected    INTEGER NOT NULL DEFAULT 0 CHECK (is_protected IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (repository_id, name)
);
CREATE INDEX idx_branches_repo ON branches(repository_id);
CREATE INDEX idx_branches_head_commit ON branches(head_commit_id);

CREATE TABLE tags (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    repository_id   INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    commit_id       INTEGER NOT NULL REFERENCES commits(id) ON DELETE CASCADE,
    tagger_id       INTEGER REFERENCES users(id) ON DELETE SET NULL,
    message         TEXT,
    is_release      INTEGER NOT NULL DEFAULT 0 CHECK (is_release IN (0,1)),
    release_title   TEXT,
    release_body    TEXT,
    is_prerelease   INTEGER NOT NULL DEFAULT 0 CHECK (is_prerelease IN (0,1)),
    is_draft        INTEGER NOT NULL DEFAULT 0 CHECK (is_draft IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (repository_id, name)
);
CREATE INDEX idx_tags_repo ON tags(repository_id);
CREATE INDEX idx_tags_commit ON tags(commit_id);

CREATE TABLE labels (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    repository_id   INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    color           TEXT NOT NULL DEFAULT 'ededed',
    description     TEXT,
    UNIQUE (repository_id, name)
);

CREATE TABLE milestones (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    repository_id   INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    description     TEXT,
    state           TEXT NOT NULL DEFAULT 'open' CHECK (state IN ('open','closed')),
    due_date        TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    closed_at       TEXT,
    UNIQUE (repository_id, title)
);

CREATE TABLE issues (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    repository_id   INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    number          INTEGER NOT NULL,
    title           TEXT NOT NULL,
    body            TEXT,
    author_id       INTEGER REFERENCES users(id) ON DELETE SET NULL,
    state           TEXT NOT NULL DEFAULT 'open' CHECK (state IN ('open','closed')),
    milestone_id    INTEGER REFERENCES milestones(id) ON DELETE SET NULL,
    is_locked       INTEGER NOT NULL DEFAULT 0 CHECK (is_locked IN (0,1)),
    comment_count   INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    closed_at       TEXT,
    closed_by       INTEGER REFERENCES users(id) ON DELETE SET NULL,
    UNIQUE (repository_id, number)
);
CREATE INDEX idx_issues_repo_state ON issues(repository_id, state);
CREATE INDEX idx_issues_author ON issues(author_id);
CREATE INDEX idx_issues_milestone ON issues(milestone_id);

CREATE TABLE issue_labels (
    issue_id        INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    label_id        INTEGER NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, label_id)
);

CREATE TABLE issue_assignees (
    issue_id        INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assigned_at     TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (issue_id, user_id)
);

CREATE TABLE issue_comments (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id        INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    author_id       INTEGER REFERENCES users(id) ON DELETE SET NULL,
    body            TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_issue_comments_issue ON issue_comments(issue_id);

CREATE TABLE pull_requests (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    repository_id           INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    number                  INTEGER NOT NULL,
    title                   TEXT NOT NULL,
    body                    TEXT,
    author_id               INTEGER REFERENCES users(id) ON DELETE SET NULL,
    state                   TEXT NOT NULL DEFAULT 'open' CHECK (state IN ('open','closed','merged')),
    source_repository_id    INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    source_branch_id        INTEGER REFERENCES branches(id) ON DELETE SET NULL,
    target_branch_id        INTEGER NOT NULL REFERENCES branches(id) ON DELETE RESTRICT,
    is_draft                INTEGER NOT NULL DEFAULT 0 CHECK (is_draft IN (0,1)),
    is_locked               INTEGER NOT NULL DEFAULT 0 CHECK (is_locked IN (0,1)),
    milestone_id            INTEGER REFERENCES milestones(id) ON DELETE SET NULL,
    merged_at               TEXT,
    merged_by               INTEGER REFERENCES users(id) ON DELETE SET NULL,
    merge_commit_id         INTEGER REFERENCES commits(id) ON DELETE SET NULL,
    additions               INTEGER NOT NULL DEFAULT 0,
    deletions               INTEGER NOT NULL DEFAULT 0,
    changed_files           INTEGER NOT NULL DEFAULT 0,
    commit_count            INTEGER NOT NULL DEFAULT 0,
    comment_count           INTEGER NOT NULL DEFAULT 0,
    review_comment_count    INTEGER NOT NULL DEFAULT 0,
    created_at              TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at              TEXT NOT NULL DEFAULT (datetime('now')),
    closed_at               TEXT,
    UNIQUE (repository_id, number)
);
CREATE INDEX idx_prs_repo_state ON pull_requests(repository_id, state);
CREATE INDEX idx_prs_author ON pull_requests(author_id);
CREATE INDEX idx_prs_target_branch ON pull_requests(target_branch_id);
CREATE INDEX idx_prs_source_branch ON pull_requests(source_branch_id);

CREATE TABLE pull_request_labels (
    pull_request_id INTEGER NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
    label_id         INTEGER NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
    PRIMARY KEY (pull_request_id, label_id)
);

CREATE TABLE pull_request_assignees (
    pull_request_id INTEGER NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
    user_id          INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assigned_at      TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (pull_request_id, user_id)
);

CREATE TABLE pull_request_reviewers (
    pull_request_id INTEGER NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
    user_id          INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    requested_at     TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (pull_request_id, user_id)
);

CREATE TABLE pull_request_commits (
    pull_request_id INTEGER NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
    commit_id        INTEGER NOT NULL REFERENCES commits(id) ON DELETE CASCADE,
    PRIMARY KEY (pull_request_id, commit_id)
);

CREATE TABLE reviews (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    pull_request_id INTEGER NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
    reviewer_id     INTEGER REFERENCES users(id) ON DELETE SET NULL,
    commit_id       INTEGER REFERENCES commits(id) ON DELETE SET NULL,
    state           TEXT NOT NULL DEFAULT 'pending'
                    CHECK (state IN ('pending','commented','approved','changes_requested','dismissed')),
    body            TEXT,
    submitted_at    TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_reviews_pr ON reviews(pull_request_id);
CREATE INDEX idx_reviews_reviewer ON reviews(reviewer_id);

CREATE TABLE review_comments (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    review_id       INTEGER REFERENCES reviews(id) ON DELETE CASCADE,
    pull_request_id INTEGER NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
    author_id       INTEGER REFERENCES users(id) ON DELETE SET NULL,
    file_path       TEXT NOT NULL,
    line_number     INTEGER,
    diff_hunk       TEXT,
    body            TEXT NOT NULL,
    in_reply_to_id  INTEGER REFERENCES review_comments(id) ON DELETE SET NULL,
    is_resolved     INTEGER NOT NULL DEFAULT 0 CHECK (is_resolved IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_review_comments_pr ON review_comments(pull_request_id);
CREATE INDEX idx_review_comments_review ON review_comments(review_id);
CREATE INDEX idx_review_comments_thread ON review_comments(in_reply_to_id);

CREATE TABLE stars (
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    repository_id   INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    starred_at      TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (user_id, repository_id)
);

CREATE TABLE watches (
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    repository_id   INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    watched_at      TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (user_id, repository_id)
);

CREATE TRIGGER trg_users_updated_at
AFTER UPDATE ON users
FOR EACH ROW BEGIN
    UPDATE users SET updated_at = datetime('now') WHERE id = NEW.id;
END;

CREATE TRIGGER trg_repositories_updated_at
AFTER UPDATE ON repositories
FOR EACH ROW BEGIN
    UPDATE repositories SET updated_at = datetime('now') WHERE id = NEW.id;
END;

CREATE TRIGGER trg_branches_updated_at
AFTER UPDATE ON branches
FOR EACH ROW BEGIN
    UPDATE branches SET updated_at = datetime('now') WHERE id = NEW.id;
END;

CREATE TRIGGER trg_issues_updated_at
AFTER UPDATE ON issues
FOR EACH ROW BEGIN
    UPDATE issues SET updated_at = datetime('now') WHERE id = NEW.id;
END;

CREATE TRIGGER trg_pull_requests_updated_at
AFTER UPDATE ON pull_requests
FOR EACH ROW BEGIN
    UPDATE pull_requests SET updated_at = datetime('now') WHERE id = NEW.id;
END;

CREATE TRIGGER trg_reviews_updated_at
AFTER UPDATE ON reviews
FOR EACH ROW BEGIN
    UPDATE reviews SET updated_at = datetime('now') WHERE id = NEW.id;
END;

CREATE TRIGGER trg_review_comments_updated_at
AFTER UPDATE ON review_comments
FOR EACH ROW BEGIN
    UPDATE review_comments SET updated_at = datetime('now') WHERE id = NEW.id;
END;

CREATE TRIGGER trg_stars_insert_count
AFTER INSERT ON stars
FOR EACH ROW BEGIN
    UPDATE repositories SET star_count = star_count + 1 WHERE id = NEW.repository_id;
END;

CREATE TRIGGER trg_stars_delete_count
AFTER DELETE ON stars
FOR EACH ROW BEGIN
    UPDATE repositories SET star_count = star_count - 1 WHERE id = OLD.repository_id;
END;

CREATE TRIGGER trg_watches_insert_count
AFTER INSERT ON watches
FOR EACH ROW BEGIN
    UPDATE repositories SET watch_count = watch_count + 1 WHERE id = NEW.repository_id;
END;

CREATE TRIGGER trg_watches_delete_count
AFTER DELETE ON watches
FOR EACH ROW BEGIN
    UPDATE repositories SET watch_count = watch_count - 1 WHERE id = OLD.repository_id;
END;

CREATE TRIGGER trg_repositories_fork_insert_count
AFTER INSERT ON repositories
FOR EACH ROW WHEN NEW.forked_from_repo_id IS NOT NULL
BEGIN
    UPDATE repositories SET fork_count = fork_count + 1 WHERE id = NEW.forked_from_repo_id;
END;

CREATE TRIGGER trg_issues_open_count_insert
AFTER INSERT ON issues
FOR EACH ROW WHEN NEW.state = 'open'
BEGIN
    UPDATE repositories SET open_issue_count = open_issue_count + 1 WHERE id = NEW.repository_id;
END;

CREATE TRIGGER trg_issues_open_count_update
AFTER UPDATE OF state ON issues
FOR EACH ROW WHEN OLD.state <> NEW.state
BEGIN
    UPDATE repositories
       SET open_issue_count = open_issue_count
           + (CASE WHEN NEW.state = 'open' THEN 1 ELSE 0 END)
           - (CASE WHEN OLD.state = 'open' THEN 1 ELSE 0 END)
     WHERE id = NEW.repository_id;
END;

CREATE TRIGGER trg_issue_comments_count_insert
AFTER INSERT ON issue_comments
FOR EACH ROW BEGIN
    UPDATE issues SET comment_count = comment_count + 1 WHERE id = NEW.issue_id;
END;

CREATE TRIGGER trg_issue_comments_count_delete
AFTER DELETE ON issue_comments
FOR EACH ROW BEGIN
    UPDATE issues SET comment_count = comment_count - 1 WHERE id = OLD.issue_id;
END;

CREATE TRIGGER trg_pr_commits_count_insert
AFTER INSERT ON pull_request_commits
FOR EACH ROW BEGIN
    UPDATE pull_requests SET commit_count = commit_count + 1 WHERE id = NEW.pull_request_id;
END;

CREATE TRIGGER trg_pr_commits_count_delete
AFTER DELETE ON pull_request_commits
FOR EACH ROW BEGIN
    UPDATE pull_requests SET commit_count = commit_count - 1 WHERE id = OLD.pull_request_id;
END;

CREATE TRIGGER trg_review_comments_count_insert
AFTER INSERT ON review_comments
FOR EACH ROW BEGIN
    UPDATE pull_requests SET review_comment_count = review_comment_count + 1 WHERE id = NEW.pull_request_id;
END;

CREATE TRIGGER trg_review_comments_count_delete
AFTER DELETE ON review_comments
FOR EACH ROW BEGIN
    UPDATE pull_requests SET review_comment_count = review_comment_count - 1 WHERE id = OLD.pull_request_id;
END;

`
