# squad

> A minimal, single-binary, dual mode (CLI + WebUI) SQLite client.

`squad` is a lightweight developer tool that lets you explore, query, edit, and export SQLite databases from a modern browser—without installing a desktop application or running Docker.

Simply point it at an SQLite database and it launches a local web interface.

```bash
$ squad ./app.db

squad v1.0.0
database : ./app.db
address  : http://127.0.0.1:7071
press Ctrl+C to stop
```

Open the displayed URL in your browser and start working with your database.

---

## Why squad?

- 🚀 Single executable (no dependencies)
- 🔒 Safe by default (read-only mode)
- 🌐 Modern browser-based interface
- ⚡ Starts in under a second
- 📦 Cross-platform
- 🛠 Built specifically for developers

---

# Features

## Database Explorer

- Browse all tables
- View schemas
- Inspect indexes
- Explore foreign keys
- Navigate large datasets

## SQL Editor

- Execute SQL interactively
- Syntax highlighting
- Query history
- Multiple output formats
- Copy results

## Data Management

Write mode enables:

- Insert records
- Update records
- Delete records
- Create tables
- Alter tables
- Drop tables

> Write operations are disabled by default and require the `--write` flag.

## Export

Export query results as:

- CSV
- JSON
- SQL INSERT statements

## Fake Data Generator

Populate tables with realistic sample data for development and testing.

## REST API

Optionally expose database tables as REST endpoints.

## Local Web UI

- Responsive interface
- Light & Dark themes
- Embedded frontend
- No internet connection required

## Rich CLI
- Interactive shell
- Command-line flags
- Dot commands
- Query history
- Seed tables with fake data
- Template functions
- Template support for SQL queries.

---

# Installation

Download the appropriate binary for your platform and place it somewhere on your `PATH`.

No installation, runtime, or database server is required.

---

# Quick Start

Open a database:

```bash
squad ./database.db
```

Open in write mode:

```bash
squad ./database.db --write
```

Enable REST endpoints:

```bash
squad ./database.db --rest
```

Choose a custom port:

```bash
squad ./database.db --port 8080
```

Bind to a custom address:

```bash
squad ./database.db --host 0.0.0.0
```

---

# Command Line

```text
squad [database] [flags]
```

Common flags:

| Flag         | Description               |
|--------------|---------------------------|
| `--write`    | Enable data modifications |
| `--rest`     | Enable REST endpoints     |
| `--host`     | Server bind address       |
| `--port`     | HTTP port                 |
| `--readonly` | Force read-only mode      |
| `--help`     | Show help                 |

---

# Interactive CLI

`squad` also includes an interactive shell for quick database operations.

Example:

```bash
squad cli database.db
```

Supports:

- SQL execution
- Query history
- Dot commands
- Multiple output formats

---

# REST API

When REST mode is enabled, squad exposes JSON APIs for:
- Table CRUD (write mode)

Example:

```text
GET /rest/contacts?limit=10"
GET /rest/contacts/:pk
POST /rest/contacts
PATCH /rest/contacts/:pk
DELETE /rest/contacts/:pk
```

---

# Safety

Security is a core design goal.

By default:

- Database is opened read-only
- Server binds to `127.0.0.1`
- No authentication is required because it is intended for local development
- Destructive operations are disabled

Write access requires explicit user opt-in.

---

# Project Goals

- Zero configuration
- Single binary
- Fast startup
- Local-first
- Developer-friendly
- Safe defaults
- Modern user experience

---

# Typical Use Cases

- Application development
- SQLite inspection
- Local debugging
- API prototyping
- Data exploration
- QA testing
- Demo databases
- Fake data generation

---

# Roadmap

- AI-assisted SQL
- SQL editor improvements
- Code generation

---

# Contributing

Contributions, feature requests, and bug reports are welcome.

Please open an issue before submitting large changes.

---

# License

[MIT](LICENSE)

---


