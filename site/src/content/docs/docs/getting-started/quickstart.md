---
title: Quickstart
description: Open a database and start browsing it with squad.
---

There is no `squad open` subcommand — the root command itself opens a
database and starts the server:

```bash
squad ./app.db
```

This prints something like:

```
squad v1.0.0
  database : ./app.db  (read-only)
  address  : http://127.0.0.1:7071
  press Ctrl+C to stop
```

By default squad auto-opens your browser at the printed address
(`http://127.0.0.1:7071`). From there you can browse tables, view schema,
and run read-only SQL.

## Enabling writes

Mutations (DDL, DML, the table editor, seeding) are disabled unless you pass
`--write`:

```bash
squad ./app.db --write
```

## Trying it without a database file

If you don't have a specific file to point at yet, `squad sandbox` starts
the same UI with no fixed database — you upload, create, and switch between
SQLite files entirely from the browser:

```bash
squad sandbox
```

Every database opened in sandbox mode is always read-write.

## Exploring from the terminal instead

`squad cli <db>` opens an interactive REPL (behaving like the stock
`sqlite3` shell) with no HTTP server at all:

```bash
squad cli ./app.db
```

See the [CLI Reference](/squad/docs/cli-reference/global-flags/) for every
flag on each of these commands.
