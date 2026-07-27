---
title: squad cli
description: An interactive terminal REPL for a SQLite database, with no web server.
---

```
squad cli <db> [SQL]
```

`squad cli` opens a SQLite database and starts a readline-based REPL that
behaves like the stock `sqlite3` shell. It starts no HTTP server and embeds
no web UI, so it ignores every server-only flag (`--addr`, `--port`,
`--rest`, `--open`, `--token` in the sense of gating HTTP — there is no HTTP
listener to gate).

If a second positional argument is given, it's run as inline SQL instead of
starting an interactive session; otherwise squad reads from stdin as a
script when stdin isn't a terminal, or drops into the interactive REPL when
it is.

## Flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--write` | `-w` | `false` | Enable mutations (DDL, DML, write operations). |
| `--read-only-pragma` | `-R` | `true` | Open SQLite with `mode=ro` when not `--write`. |
| `--log-level` | `-l` | `info` | Log level: `debug`/`info`/`warn`/`error`. |

`squad cli` also registers the REST flags (`--rest`, `--rest-port`,
`--rest-bind-addr`) from the shared `restFlags` struct — these are consulted
by the `.listener start`/`.rest` dot-commands inside the REPL, not by any
HTTP server started at launch (`squad cli` never starts one on its own).

## Behavior

- Shares the same DB-open path and safety model as the root command:
  read-only by default, mutations gated behind `--write`.
- Interactive sessions get readline-style line editing and history.
- Exit with `.quit`, `.exit`, or Ctrl-D.

## Example

```bash
squad cli ./app.db "select count(*) from users;"
```

## In the terminal

Real `squad cli` sessions, recorded with [VHS](https://github.com/charmbracelet/vhs)
against a copy of `examples/ecommerce.db` — nothing staged or hand-edited.

### Browsing & querying

`.tables`, `.schema`, a `SELECT`, and switching `.mode` to `markdown`:

![Terminal recording: browsing tables, viewing schema, running a query, and switching output mode to markdown](/squad/casts/cli-basics.gif)

### Power-user dot-commands

[`.timer`](/docs/cli-reference/dot-commands/#timer),
[`.explain`](/docs/cli-reference/dot-commands/#explain--plan),
[`.grep`](/docs/cli-reference/dot-commands/#grep) on the last result set, and
[`.constraints`](/docs/cli-reference/dot-commands/#constraints):

![Terminal recording: using .timer, .explain, .grep, and .constraints](/squad/casts/cli-dotcommands.gif)

### Templated fake data

[`.echo`](/docs/cli-reference/dot-commands/#echo) with
[`{{ }}` template functions](/docs/cli-reference/template-functions/),
[`.seed`](/docs/cli-reference/dot-commands/#seed), and
[`.repeat`](/docs/cli-reference/dot-commands/#repeat):

![Terminal recording: using .echo with template functions, .seed, and .repeat](/squad/casts/cli-templates.gif)

## Learn more

- [Dot-command reference](/docs/cli-reference/dot-commands/) — every
  `.`-prefixed builtin, with usage examples.
- [Template function reference](/docs/cli-reference/template-functions/) —
  every function callable inside `{{ }}` blocks, with usage examples.
