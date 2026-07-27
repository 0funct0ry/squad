---
title: squad sandbox
description: Start squad with an ad-hoc, browser-managed set of SQLite databases.
---

```
squad sandbox [flags]
```

`squad sandbox` starts the same web UI with no fixed database. Upload,
create, and switch between one or more SQLite files entirely from the
browser instead of passing a `<db>` path on the command line. Every
database opened in sandbox mode is always read-write — there is no
`--write` or `--read-only-pragma` flag on this subcommand.

## Flags specific to sandbox

| Flag | Default | Description |
|---|---|---|
| `--dir` | fresh OS temp dir | Directory to store sandbox database files (env `SQUAD_SANDBOX_DIR`). If omitted, a temp directory is created and removed on graceful shutdown; if given explicitly, files persist and are re-scanned on the next run against the same directory. |
| `--max-upload-size` | `512` (MB) | Max upload size in MB for sandbox database files. |
| `--examples` | `false` | Enable the canned example data-model library. |

It also registers the shared flags from [Global Flags](/squad/docs/cli-reference/global-flags/):
`--addr`, `--port`, `--open`, `--token`, `--log-level`, `--rest`,
`--rest-port`, `--rest-bind-addr`.

## Auto-REST in sandbox mode

`--rest` behaves as in root mode, except `/rest/:table` always resolves
against whichever sandbox database is currently active — there is no
`/rest/:dbId/:table` form. Switching the active database while the REST
listener is running stops it automatically.

## Example

```bash
squad sandbox --dir ./my-sandbox-dbs --max-upload-size 256
```
