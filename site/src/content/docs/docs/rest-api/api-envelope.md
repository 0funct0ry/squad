---
title: API Envelope
description: The /api/* JSON envelope and error codes.
---

All application endpoints live under `/api/*` and respond with one
consistent JSON envelope:

```json
{ "ok": true, "data": { ... } }
```

or, on failure:

```json
{ "ok": false, "error": { "code": "READ_ONLY", "message": "..." } }
```

## Key endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/api/meta` | DB name, mode (`ro`/`rw`), SQLite version, page size, size on disk, table & view counts. |
| GET | `/api/tables` | List tables & views with row counts. |
| GET | `/api/tables/:name/schema` | Columns, indexes, foreign keys, triggers, and the `CREATE` DDL. |
| GET | `/api/tables/:name/rows` | Paginated rows (`limit`, `offset`, `orderBy`, `dir`, `filter`). |
| POST | `/api/query` | Run arbitrary SQL. Body `{ "sql": "...", "limit": 1000 }`. |
| POST/PATCH/DELETE | `/api/tables/:name/rows` | Row CRUD (parameterized, write mode). |
| POST | `/api/ddl`, `/api/tables` | DDL / structured table creation (write mode). |
| GET | `/api/tables/:name/export` | Stream a table export (`?format=csv\|json\|sql`). |
| POST | `/api/export/query` | Export an ad-hoc query's result set. |
| GET/POST | `/api/tables/:name/seed/plan`, `/api/tables/:name/seed` | Seeding (write mode). |
| GET/POST | `/api/rest/status`, `/api/rest/start`, `/api/rest/stop` | Control the auto-REST listener — see [Auto-REST](/squad/docs/rest-api/auto-rest/). |

List endpoints return `"data": []` for an empty result, never `null` — for
example `GET /api/tables` on a freshly created, table-less database.

## Error codes

`POST /api/query` runs a statement classifier before execution: in
read-only mode, only `SELECT`/`EXPLAIN`/read `PRAGMA` statements are
allowed, and anything else is rejected with the `READ_ONLY` code shown
above rather than being sent to SQLite.
