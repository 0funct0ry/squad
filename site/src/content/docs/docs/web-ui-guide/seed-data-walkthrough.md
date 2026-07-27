---
title: "Seeding data from the UI: a walkthrough"
description: A screenshot-driven walkthrough of the Seed tab, from plan to inserted rows.
---

This walks through seeding a table with fake data end to end, using the
actual Seed tab UI. Seeding is only available in `--write` mode — launch
`squad` with `--write` if the banner below tells you it's required.

## 1. Select a table and open the Seed tab

Pick a table from the sidebar, then click **Seed**. The tab immediately
fetches a seed plan for that table (`GET /api/tables/:name/seed/plan`) and
pre-fills a generator for every column, auto-suggested from its name and
type — no manual setup required to get a reasonable first pass.

![Seed plan for the products table, with a generator column auto-filled per row and a live sample preview next to each generator name](/squad/screenshots/seed-plan.png)

In this example (`products`):
- **`id`** is skipped automatically, with the reason shown inline:
  *"skipped — auto-assigned rowid primary key"*. Click **Override** if you
  ever need to force-generate values for a column like this anyway.
- **`sku`** got the generic `sentence` generator (no name hint matched it)
  — a good candidate to swap for something more realistic, like `regexEnum`
  with a pattern such as `SKU-[0-9]{6}`. See the [generator
  reference](/docs/web-ui-guide/generator-reference/) for the full catalog.
- **`name`** correctly picked the `name` generator from the column name.
- **`category_id`** was detected as a foreign key and got `foreignKey`,
  pointed at `categories.id` — its values will always reference a real,
  existing row in `categories`.
- **`price`** picked the `price` generator (name-heuristic match) with
  optional `Min`/`Max` fields you can fill in to constrain the range.
- **`stock`** and **`active`** both got the generic `int` generator (no
  name hint for `active` as a boolean, so it defaults to a wide integer
  range — swap it for `bool` if that's what the column actually needs).
- **`created_at`** picked `datetime`, with `From`/`To` date pickers and an
  **Only date** checkbox to strip the time component.

Every generator button also shows a live one-value sample (the small gray
`→ ...` line) so you can sanity-check the output without running anything.

## 2. Change a generator

Click any generator button to open the **generator picker** — a searchable,
category-grouped catalog of all 226 available generators.

![The generator picker modal open, showing category counts in the left sidebar (Color, Company, Cross-Column, Custom List, Date & Time, Distribution, Domain Lookup, Finance, Food, Geo, Identifier, Internet, Media, misc, Novelty, Numeric...) and a scrollable list of generator cards on the right, each with a name, description, and live sample](/squad/screenshots/seed-generator-picker.png)

- The left sidebar groups generators by category, with counts, so you can
  browse instead of guessing a name.
- Cards tagged **novelty** or **domain-lookup** (visible on `airline`,
  `animal`, `app`, `banking` here) are themed generator packs — see the
  [generator reference](/docs/web-ui-guide/generator-reference/) for what
  each theme covers.
- **"Show all types (ignore column type compatibility)"** is unchecked by
  default, so the list only shows generators compatible with the target
  column's SQLite type affinity. Check it if you deliberately want to
  cross-cast (e.g. store a generated number as TEXT).

Type in the search box to filter by name or description:

![The same generator picker filtered by the search term "date", showing dateRange, datetime, customDateSequence, futureDate, pastDate, cronExpression, durationInterval, monthString, and time as matches](/squad/screenshots/seed-generator-picker-search.png)

Search matches on generator name (exact/prefix/substring, in that priority
order) and falls back to matching aliases, group, and description — so
searching `date` surfaces both obvious matches (`dateRange`, `datetime`,
`futureDate`, `pastDate`) and less obvious ones whose description mentions
dates (`durationInterval`, `cronExpression`).

Click a card to select it and close the picker. Use the arrow keys to move
the highlight and Enter to select without touching the mouse.

## 3. Set a row count and preview before inserting

Set the **Rows** field (1–100,000, clamped client-side if you type
something outside that range), then click **Preview 5 rows** to see exactly
what would be generated — without touching the database
(`dryRun: true` under the hood).

![The seed plan for products with a preview table shown underneath the Rows/Preview/Insert controls, listing 5 generated rows with columns active, category_id, created_at, name, price, sku, stock](/squad/screenshots/seed-preview.png)

This is the moment to catch problems before they hit the database: does
`sku` actually look like a SKU? Is `price` in a sane range? Are foreign
keys (`category_id`) pointing at values that exist? Go back to step 2 and
swap generators or adjust options until the preview looks right.

## 4. Insert

Once the preview looks right, click **Insert** to actually run the insert
— all rows in a single transaction. If a `UNIQUE` constraint is violated
partway through, squad automatically regenerates just the offending column
group and retries (up to 20 times per row) rather than failing the whole
batch outright. If something still can't be resolved, the error is shown
inline below the buttons and the whole transaction is rolled back — no
partial inserts.

After a successful insert, switch to the **Data** tab to see the new rows
alongside the existing ones, or the **Info** tab to confirm the table's row
count went up by exactly what you asked for.

## Tips

- **Foreign keys and `enumFromColumn` need real data to reference.** If the
  referenced table is empty, seeding will fail with an `EMPTY_REFERENCE`
  error — seed the referenced table first (e.g. `categories` before
  `products`).
- **Cross-column generators** (`formula`, `dependentOneOf`,
  `customDateSequence`, `statusTransitionLog`, `checksumOfColumns`,
  `slugFromColumn`, `jsonTemplate`) read other columns' *already-generated*
  values for the same row — pick which sibling columns they depend on in
  their options form. squad automatically orders generation so dependencies
  run first; a circular dependency is rejected with an error before any
  rows are generated.
- **Skipped columns aren't a bug.** Autoincrement rowid primary keys are
  skipped by default because SQLite assigns them automatically — use
  **Override** only if you have a real reason to set them explicitly.
