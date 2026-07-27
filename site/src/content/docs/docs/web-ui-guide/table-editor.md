---
title: Table Editor
description: Creating and altering tables visually, in write mode.
---

The table editor is available only when squad was started with `--write`.

## Creating a table

Add columns one at a time with a name, type affinity, and
primary-key/not-null/unique/default flags, then submit — this calls
`POST /api/tables` with a structured column definition.

## Altering a table

Rename the table, add a column, rename a column, or drop a column, via
`PATCH /api/tables/:name`. Column drops are emulated with SQLite's
rebuild pattern (`create new → copy → drop → rename`) on SQLite versions
that lack native `DROP COLUMN`.

## Dropping a table

`DELETE /api/tables/:name` drops the table entirely.

## Row CRUD

Individual rows are inserted, updated, and deleted through
`POST`/`PATCH`/`DELETE /api/tables/:name/rows`, using parameterized
queries (unlike the raw SQL editor, which is intentionally unparameterized
for the trusted local user running it).
