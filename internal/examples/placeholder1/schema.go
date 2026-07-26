// Package placeholder1 is scaffolding-only content for M6f: it exists purely
// to exercise the examples registry, embed, flag gate, and GUI/CLI surfaces
// end-to-end. It is expected to be replaced/supplemented once the real 40
// data-model schemas land as internal/examples/<slug>/schema.go files.
package placeholder1

// Schema is a minimal blog data model: authors, posts, and comments.
const Schema = `CREATE TABLE authors (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL
);

CREATE TABLE posts (
    id INTEGER PRIMARY KEY,
    author_id INTEGER NOT NULL REFERENCES authors(id),
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE comments (
    id INTEGER PRIMARY KEY,
    post_id INTEGER NOT NULL REFERENCES posts(id),
    author_id INTEGER NOT NULL REFERENCES authors(id),
    body TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
`
