---
title: Dot-command reference
description: Every dot-prefixed builtin command available inside the squad cli shell, with usage examples.
---

Inside an interactive `squad cli` session, any line starting with `.` is a
**builtin command**, not SQL — it's never sent to SQLite. Run `.help` at any
time to see the built-in summary; this page expands on every command with
usage examples for common scenarios.

Builtins fall into two generations: a **baseline set** compatible with the
stock `sqlite3` shell, and a **power-user set** (20 commands) added on top
of it. Both participate in tab-completion and in the read-only/`--write`
safety model — commands that mutate the database (`.import`, `.clone`,
`.seed`, some `.rest` flags) are rejected with a `READ_ONLY`-style error
unless `squad cli` was started with `--write`.

:::note
`.plan` is an alias for `.explain`, and `.sh` is an alias for `.shell`. Both
aliases work when typed, but only the canonical name (`.explain`, `.shell`)
appears in tab-completion.
:::

## Baseline commands

### `.help`

Prints the full builtin command summary. No arguments.

```
sqlite> .help
```

### `.tables`

List table and view names, optionally filtered by a glob pattern.

```
sqlite> .tables
```
```
sqlite> .tables user*
```
```
sqlite> .tables *_log
```

### `.schema`

Show `CREATE` statements for matching tables, or (with `-t`) render their
columns as a table instead.

```
sqlite> .schema
```
```
sqlite> .schema orders
```
Show every table whose name starts with `order`:
```
sqlite> .schema order*
```
Render `orders`' columns (`name,type,notnull,dflt_value,pk`) instead of its DDL:
```
sqlite> .schema -t orders
```

### `.indexes`

List index names, optionally scoped to one table.

```
sqlite> .indexes
```
```
sqlite> .indexes orders
```

### `.databases`

List attached databases (runs `PRAGMA database_list` under the hood).

```
sqlite> .databases
```

### `.mode`

Set the output rendering mode. Valid modes: `ascii box column csv json list
markdown table tabs`.

```
sqlite> .mode column
```
Switch to CSV for piping into another tool:
```
sqlite> .mode csv
sqlite> .once out.csv
sqlite> select * from orders;
```
Switch to JSON for scripting:
```
sqlite> .mode json
sqlite> select id, total from orders limit 5;
```

### `.headers`

Turn column headers on or off (only meaningful in modes that print headers,
like `column` or `csv`).

```
sqlite> .headers on
sqlite> .headers off
```

### `.nullvalue`

Set the text rendered in place of SQL `NULL`.

```
sqlite> .nullvalue NULL
```
Make NULLs visually obvious in a `column`-mode report:
```
sqlite> .nullvalue "<null>"
sqlite> select shipped_at from orders where shipped_at is null limit 5;
```

### `.output`

Redirect all subsequent output to a file, or back to the terminal.

```
sqlite> .output report.txt
sqlite> select * from orders;
sqlite> .output
```

### `.once`

Like `.output`, but only for the *next* statement — output reverts to the
terminal automatically afterward. Useful for one-off exports without
forgetting to reset `.output`.

```
sqlite> .once monthly_totals.csv
sqlite> .mode csv
sqlite> select strftime('%Y-%m', created_at) as month, sum(total) from orders group by 1;
```

### `.import`

Import CSV data from a file into an existing table. Requires `--write`.
The first line of the file is treated as the header row; values are split
on plain commas (no quote-escaping), so avoid embedded commas in fields.

```
sqlite> .import new_customers.csv customers
```

### `.dump`

Dump matching tables as a self-contained SQL script (DDL + `INSERT`s wrapped
in a transaction) — useful for backups or migrating data between databases.

```
sqlite> .dump
```
Dump just one table:
```
sqlite> .dump orders
```
Dump to a file via `.output`:
```
sqlite> .output backup.sql
sqlite> .dump
sqlite> .output
```

### `.read`

Execute SQL statements (and dot-commands) from a file, as if they'd been
typed at the prompt.

