---
title: Auto-REST
description: The opt-in /rest/* per-table REST endpoints.
---

Auto-REST is an opt-in capability, unlocked with `--rest` at launch, that
serves per-table REST endpoints on a **separate HTTP listener** — its own
port (`--rest-port`, default `7072`, bound to `--rest-bind-addr`, default
`127.0.0.1`) — distinct from the main `/api` + web UI server. This is a
deliberate, separate trust boundary: `/rest/*` is **not** gated by
`--token`, even when `--token` is set for `/api/*`.

## Unlocking vs. mounting

`--rest` at launch only unlocks the capability — it does not automatically
expose any table or start the listener. Everything else happens from the
REST tab in the UI:

- Each table has an in-memory "exposed via REST" flag (reset every process
  restart, never persisted), plus independent Create/Update/Delete
  toggles. Write toggles only take effect when the process was started
  with `--write` — the server enforces this with `403` regardless of
  stored toggle state.
- SQLite-internal tables are never offered as exposable. Views may be
  exposed but only ever get GET routes.

Control routes live on the **main** `/api` listener, so they work even
while the REST listener itself is stopped:

| Method | Path | Description |
|---|---|---|
| GET | `/api/rest/status` | Whether the REST listener is running, its bind address:port, and the per-table snapshot it's serving vs. the live configured state. |
| POST | `/api/rest/start` | Start the listener, snapshotting the current per-table flags. No-op error if `--rest` wasn't passed at launch. |
| POST | `/api/rest/stop` | Stop the listener. |

Changing a table's exposure/CRUD flags while the listener is running does
not take effect until the next `start` — the UI surfaces this with a toast
and a persistent badge.

## Routes on the REST listener

| Method | Path | Description |
|---|---|---|
| GET | `/rest/:table` | List rows. Pagination via `?limit&offset`; filtering via `?col=val`, exact-match, ANDed across params. |
| GET | `/rest/:table/:pk` | Get one row by primary/fallback key. |
| POST | `/rest/:table` | Create — only mounted if `--write` and the table's create toggle is on. |
| PATCH | `/rest/:table/:pk` | Update — only mounted if `--write` and the table's update toggle is on. |
| DELETE | `/rest/:table/:pk` | Delete — only mounted if `--write` and the table's delete toggle is on. |
| GET | `/rest/_schema` | Describes only the tables/methods in the currently running snapshot. |

**Key resolution:** a single-column primary key is used as-is. Composite
primary keys and tables with no explicit primary key fall back to SQLite's
`rowid`. A `WITHOUT ROWID` table with a composite key and no usable
single-column identity gets no per-row routes at all (list-only).

## Response shape

Unlike `/api/*`, `/rest/*` does **not** use the `{ok,data}` envelope — it
returns plain resource JSON (a bare array for list, a bare object for
get/create/update). Errors use a small dedicated shape instead:

```json
{ "error": "<code>", "message": "<text>" }
```

with standard HTTP status codes: `404` (unknown table/row), `400` (bad
body/unknown column), `403` (write attempted without `--write` or without
that table's write toggle enabled at last start).

## Sandbox mode

Under `squad sandbox`, `/rest/:table` always resolves against whichever
sandbox database is currently active. Switching the active database while
the listener is running stops it automatically, with an explanatory toast.
