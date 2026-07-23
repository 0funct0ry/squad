# Example databases

Sample SQLite databases for testing and demoing `squad`. Each is a single,
self-contained `.db` file (default rollback journal — no `-wal`/`-shm`
sidecars), safe to open in squad's default **read-only** mode:

```bash
squad examples/blog.db
# or the interactive shell:
squad cli examples/blog.db
```

Together they exercise the full read path (browse / schema / rows / info),
export (CSV/JSON/SQL, including BLOBs and unicode), code generation (varied
types + nullables), and the read-only safety model. Between them they cover
tables, views, triggers, indexes (incl. unique and partial), foreign keys,
CHECK constraints, generated columns, `WITHOUT ROWID` tables, and awkwardly
quoted identifiers.

## Files

### `blog.db` — classic relational blog
Users, posts, comments, tags. Good baseline for browsing and FK navigation.

- **Tables:** `users`, `posts`, `comments`, `tags`, `post_tags` (M2M), `post_stats`
- **View:** `published_posts`
- **Trigger:** `trg_comment_ai` — upserts a denormalized comment count into `post_stats`
- **Highlights:** FK `ON DELETE CASCADE` / `SET NULL`, `CHECK (status IN (...))`,
  unique constraints, composite PK on the join table, three indexes.

### `ecommerce.db` — store catalog & orders
Products, categories, customers, orders. Money as `REAL`, aggregate view.

- **Tables:** `categories`, `products`, `customers`, `orders`, `order_items`
- **View:** `order_totals` (sums line items per order)
- **Highlights:** self-referencing FK (`categories.parent_id` → category tree),
  composite PK on `order_items`, `CHECK` constraints on price/quantity/status.

### `library.db` — books, members, loans
Authors ↔ books (many-to-many), member loans with due/return tracking.

- **Tables:** `authors`, `books`, `book_authors` (M2M), `members`, `loans`
- **View:** `open_loans` (unreturned loans)
- **Trigger:** `trg_loan_stock` — `RAISE(ABORT)` when loaning a book with 0 copies
- **Highlights:** **partial index** `idx_loans_open` (`WHERE returned_at IS NULL`),
  nullable date columns, `BEFORE INSERT` guard trigger.

### `analytics.db` — event stream (largest fixture)
~5k events across 500 sessions. Exercises pagination and BLOB rendering.

- **Tables:** `sessions` (`WITHOUT ROWID`, TEXT PK), `events`, `daily_rollup` (`WITHOUT ROWID`)
- **View:** `event_counts` (per-event aggregates)
- **Highlights:** `WITHOUT ROWID` tables, TEXT (uuid-ish) primary key, **BLOB**
  `payload` column, JSON-stored-as-TEXT `props`, composite index for sorted scans.

### `types_zoo.db` — type & identifier edge cases
Deliberately awkward schema to stress rendering, codegen, and identifier quoting.

- **Tables:** `affinities`, `measurements`, `"weird names"`, `contacts`
- **View:** `big_boxes`
- **Highlights:** all SQLite affinities (INTEGER/REAL/TEXT/BLOB/NUMERIC) plus
  NULLs, **generated columns** (`area_cm2` STORED, `ratio` VIRTUAL),
  **quoted identifiers** (table `"weird names"`; columns `"select"`,
  `"space col"`, `"from"`, `"MixedCase"`), `COLLATE NOCASE`, a unique index and a
  partial index, and unicode/emoji text values.

## Regenerating

The fixtures are produced by [`generate.py`](generate.py) (Python stdlib only,
fixed RNG seed so output is deterministic):

```bash
python3 examples/generate.py          # rewrite the .db files in place
python3 examples/generate.py /tmp/out # write them somewhere else
```
