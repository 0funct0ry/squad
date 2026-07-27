---
title: SQL Editor
description: Running arbitrary SQL against your database.
---

The SQL editor (CodeMirror 6, with SQL syntax highlighting) lets you run
arbitrary SQL against the open database via `POST /api/query`.

- Run with the run button or Ctrl/Cmd+Enter.
- Results render in a grid alongside execution time and rows-affected.
- Query history is kept in-memory for the session (not persisted).

## Read-only mode

In read-only mode (the default, without `--write`), only `SELECT`,
`EXPLAIN`, and read `PRAGMA` statements are allowed. A statement classifier
blocks any write statement before it reaches SQLite and reports a
`READ_ONLY` error — the editor surfaces this with a clear message rather
than a raw driver error.

## DDL

`CREATE`/`ALTER`/`DROP` statements can also be run from the SQL editor (or
a dedicated DDL affordance), but only in `--write` mode. SQLite errors are
surfaced verbatim.
