---
title: Generator reference
description: Complete reference for all 226 seed data generators available in the Seed tab, grouped by category, with examples.
---

This is the complete reference for every generator available in the
[Seed tab](/docs/web-ui-guide/seed-data-walkthrough/)'s generator picker —
226 in total, grouped into the same categories the picker itself uses.
Most generators take no options and just produce a realistic random value
of their kind; the ones with meaningful options are documented individually
below their category table.

Every generator declares which SQLite type affinities it's compatible with
— the picker filters to compatible generators by default (uncheck **"Show
all types"** to see everything). You can preview any generator's live
output for a given column without leaving the picker.

## Person

| Generator | Description |
|---|---|
| `email` | Email address |
| `firstName` | First name |
| `lastName` | Last name |
| `name` | Full name |
| `username` | Username |
| `phone` | Phone number |
| `phoneFormatted` | Formatted phone number |
| `age` | Age in years |
| `bio` | Short biography |
| `ein` | Employer identification number |
| `ethnicity` | Ethnicity |
| `gender` | Gender |
| `hobby` | Hobby |
| `middleName` | Middle name |
| `namePrefix` | Name prefix (Mr., Mrs., ...) |
| `nameSuffix` | Name suffix (Jr., Sr., ...) |
| `socialMedia` | Social media handle |
| `ssn` | Social security number |

**`age`** — options `min` (default `18`), `max` (default `80`). Example: a
`users.age` column with a working-age skew: set `min: 21, max: 65`.

## Geo

| Generator | Description |
|---|---|
| `address` | Street address |
| `addressLine2` | Apartment/floor line |
| `city` | City name |
| `country` | Country name |
| `countryAbr` | Country abbreviation |
| `state` | State/province |
| `stateAbr` | State abbreviation |
| `street` | Street address (short form) |
| `streetName` | Street name |
| `streetNumber` | Street number |
| `streetPrefix` | Street prefix |
| `streetSuffix` | Street suffix |
| `unit` | Unit/apartment number |
| `zipCode` | Postal code |
| `latitude` | Latitude |
| `longitude` | Longitude |
| `latitudeRange` | Latitude within a range |
| `longitudeRange` | Longitude within a range |
| `geohash` | Geohash string |

**`latitudeRange`/`longitudeRange`** — options `min`/`max` (float). Example:
constrain both to a bounding box around a specific city for realistic
"nearby" test data.

**`geohash`** — option `precision` (default `9`, higher = more precise). If
paired with a `latitude`/`longitude` pair via its `columns` option, it
derives a real geohash *from* those two sibling columns instead of a random
one — a two-column dependency, configured like the other cross-column
generators below.

## Date & Time

| Generator | Description |
|---|---|
| `datetime` | Date/time within a range |
| `dateRange` | Date/time within a range (alias-style pairing with `datetime`) |
| `timeRange` | Time within a range |
| `day` | Day of month |
| `futureDate` | Future date/time |
| `hour` | Hour |
| `minute` | Minute |
| `month` | Month number |
| `monthString` | Month name |
| `nanosecond` | Nanosecond component |
| `pastDate` | Past date/time |
| `second` | Second |
| `time` | Time of day |
| `timezone` | Timezone name |
| `timezoneAbv` | Timezone abbreviation |
| `timezoneFull` | Full timezone name |
| `timezoneOffset` | Timezone UTC offset |
| `timezoneRegion` | Timezone region |
| `weekday` | Weekday name |
| `year` | Year |
| `durationInterval` | A duration value for columns like `session_length`/`sla_window`/`cook_time` |
| `cronExpression` | A valid cron string picked from a curated set of common schedules |

**`datetime`/`dateRange`/`timeRange`** — options `from`, `to` (date/time
pickers in the UI), plus `onlyDate` (checkbox, strips the time component).
Example: seed `orders.created_at` with `from: 2025-01-01, to: 2026-01-01`
to keep all generated orders within the current fiscal year.

**`durationInterval`** — options `format` (`iso8601` / `short` / `seconds`,
default `short`), `minSeconds` (default `60`), `maxSeconds` (default
`14400`). Example: a `videos.duration` column with `format: iso8601,
minSeconds: 30, maxSeconds: 600` for short-clip-shaped durations.

## Numeric

| Generator | Description |
|---|---|
| `int` | Random integer |
| `intN` | Random integer below N |
| `intRange` | Random integer within a range |
| `int8`/`int16`/`int32`/`int64` | Sized signed integer |
| `uint`/`uint8`/`uint16`/`uint32`/`uint64` | Unsigned integer |
| `uintN` | Random unsigned integer below N |
| `uintRange` | Random unsigned integer within a range |
| `float` | Random decimal number |
| `float32` | Random 32-bit float |
| `float32Range`/`float64Range` | Random float within a range |
| `price` | Random decimal price |
| `hexUint` | Random unsigned integer, hex-formatted |
| `randomInt`/`randomUint` | Pick from a small internal pool |
| `shuffleInts` | Shuffled sequence of small integers |
| `percentageValue` | A percentage value |
| `fileSizeBytes` | Realistic file size in bytes (log-distributed) |

**`price`** — options `min` (default `1.0`), `max` (default `1000.0`).
Example: `products.price` with `min: 4.99, max: 249.99`.

**`int`/`float`** — options `min`/`max`. **`intN`/`uintN`** — option `n`
(upper-exclusive bound). **`intRange`/`uintRange`/`float32Range`/
`float64Range`** — options `min`/`max`. Example: `products.stock` as
`int` with `min: 0, max: 500`.

**`percentageValue`** — option `precision` (`0`–`2`, default `0`). Example:
a `discount_pct` column with `precision: 1` for values like `12.5`.

**`fileSizeBytes`** — options `minBytes` (default `1024`), `maxBytes`
(default `104857600`). Log-distributed, so most values cluster toward the
smaller end with occasional large outliers — realistic for file-size
columns.

## Distribution

| Generator | Description |
|---|---|
| `normal` | Normally (Gaussian) distributed value |
| `binomial` | Binomially distributed value |
| `exponential` | Exponentially distributed value |
| `geometric` | Geometrically distributed value |
| `poisson` | Poisson-distributed value |

Each takes its own statistical parameters as options (e.g. `normal` takes
`mean`/`stddev`) — use these when a plain uniform `int`/`float` range would
look too artificial, for example modeling response times (`exponential`)
or event counts per period (`poisson`).

## Internet

| Generator | Description |
|---|---|
| `url` | URL |
| `urlSlug` | URL-friendly slug |
| `ipv4` | IPv4 address |
| `ipv6` | IPv6 address |
| `macAddress` | MAC address |
| `domainName` | Domain name |
| `domainSuffix` | Domain suffix |
| `httpMethod` | HTTP method |
| `httpStatusCode` | HTTP status code |
| `httpStatusCodeSimple` | Common HTTP status code |
| `httpVersion` | HTTP version string |
| `apiUserAgent` | API-style user agent |
| `userAgent` | User agent string |
| `userAgentByDevice` | User agent for a specific device type |
| `logLevel` | Log level string |

**`userAgentByDevice`** — option `device` (`mobile`/`desktop`/`tablet`/
`bot`, default random-weighted). Example: seed `sessions.user_agent` with
`device: mobile` to test a mobile-only report.

## Finance

| Generator | Description |
|---|---|
| `achAccount` | ACH account number |
| `achRouting` | ACH routing number |
| `bankName` | Bank name |
| `bankType` | Bank account type |
| `bitcoinAddress` | Bitcoin address |
| `bitcoinPrivateKey` | Bitcoin private key |
| `ethereumAddress` | Ethereum address (`0x` + 40 hex chars) |
| `creditCardCvv` | Credit card CVV |
| `creditCardExp` | Credit card expiry |
| `creditCardNumber` | Credit card number |
| `creditCardType` | Credit card network |
| `currencyLong` | Currency name |
| `currencyShort` | Currency code |
| `iban` | IBAN |
| `cusip` | CUSIP identifier |
| `isin` | ISIN identifier |

:::caution
These produce structurally plausible values for UI/testing purposes only —
`iban` is not mod-97 validated, `ethereumAddress` is a random hex string,
and none of these are real, spendable financial instruments.
:::

## Company

| Generator | Description |
|---|---|
| `company` | Company name |
| `blurb` | Company blurb |
| `bs` | Corporate buzzword phrase |
| `buzzword` | Single buzzword |
| `companySuffix` | Company suffix (Inc, LLC, ...) |
| `jobDescriptor` | Job descriptor |
| `jobLevel` | Job level |
| `jobTitle` | Job title |
| `slogan` | Company slogan |

## Color

| Generator | Description |
|---|---|
| `color` | Color name |
| `hexColor` | Hex color code |
| `safeColor` | Web-safe color name |
| `shortHexColor` | Short hex color code |

## Text & Grammar

| Generator | Description |
|---|---|
| `sentence` | Sentence |
| `paragraph` | Paragraph |
| `word` | Single word |
| `phrase` | Phrase |
| `comment` | Comment-style text |
| `question` | Question |
| `quote` | Quote |
| `loremIpsumWord`/`loremIpsumSentence`/`loremIpsumParagraph` | Lorem-ipsum-style filler text |
| `versionString` | Semantic version string |

**`sentence`** — option `wordCount` (default `8`). **`paragraph`** — option
`sentences` (default `3`). Example: `reviews.body` as `paragraph` with
`sentences: 2` for short reviews, or `sentences: 8` for detailed ones.

**`versionString`** — options `preReleaseRate` (default `0.1`), `maxMajor`
(default `5`), `maxMinor` (default `20`), `maxPatch` (default `50`).
Example: a `releases.version` column with `preReleaseRate: 0.3` to get more
`-alpha`/`-beta`-suffixed versions in your test data.

## Food

| Generator | Description |
|---|---|
| `food` | Food item name |

**`food`** — option `mealType` (`breakfast`/`dessert`/`dinner`/`drink`/
`fruit`/`lunch`/`snack`/`vegetable`, default `breakfast`). Example: a
`menu_items.name` column with `mealType: dinner`.

## Product

| Generator | Description |
|---|---|
| `productName` | Product name |
| `productAudience` | Target audience |
| `productBenefit` | Product benefit |
| `productCategory` | Product category |
| `productDescription` | Product description |
| `productDimension` | Product dimension |
| `productFeature` | Product feature |
| `productIsbn` | ISBN |
| `productMaterial` | Product material |
| `productSuffix` | Product name suffix |
| `productUpc` | UPC code |
| `productUseCase` | Product use case |

## Identifier

| Generator | Description |
|---|---|
| `uuid` / `guid` | UUID (v4) — `guid` is an alias of `uuid` |
| `ulid` | ULID |
| `mongoObjectId` | MongoDB ObjectId-shaped hex string |
| `digit` | Single digit |
| `digitN` | N-digit number |
| `letter` | Single letter |
| `letterN` | N-letter string |
| `vowel` | Single vowel |
| `randomString` | Random string |
| `lexify` | String from a letter pattern |
| `numerify` | String from a digit pattern |
| `regex` | String matching a regex pattern |
| `bytes` | Random binary blob |
| `fileExtension` | File extension |
| `mimeType` | MIME type (matches `fileExtension`'s category) |

**`digitN`/`letterN`** — option `n` (default `6`). Example: `letterN` with
`n: 8` for an 8-character alphabetic code.

**`lexify`** — option `pattern` (default `????????`, each `?` becomes a
random letter). **`numerify`** — option `pattern` (default `#####`, each
`#` becomes a random digit). Example: `lexify` with `pattern: SKU-????-??`
for a structured-but-random SKU.

**`regex`** — option `pattern` (required). Example: `pattern:
[A-Z]{3}-\d{4}` for a value like `XJQ-4821`. This is the single most
flexible identifier generator — reach for it whenever a column needs a
specific shape that isn't already covered.

**`bytes`** — option `length` (default `16`).

**`fileExtension`/`mimeType`** — option `category` (`document`/`image`/
`archive`/`audio`/`video`/`code`, default any). Use the same `category` on
both a `files.extension` and `files.mime_type` column pair to keep them
consistent (they won't automatically match each other unless you pin both
to the same category).

## Sequence (stateful)

These increment deterministically per row, rather than picking randomly —
useful for columns that need to look ordered.

| Generator | Description |
|---|---|
| `sequence` | Increments by a fixed step each row |
| `rowNumber` | 1-based row counter |
| `characterSequence` | Base-26 labeling (A, B, ..., Z, AA, AB, ...) |
| `digitSequence` | Zero-padded incrementing digit string |

**`sequence`** — options `start`, `step`. Example: `start: 1000, step: 1`
for an order-number-shaped column.

**`digitSequence`** — options for start value and zero-padded width.
Example: width `6` produces `000001`, `000002`, ...

## Security

| Generator | Description |
|---|---|
| `md5`/`sha1`/`sha256` | Hash of a random word |
| `password` | Password string |
| `passwordHash` | Bcrypt hash of a random password |
| `encrypt` | Base64 of random bytes (not real encryption) |
| `naughtyString` | A string from a curated "naughty strings" test-data pool |

**`password`** — options `lower`, `upper`, `numeric`, `special` (all
booleans, default `true`), `length` (default `12`). Example: `length: 20,
special: false` for a policy that disallows special characters.

`naughtyString` is intended specifically for exercising input validation —
it draws from strings known to break naive parsers (SQL-injection-shaped
text, unicode edge cases, format-string tokens, etc.), not for representing
real user data.

## Novelty & theme packs

Each of these exposes a `category` option to pick a specific facet of its
theme.

| Generator | `category` choices |
|---|---|
| `airline` | `aircraftType` |
| `animal` | `animal`, `type` |
| `app` | `name`, `version`, `author` |
| `beer` | `name`, `style`, `hop`, `yeast`, `malt`, `alcohol` |
| `book` | `title`, `author`, `genre` |
| `car` | `maker`, `model`, `type`, `fuelType`, `transmissionType`, `modelYear`, `vin` |
| `celebrity` | `actor`, `business`, `sport` |
| `emoji` | `emoji`, `category`, `alias`, `tag` |
| `error` | `generic`, `database`, `grpc`, `http`, `httpClient`, `httpServer`, `runtime` |
| `game` | `gamertag` |
| `hacker` | `phrase`, `abbreviation`, `adjective`, `noun`, `verb` |
| `hipster` | `word`, `sentence`, `paragraph` |
| `language` | `name`, `abbreviation` |
| `minecraft` | biome/mob/ore and other Minecraft-flavored facets |
| `misc` | `flipACoin`, `weightedChoice` |
| `movie` | `name`, `genre` |
| `school` | (single string, no options) |
| `song` | `name`, `artist`, `genre` |

Example: a `logs.message` column seeded with `error`, `category:
httpServer`, for a realistic-looking server-error log fixture. Or
`car`, `category: vin`, for a `vehicles.vin` column.

## Domain lookup

| Generator | `category` choices |
|---|---|
| `healthcare` | `drugName`, `icdCode`, `hospitalName` |
| `banking` | `bankName`, `swiftCode` |
| `construction` | `equipment`, `trade`, `material` |

Example: a `claims.diagnosis_code` column seeded with `healthcare`,
`category: icdCode`.

## Media / binary

BLOB-producing generators, each with a hard size ceiling (64 KB for
images/SVG/icon/QR/barcode, 128 KB for `soundData`) — exceeding it is a
hard error, not a silent truncation.

**`qrCode`** — options `content` (default a random UUID), `size` (default
`256`, range `64`–`1024`). Produces a real, scannable PNG QR code. Example:
`content: https://example.com/{{uuid}}` shaped values for per-row unique
QR codes (via `.echo`/templates in `squad cli`, or a `jsonTemplate`-style
composed value in the UI).

**`barcode`** — options `format` (`code128`/`ean13`/`ean8`, default
`code128`), `content` (digit string; must be exactly 13 digits for
`ean13` or 8 for `ean8` if you supply your own), `size` (default `300`,
range `100`–`800`). Example: `format: ean13` for realistic retail-product
barcodes.

**`profilePicture`** — options `seed` (default a random UUID — **the same
seed always produces the identical image**), `size` (default `128`, range
`32`–`512`). A deterministic, procedurally-composited cartoon avatar.
Example: set `seed` to the row's own generated `id`/`email` value (via a
cross-column reference) so the same user always gets the same avatar
across re-seeds.

**`svgImage`** — options `shape` (`circles`/`rects`/`blob`, default
`circles`), `size` (default `200`, range `50`–`800`). Produces a real,
valid SVG (not a raster image) — usable for either a BLOB or TEXT column.

**`icon`** — options `name` (one of 7 embedded Lucide icons: `alert-circle`,
`check-circle`, `heart`, `home`, `mail`, `settings`, `star`, `user` —
default random), `color` (hex string, recolors the icon). Example: `name:
star, color: #f59e0b` for a consistently-colored star icon fixture.

**`soundData`** — options `waveform` (`sineTone`/`squareWave`/
`triangleWave`/`sawtoothWave`/`whiteNoise`/`pinkNoise`/`chirp`/`dtmf`/
`notificationChime`/`drumHit`, default `sineTone`), `durationMs` (default
`500`, range `50`–`5000`, ignored by `dtmf`/`notificationChime`/`drumHit`),
`frequency` (default `440`, used by tone waveforms), `startFrequency`/
`endFrequency` (used by `chirp`), `digit` (DTMF key, `0`–`9`/`*`/`#`/`A`–`D`,
default `5`), `decayMs` (default `150`, used by `drumHit`). Produces a real
16-bit PCM WAV file. Example: `waveform: notificationChime` for a
`sounds.data` column that needs to sound like an actual UI notification.

## Custom list & conditional

These let *you* supply the value pool or logic, rather than picking from a
built-in catalog.

**`oneOf`** — option `values` (comma or newline-separated list, at least 2
required). Uniform random pick. Example: `values: PAID, PENDING, REFUNDED`
for an `orders.status` column with no weighting.

**`weightedOneOf`** — option `values` as `value:weight` pairs (bare values
default to weight `1`). Example: `values: PAID:70, PENDING:20,
REFUNDED:10` produces roughly 70% `PAID` rows — much more realistic than a
uniform `oneOf` for a status-like column.

**`regexEnum`** — option `patterns` (one regex per line). Randomly picks a
pattern per row, then expands it into a matching string. Example:
`patterns:\nSKU-[A-Z]{3}-[0-9]{4}\nLEGACY-[0-9]{6}` mixes two SKU formats
in the same column, mimicking a real migrated dataset.

**`incrementalEnum`** (stateful) — options `values`, `start` (default
`0`), `step` (default `1`). Cycles deterministically through the list in
order rather than picking randomly — useful when you want an even,
predictable spread across a fixed set of values (e.g. round-robin
assigning rows across a fixed set of `region` codes).

## Cross-column

These read *other columns' already-generated values* for the same row.
Pick which columns to depend on in the generator's options — squad
automatically figures out the right generation order and rejects circular
dependencies before any rows are generated.

**`formula`** — options `columns` (sibling columns the expression reads),
`expression`. A restricted arithmetic expression: bare identifiers resolve
to sibling column values, `+ - * /` do ordinary arithmetic (`+`
concatenates when both sides are strings), and a whitelisted function set
is available:
- string — `upper`, `lower`, `concat`, `trim`, `len`, `capitalize`,
  `kebabCase`, `camelCase`
- encoding — `hex`, `base32`, `base64`
- crypto digests — `sha1`, `md5`, `sha256`, `sha512`
- math — `abs`, `round`, `floor`, `ceil`, `min`, `max`, `pow`, `mod`

Examples: `columns: price, qty`, `expression: round(price * qty)` for an
`order_items.line_total` column; `columns: email`, `expression:
sha256(email)` for a hashed lookup key.

**`dependentOneOf`** — options `columns` (exactly one dependency column),
`cases` (one `whenValue => v1|v2|...` per line, plus an optional `default
=>` line). Example: depend on a `country` column with `cases:
US => USD\nDE => EUR\ndefault => USD` to generate a currency that's
consistent with each row's country.

**`customDateSequence`** — options `columns` (an ordered milestone list
including this column's own name at its position), `gaps` (one
`minMinutes-maxMinutes` range per milestone step), `skipProbability`
(`0`–`1`). Builds a coherent timeline across several columns — e.g.
`ordered_at` → `paid_at` → `shipped_at` → `delivered_at`, each a random
but plausible gap after the previous non-null milestone, with
`skipProbability` occasionally leaving a milestone `NULL` (e.g. not yet
shipped).

**`statusTransitionLog`** — option `columns` (one dependency column
holding the row's real terminal status), `transitions` (one `fromStatus
=> to1,to2` per line, describing a state machine). Produces a plausible
`→`-joined path from a valid starting state to the row's actual status —
useful for an audit-log-shaped column that should read like a real history
of state changes, not just the final state.

**`checksumOfColumns`** — options `columns` (one or more), `algorithm`
(`md5`/`sha1`/`sha256`, default `sha256`), `separator` (default `|`).
Concatenates the listed columns' values with the separator and hashes the
result — for an integrity-check-shaped column.

**`slugFromColumn`** — options `columns` (exactly one), `suffixLength`
(default `0`). Lowercases the source value, replaces non-alphanumeric runs
with `-`, and optionally appends a random lowercase-letter suffix for
uniqueness. Example: `columns: title`, `suffixLength: 4` turns `"Hello
World!"` into something like `hello-world-xk3q`.

**`jsonTemplate`** — options `columns` (sibling columns referenced),
`template` (a string with `{{column:name}}` and
`{{generator:name(jsonOptions)}}` tokens). Builds a JSON document by
substituting sibling-column values and inline generator calls, then
validates the result is real JSON. Example template:
```
{"customer": "{{column:name}}", "note": "{{generator:sentence({"wordCount":5})}}"}
```
Cannot nest `foreignKey`, `enumFromColumn`, `nullWithProbability`, another
cross-column generator, or any stateful generator inside a
`{{generator:...}}` token — those need row/table context this generator
doesn't have.

## Special (database-aware)

These two need real database context and can't be previewed standalone in
the generator picker's sample view — they're resolved when you actually
run the seed.

**`foreignKey`** — auto-assigned by the seed plan for detected foreign-key
columns; samples an existing row from the referenced table so every
generated value points at something real. Fails with an `EMPTY_REFERENCE`
error if the referenced table has no rows yet — seed the parent table
first.

**`enumFromColumn`** — options `table`, `column` (both required, and
validated to exist before seeding starts). Samples up to 500 distinct real
values already present in that table's column, then picks uniformly among
them per row. Example: seed `orders.shipping_country` from
`table: customers, column: country` so shipping countries only ever match
countries that already appear among your customers.

## Wrapper

**`nullWithProbability`** — options `generator` (a nested generator +
options, picked via a mini generator-picker inside this generator's own
options form), `nullRate` (`0`–`1`, default `0.15`). Wraps any other
generator and substitutes SQL `NULL` with probability `nullRate` instead of
that generator's normal output. Example: wrap `phone` with `nullRate: 0.3`
to simulate "30% of customers didn't provide a phone number." Cannot wrap
itself, and the wrapped generator name must be a real, known generator.

## Boolean

**`bool`** — no options. Produces `0`/`1`. Use this instead of `int` for
any column that's semantically a boolean flag (e.g. `active`, `is_admin`).

## Previewing before you seed

Every generator card in the picker shows a live one-value sample on
hover/focus — for the database-aware generators (`foreignKey`, `formula`,
`enumFromColumn`), no sample is fetched since they need row/table context
that isn't available outside an actual seed run. Use **Preview 5 rows** in
the Seed tab itself (see the
[walkthrough](/docs/web-ui-guide/seed-data-walkthrough/#3-set-a-row-count-and-preview-before-inserting))
to check those in context instead.
