---
title: Template function reference
description: Every function callable inside {{ }} template blocks in the squad cli shell, with example usages.
---

Any statement typed at the `squad cli` prompt that contains `{{ }}` is run
through a template preprocessor before being sent to SQLite (or, for
`.echo`, before being printed). This lets you generate realistic fake
values inline, without leaving the shell — the same generator registry
that powers the web UI's Seed tab.

Run `.templates` at any time to list every function with its quoting hint
straight from the running binary — this page is the same information with
worked examples.

## The quoting rule

Template functions return **raw values, not pre-quoted SQL literals**. Each
function falls into one of three quoting categories, and getting this
wrong is the single most common mistake when hand-writing `{{ }}` SQL:

- **add quotes** — the function returns a string; you must wrap the call
  yourself in `'...'`.
  ```
  sqlite> insert into users (name, email) values ('{{name}}', '{{firstName}}@{{lastName}}.in');
  ```
- **bare** — the function returns a number or boolean; write it with no
  quotes at all.
  ```
  sqlite> insert into products (price, in_stock) values ({{price}}, {{bool}});
  ```
- **self-quoted** — the function returns a BLOB and already emits its own
  `X'...'` delimiters; do **not** wrap it in quotes.
  ```
  sqlite> insert into files (checksum) values ({{bytes 16}});
  ```

Preview any expansion before running it for real with `.echo`:
```
sqlite> .echo {{email}}
```

## Generators

Generators are exposed by a flat, undotted name (e.g. `{{email}}`, not
`{{seed.email}}`). Some accept positional arguments that map onto that
generator's options — see each entry below.

| Function | Quoting | Description |
|---|---|---|
| `email` | add quotes | Random email address. |
| `firstName` | add quotes | Random first name. |
| `lastName` | add quotes | Random last name. |
| `name` | add quotes | Random full name. |
| `username` | add quotes | Random username. |
| `uuid` | add quotes | Random UUID (v4). |
| `datetime` | add quotes | Random timestamp. |
| `price` | bare | Random decimal price. |
| `url` | add quotes | Random URL. |
| `phone` | add quotes | Random phone number. |
| `bool` | bare | Random `0`/`1`. |
| `sentence` | add quotes | Random sentence. |
| `word` | add quotes | Random single word. |
| `paragraph` | add quotes | Random paragraph. |
| `int` | bare | Random integer. |
| `float` | bare | Random decimal number. |
| `company` | add quotes | Random company name. |
| `address` | add quotes | Random street address. |
| `city` | add quotes | Random city name. |
| `country` | add quotes | Random country name. |
| `zipCode` | add quotes | Random postal code. |
| `ipv4` | add quotes | Random IPv4 address. |
| `bytes` | self-quoted | Random binary blob. |
| `enumFromColumn` | add quotes | Random existing value picked live from another table's column. |

### No-option generators

Most generators take no arguments — call them bare:

```
sqlite> insert into customers (name, email, phone) values ('{{name}}', '{{email}}', '{{phone}}');
```
```
sqlite> insert into companies (name, city, country) values ('{{company}}', '{{city}}', '{{country}}');
```
```
sqlite> insert into sessions (id) values ('{{uuid}}');
```

### `datetime`

Random timestamp.

```
sqlite> insert into events (occurred_at) values ('{{datetime}}');
```

### `price`

Random decimal price. Optional `min`/`max` (positional).

```
sqlite> insert into products (price) values ({{price}});
```
Constrain the range for a "budget" catalog:
```
sqlite> insert into products (price) values ({{price 1 20}});
```

### `sentence`

Random sentence. Optional word-count.

```
sqlite> insert into reviews (body) values ('{{sentence}}');
```
Force a shorter sentence:
```
sqlite> insert into reviews (body) values ('{{sentence 6}}');
```

### `paragraph`

Random paragraph. Optional sentence-count.

```
sqlite> insert into posts (body) values ('{{paragraph}}');
```
A longer paragraph for a "long-form" test fixture:
```
sqlite> insert into posts (body) values ('{{paragraph 8}}');
```

### `int` / `float`

Random numbers. Optional `min`/`max`.

```
sqlite> insert into inventory (quantity) values ({{int}});
```
Constrain to a realistic range:
```
sqlite> insert into inventory (quantity) values ({{int 0 500}});
```
```
sqlite> insert into measurements (reading) values ({{float 0 1}});
```

