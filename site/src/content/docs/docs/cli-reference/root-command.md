---
title: "Root command: squad <db>"
description: The primary command that opens a database and starts the server.
---

```
squad <path/to.db> [flags]
```

There is no literal `squad open` subcommand — the root command itself opens
the given database and starts the web server. It takes exactly one
positional argument, the database path (or `:memory:`, or a `file:` DSN).

## Flags specific to the root command

| Flag | Short | Default | Description |
|---|---|---|---|
| `--write` | `-w` | `false` | Enable mutations (DDL, DML, write operations). |
| `--read-only-pragma` | `-R` | `true` | Open SQLite with `mode=ro` when not `--write`. |
| `--examples` | `-e` | `false` | Enable the canned example data-model library. |

It also registers every flag in [Global Flags](/squad/docs/cli-reference/global-flags/):
`--addr`, `--port`, `--open`, `--token`, `--log-level`, `--rest`,
`--rest-port`, `--rest-bind-addr`.

## Behavior

- If the path doesn't exist and `--write` is set, squad creates an empty,
  valid SQLite file there (the driver always opens with
  `SQLITE_OPEN_CREATE`) — this is expected, not an error.
- Read-only mode is active whenever `--write` is false, and the connection
  is opened with `mode=ro` when `--read-only-pragma` is also true (the
  default).
- Auto-opens the default browser 500ms after startup unless `--open=false`.
- Prints a warning if `--addr` (or, with `--rest`, `--rest-bind-addr`) is
  set to `0.0.0.0`.

## Example

```bash
squad ./app.db --write --rest --port 8080
```

Opens `./app.db` read-write, unlocks the auto-REST capability (still
requires starting the listener from the REST tab), and serves the main UI
on port 8080 instead of the default 7071.
