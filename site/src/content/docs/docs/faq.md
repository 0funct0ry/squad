---
title: FAQ / Troubleshooting
description: Frequently asked questions about squad.
---

### Is there a `squad open` subcommand?

No. The root command itself opens a database and starts the server:
`squad <path/to.db>`. There's also no `squad serve` — see the
[Root Command](/squad/docs/cli-reference/root-command/) page.

### Why does squad say my database is read-only?

Because it is, by default. Pass `--write` to enable mutations. See
[Concepts](/squad/docs/getting-started/concepts/) and the
[Security Model](/squad/docs/security-model/).

### I ran a write statement in the SQL editor and got a `READ_ONLY` error. Why?

`POST /api/query` classifies every submitted statement before running it.
In read-only mode (no `--write`), only `SELECT`/`EXPLAIN`/read `PRAGMA`
statements are permitted — anything else is rejected with the
`READ_ONLY` error code rather than being sent to SQLite.

### How do I expose an endpoint over the network?

Pass `--addr 0.0.0.0` (and, for the REST listener, `--rest-bind-addr
0.0.0.0`). squad prints a warning when you do this and recommends also
setting `--token`. This is an explicit opt-in — the default is
loopback-only.

### Does `--token` protect the REST endpoints too?

No. `--token` only gates `/api/*`. `/rest/*` is a deliberately separate
trust boundary and is never gated by `--token` — see
[Auto-REST](/squad/docs/rest-api/auto-rest/).

### Can I generate code (models/clients) from my schema?

Not currently — code generation is explicitly deferred in favor of
existing per-language tools like sqlc, sqlx-codegen, and sqlacodegen.

### Does squad support Postgres or MySQL?

No, v1 is SQLite-only.

### Where do sandbox database files live?

In `--dir` if given (persisted across runs), otherwise a fresh OS temp
directory that's removed on graceful shutdown. See
[squad sandbox](/squad/docs/cli-reference/sandbox/).

### How do I try squad without a real database file?

Run `squad sandbox` and upload or create a database from the browser, or
pass `--examples` to the root command / `squad sandbox` to enable the
canned example data-model library.