```
sqlite> .read setup.sql
```

### `.templates`

List every function callable inside `{{ }}` template blocks (see
[Template function reference](/docs/cli-reference/template-functions/) for
full detail on each one).

```
sqlite> .templates
```

### `.history`

List this session's command history, or re-run one entry by its number.

```
sqlite> .history
```
Re-run history entry 12:
```
sqlite> .history 12
```

### `.echo`

Expand `{{ }}` template functions in TEXT and print the result — without
ever sending it to SQLite. Handy for previewing what a generated value will
look like before using it in a real `INSERT`.

```
sqlite> .echo {{email}}
```
```
sqlite> .echo {{firstName}} {{lastName}} <{{email}}>
```

### `.prompt`

Show or set the prompt / continuation-prompt templates. Supports `{db}`
(expands to the current database name) and color tags
`{red}/{green}/{yellow}/{blue}/{magenta}/{cyan}/{bold}/{dim}/{reset}`.

Show current prompts:
```
sqlite> .prompt
```
Set a colored prompt that shows the current db name:
```
sqlite> .prompt {cyan}{db}{reset}> 
```
Set the continuation prompt shown on multi-line statements:
```
sqlite> .prompt continuation {dim}...{reset} 
```

### `.quit` / `.exit`

Exit the shell. Equivalent to Ctrl-D.

```
sqlite> .quit
```

## Power-user commands (M10a)

### `.edit`

Open `$EDITOR` (falling back to `vi`) seeded from either a history entry or
the OS clipboard, then load the edited text as the next statement to run.
Interactive sessions only.

Edit and re-run history entry 4:
```
sqlite> .edit -h 4
```
Paste a long query from the clipboard into `$EDITOR` before running it:
```
sqlite> .edit -c
```

### `.save`

Run a `SELECT`-shaped QUERY and write its rendered output to FILE, using
the current `.mode`/`.headers`/`.nullvalue` — independent of `.output`/
`.once`. Rejects write-shaped queries.

```
sqlite> .save top_customers.txt "select name, sum(total) as spent from orders join customers using(customer_id) group by name order by spent desc limit 10;"
```
Save as CSV regardless of the shell's current mode by switching mode first:
```
sqlite> .mode csv
sqlite> .save export.csv "select * from products;"
```

### `.grep`

Filter the *last* result set's rows (already fetched, no re-query) by a
substring or regex across all columns, and re-render the matches in the
current mode.

```
sqlite> select * from customers;
sqlite> .grep gmail.com
```
Use a regex instead of a plain substring:
```
sqlite> select * from products;
sqlite> .grep -r '^SKU-2024-\d+$'
```

### `.rest`

Configure a table's exposure via the in-process REST listener, without
starting it. `--r` (or no flag) exposes read-only; `--rw` adds create/
update (needs `--write`); `--rwd` also adds delete (needs `--write`).

Expose `products` read-only:
```
sqlite> .rest products
```
Expose `orders` for read + create/update (requires `squad cli --write`):
```
sqlite> .rest --rw orders
```
Expose `carts` fully, including delete:
```
sqlite> .rest --rwd carts
```

### `.listener`

Start or stop the in-process REST listener, bound to the `--rest-port`/
`--rest-bind-addr` flags `squad cli` was started with. Tables must already
be configured via `.rest` before they're reachable.

```
sqlite> .listener start
```
```
sqlite> .listener stop
```

### `.token`

Get or set a stored bearer token value for this session. Not yet enforced
by the REST listener — bookkeeping only, for now.

Show the current token:
```
sqlite> .token
```
Set a token to remember for later use:
```
sqlite> .token my-dev-token-123
```

### `.timer`

Print how long each statement took to run, immediately after it runs.

```
sqlite> .timer on
sqlite> select count(*) from orders;
sqlite> .timer off
```

### `.stats`

Print row-count / duration / schema-change stats after each statement —
more detail than `.timer` alone.

```
sqlite> .stats on
sqlite> update orders set status = 'shipped' where id = 42;
sqlite> .stats off
```

