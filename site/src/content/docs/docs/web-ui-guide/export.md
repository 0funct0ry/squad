---
title: Export
description: Exporting tables and query results to CSV, JSON, or SQL.
---

## Exporting a table

`GET /api/tables/:name/export?format=csv|json|sql` streams a full table
export so large tables don't buffer fully in memory:

- `csv` / `json` — the table's rows.
- `sql` — `INSERT` statements (with an optional `CREATE TABLE` prefix).

Pass `?filtered=true` to respect whatever filter is currently applied in
the data grid.

## Exporting a query result

`POST /api/export/query?format=csv|json` exports the result set of an
ad-hoc query instead of a whole table, using the same body shape as
`POST /api/query`.

Both export paths are available in read-only mode — exporting doesn't
mutate anything.
