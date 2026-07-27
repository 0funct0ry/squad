---
title: Security Model
description: squad's read-only-by-default trust model.
---

squad is built for one developer running it against their own local
files, not as a hosted multi-tenant service. Its safety model reflects
that.

## Read-only by default

The SQLite connection is opened once, at process start, in read-only or
read-write mode for the whole process lifetime — this is never decided
per request. Without `--write`, all mutating `/api/*` routes return `403`,
and `POST /api/query` runs every submitted statement through a classifier
that rejects writes with a `READ_ONLY` error code before they reach
SQLite. `--write` also gates the table editor and seeding.

## Loopback binding

`--addr` defaults to `127.0.0.1` — reachable only from the machine running
squad. Setting `--addr 0.0.0.0` (or any non-loopback address) is an
explicit opt-in that prints a warning recommending you also set `--token`.
The same independently applies to `--rest-bind-addr` for the auto-REST
listener.

## Bearer token

`--token` optionally gates every `/api/*` route behind a bearer token,
for the rare case of exposing squad beyond your own machine. It is off by
default. It does **not** gate `/rest/*` — the auto-REST listener is a
separate trust boundary by design (see [Auto-REST](/squad/docs/rest-api/auto-rest/)).

## Auto-REST is opt-in and off by default

`--rest` unlocks the auto-REST capability but exposes nothing by itself:
tables must be explicitly toggled "exposed" from the REST tab, and REST
writes additionally require `--write` at the process level plus that
table's own Create/Update/Delete toggle.

## Parameterized queries, except where you asked for raw SQL

Row CRUD and auto-REST always use parameterized queries. The SQL editor
(`POST /api/query`) is intentionally unparameterized raw SQL — it's
designed for the trusted local user running squad against their own
database, not for untrusted input.

## No filesystem browsing

squad exposes no filesystem-browsing endpoints. Only the database you
opened it against (or, in `squad sandbox`, the databases in its sandbox
directory) is reachable through the API.