### `.explain` / `.plan`

Run `EXPLAIN QUERY PLAN` against QUERY (auto-prepended if you didn't type
it yourself) and render the plan as an indented tree — useful for spotting
missing indexes.

```
sqlite> .explain select * from orders where customer_id = 7;
```
Already-prefixed queries work too:
```
sqlite> .explain explain query plan select * from orders o join customers c on o.customer_id = c.id;
```
`.plan` is identical:
```
sqlite> .plan select * from products where sku = 'SKU-1';
```

### `.bookmark` / `.bookmarks`

Save or restore the current `Mode`/`Headers`/`NullValue`/`Prompt`/
`OutputFile` combination under a name, persisted to `~/.squad_bookmarks`.
Default verb is `save`, default name is `default`.

Save your current display setup under a name:
```
sqlite> .mode markdown
sqlite> .headers on
sqlite> .bookmark save reporting
```
Switch back to it later, even in a different session:
```
sqlite> .bookmark load reporting
```
List every saved bookmark:
```
sqlite> .bookmarks
```

### `.shell` / `.sh`

Run a shell command via `$SHELL -c` (falling back to `/bin/sh`), inheriting
stdio — handy for peeking at a file or piping `.dump` output through
`grep` without leaving the REPL.

```
sqlite> .shell ls -la *.db
```
```
sqlite> .sh date
```

### `.watch`

Re-run QUERY every SECONDS (a float), clearing the screen each tick, until
Ctrl-C. Template functions inside QUERY are re-expanded on every tick, so
this also works as a lightweight fake-traffic generator against a live
dashboard.

Poll a row count every 2 seconds:
```
sqlite> .watch 2 "select count(*) from orders;"
```
Watch the newest rows appear as they're inserted elsewhere:
```
sqlite> .watch 1 "select * from events order by id desc limit 5;"
```

### `.open`

Close the current database connection and open a different one, keeping
the same `--write`/read-only-pragma mode.

```
sqlite> .open ../other_project/app.db
```
Switch to an in-memory scratch database:
```
sqlite> .open :memory:
```

### `.backup`

Back up the current database to FILE via SQLite's `VACUUM INTO`. Works even
in read-only mode. Errors if FILE already exists (never silently
overwrites).

```
sqlite> .backup ./backups/app-2026-07-27.db
```

### `.clone`

Recreate a table's exact schema under a new name. Requires `--write`.
Add `--data` to also copy every row.

Clone just the schema, for a fresh empty staging table:
```
sqlite> .clone orders orders_staging
```
Clone schema and data together, e.g. before a risky migration:
```
sqlite> .clone orders orders_backup --data
```

### `.seed`

Insert N fake rows into TABLE, using the same generator registry as the web
UI's Seed tab. Requires `--write`. Retries on unique-constraint violations.

```
sqlite> .seed customers 50
```
Seed a larger batch for load testing:
```
sqlite> .seed events 5000
```

### `.diff`

Compare two tables' columns (name/type/notnull/pk) and print a diff-style
summary — useful for spotting schema drift between, say, a staging and
production copy of the same table.

```
sqlite> .diff orders orders_staging
```

### `.constraints`

Print a table's `PRIMARY KEY`, `FOREIGN KEY`, `NOT NULL`, `UNIQUE`, and
`CHECK` constraints in one readable summary.

```
sqlite> .constraints orders
```

### `.size` / `.stat db`

Print database file/meta info: path, mode, SQLite version, file size, page
size/count, encoding, journal mode, and table/view counts.

```
sqlite> .size
```
`.stat` requires the literal argument `db`:
```
sqlite> .stat db
```

### `.repeat`

Run QUERY N times in a row, re-expanding `{{ }}` template functions fresh
on every iteration — the simplest way to bulk-insert generated rows one
statement at a time (compare with `.seed` for a single-table shortcut).

```
sqlite> .repeat 20 "insert into customers (name, email) values ('{{name}}', '{{email}}');"
```
