---
title: Seed Data
description: Generating fake rows to populate a table, in write mode.
---

Seeding is available only in `--write` mode.

## Workflow

1. Choose a target table.
2. For each column, pick a generator — auto-suggested from the column's
   name and type via `GET /api/tables/:name/seed/plan`, which also returns
   the full list of available generators (the single source of truth used
   by the generator picker UI).
3. Set a row count and optionally preview a sample.
4. Insert via `POST /api/tables/:name/seed` with a body like
   `{ "count": 100, "dryRun": false, "columns": { "email": { "generator": "email" } } }`.
   With `dryRun: true`, rows are generated and returned but never inserted.

## Generator catalogue

The generator registry is backed by `gofakeit` and spans person, geo/address,
datetime, numeric, internet, payment/finance, company, color, text/grammar,
food, product, identifiers, and novelty/theme-pack generators, plus:

- **Media/binary generators** that produce real, structurally valid file
  bytes for BLOB columns: `qrCode`/`barcode` (scannable images),
  `profilePicture` (a procedurally-composited avatar), `svgImage`/`icon`
  (real vector shapes), and `soundData` (a synthesized WAV file).
- **Custom-list & conditional generators**: user-typed value lists
  (`oneOf`, `weightedOneOf`, `incrementalEnum`, `regexEnum`), generators
  that read already-generated sibling columns (`dependentOneOf`,
  `customDateSequence`, `statusTransitionLog`, `checksumOfColumns`,
  `slugFromColumn`, `jsonTemplate`), `enumFromColumn` (samples real values
  from another table/column), and `nullWithProbability` (wraps any other
  generator to occasionally substitute `NULL`).
- A **formula** generator whose expression is parsed as a restricted Go
  expression: bare identifiers resolve to sibling column values, `+ - * /`
  do ordinary arithmetic (`+` concatenates strings), and a whitelisted set
  of functions is available — string (`upper`, `lower`, `concat`, `trim`,
  `len`, `capitalize`, `kebabCase`, `camelCase`), encoding (`hex`,
  `base32`, `base64`), crypto digests (`sha1`, `md5`, `sha256`, `sha512`),
  and math (`abs`, `round`, `floor`, `ceil`, `min`, `max`, `pow`, `mod`).
  Example: `round(price * qty)`.

With 226 generators, the Generator Picker groups them by category with
live per-generator sample previews and a type-compatibility filter, rather
than a flat dropdown. `GET /api/seed/generators/:name/sample` powers the
lightweight single-value preview.

## Learn more

- [Seeding data from the UI: a walkthrough](/docs/web-ui-guide/seed-data-walkthrough/) —
  a screenshot-driven walkthrough from opening the Seed tab to inserted rows.
- [Generator reference](/docs/web-ui-guide/generator-reference/) — every
  generator, grouped by category, with options and usage examples.
