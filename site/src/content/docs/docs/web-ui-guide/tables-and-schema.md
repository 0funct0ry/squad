---
title: Tables & Schema
description: Browsing tables, rows, and schema in the squad web UI.
---

## Browsing tables

The sidebar lists every table and view with a live row count. Selecting one
opens a paginated, sortable, filterable data grid. Cells render typed
values, `NULL` is visually distinct, and BLOB columns show a size and hex
preview rather than raw bytes. Pagination is server-side (`limit`/`offset`
against `GET /api/tables/:name/rows`), with a default page size of 100.

## Schema

Each table has a "Schema" tab showing:

- Columns — name, type, primary key, nullable, default value.
- Indexes, foreign keys, and triggers.
- The raw `CREATE TABLE` DDL, with a copy button.

## Database info

An "Info" panel shows the database file path, size on disk, SQLite library
version, page size, page count, encoding, journal mode, and per-table row
counts — backed by `GET /api/meta`.