### `bytes`

Random BLOB of a given length (bytes). Already self-quoted — never wrap it.

```
sqlite> insert into files (checksum) values ({{bytes 32}});
```

### `enumFromColumn`

Pick a random value that already exists in another table's column — the
easiest way to generate realistic foreign-key-shaped values without a real
foreign key or a lookup join.

```
sqlite> insert into orders (customer_id) values ((select id from customers order by random() limit 1));
```
Or, using the generator directly against an existing column of raw values:
```
sqlite> insert into orders (status) values ('{{enumFromColumn "orders" "status"}}');
```

### Combining generators into one row

```
sqlite> insert into customers (name, email, phone, city, country)
   ...> values ('{{name}}', '{{email}}', '{{phone}}', '{{city}}', '{{country}}');
```

Bulk-generate several rows with `.repeat` (re-expands `{{ }}` fresh each
time, so every row gets different fake data):
```
sqlite> .repeat 25 "insert into customers (name, email) values ('{{name}}', '{{email}}');"
```

## Formula functions

A whitelist of general-purpose string/encoding/crypto/math helpers, useful
for post-processing a generator's output or deriving one column from
another inside the same `{{ }}` block.

| Function | Quoting | Description |
|---|---|---|
| `upper(s)` | add quotes | Uppercase a string. |
| `lower(s)` | add quotes | Lowercase a string. |
| `concat(a, b, ...)` | add quotes | Concatenate strings. |
| `trim(s)` | add quotes | Trim surrounding whitespace. |
| `len(s)` | bare | Rune length of a string. |
| `capitalize(s)` | add quotes | Capitalize the first letter. |
| `kebabCase(s)` | add quotes | Convert to `kebab-case`. |
| `camelCase(s)` | add quotes | Convert to `camelCase`. |
| `hex(s)` | add quotes | Hex-encode a string. |
| `base32(s)` | add quotes | Base32-encode a string. |
| `base64(s)` | add quotes | Base64-encode a string. |
| `sha1(s)` | add quotes | SHA-1 hex digest. |
| `md5(s)` | add quotes | MD5 hex digest. |
| `sha256(s)` | add quotes | SHA-256 hex digest. |
| `sha512(s)` | add quotes | SHA-512 hex digest. |
| `abs(x)` | bare | Absolute value. |
| `round(x)` | bare | Round to nearest integer. |
| `floor(x)` | bare | Round down. |
| `ceil(x)` | bare | Round up. |
| `min(a, b, ...)` | bare | Minimum of the arguments. |
| `max(a, b, ...)` | bare | Maximum of the arguments. |
| `pow(x, y)` | bare | `x` raised to the power `y`. |
| `mod(x, y)` | bare | `x` modulo `y`. |

### String transforms

Derive a username from a generated name:
```
sqlite> insert into users (name, username) values ('{{name}}', '{{lower "{{name}}"}}');
```
Generate a URL-friendly slug from a random word:
```
sqlite> insert into posts (title, slug) values ('{{word}}', '{{kebabCase "{{word}}"}}');
```

### Hashing

Store a hashed placeholder password for a fixture user:
```
sqlite> insert into users (email, password_hash) values ('{{email}}', '{{sha256 "dev-password"}}');
```
Derive a short, stable-looking id from an email:
```
sqlite> insert into avatars (user_email, cache_key) values ('{{email}}', '{{md5 "{{email}}"}}');
```

### Encoding

```
sqlite> insert into tokens (value) values ('{{base64 "{{uuid}}"}}');
```

### Math

Clamp a generated float into a fixed range using `min`/`max` together:
```
sqlite> insert into scores (value) values ({{max 0 (min 100 {{int 0 150}})}});
```
Round a generated price to a whole number:
```
sqlite> insert into products (price) values ({{round {{price 1 1000}}}});
```

## Previewing before you commit

Since template functions are re-evaluated fresh every time a statement
runs, use `.echo` first when a template is complex enough that you want to
see the exact expansion before it touches the database:

```
sqlite> .echo insert into users (name, email) values ('{{name}}', '{{email}}');
```

See also [`.templates`](/docs/cli-reference/dot-commands/#templates) for
listing this same catalog live from the running binary, and
[`.seed`](/docs/cli-reference/dot-commands/#seed) /
[`.repeat`](/docs/cli-reference/dot-commands/#repeat) for the two most
common ways to use these functions to generate many rows at once.
