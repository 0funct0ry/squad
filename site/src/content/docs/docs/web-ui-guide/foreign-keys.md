---
title: Foreign Keys
description: How squad surfaces foreign keys in the schema and seed generators.
---

## In the schema tab

A table's Schema tab lists its foreign keys alongside its columns, indexes,
and triggers, sourced from `GET /api/tables/:name/schema`.

## In seed data

Foreign-key-aware seeding is handled through the generator system rather
than a dedicated FK UI:

- `enumFromColumn` samples real, already-existing values from another
  table/column — the natural choice for a column that references a parent
  table's primary key.
- `dependentOneOf` and other cross-column generators can key off sibling
  columns within the same row to keep foreign-key-adjacent data internally
  consistent (for example, deriving a status transition log from a foreign
  key's target state).

See [Seed Data](/squad/docs/web-ui-guide/seed-data/) for the full generator
catalogue.
