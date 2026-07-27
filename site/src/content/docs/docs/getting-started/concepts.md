---
title: Concepts
description: Read-only vs write mode, the JSON envelope, and loopback binding.
---

A handful of ideas recur throughout squad's design. Understanding them makes
the rest of the docs, and the CLI flags, easier to reason about.

## Read-only vs write mode

squad opens its SQLite connection once, at process start, in either
read-only or read-write mode — this is fixed for the lifetime of the
process, not decided per request.

- **Without `--write`** (the default): the connection is opened read-only
  (when `--read-only-pragma` is also true, the default, SQLite is opened
  with `mode=ro`), and all mutating API routes return `403`. A statement
  classifier inspects SQL submitted to `POST /api/query` and rejects any
  write statement with a `READ_ONLY` error code before it reaches the
  database.
- **With `--write`**: mutations are permitted — DDL, DML, the table editor,
  and fake-data seeding all become available, and the UI shows a "WRITE
  MODE" badge.

`squad sandbox` databases are always read-write; there is no
`--write`/`--read-only-pragma` flag on that subcommand.

## The JSON envelope

Every `/api/*` response uses one consistent shape:

```json
{ "ok": true, "data": { ... } }
```

or, on failure:

```json
{ "ok": false, "error": { "code": "READ_ONLY", "message": "..." } }
```

Note that `/rest/*` (the opt-in auto-REST capability) deliberately does
**not** use this envelope — see [Auto-REST](/squad/docs/rest-api/auto-rest/).

## Loopback binding

squad binds to `127.0.0.1` by default — reachable only from the machine
it's running on. Binding to all interfaces (`--addr 0.0.0.0`) is an
explicit opt-in; doing so prints a warning recommending you also set
`--token`. The same applies independently to the REST listener's
`--rest-bind-addr`.
