---
title: Global Flags
description: Flags shared across squad's commands.
---

These flags are shared between the root command (`squad <db>`) and
`squad sandbox`, registered by a common `commonFlags`/`restFlags` struct in
`cmd/flags.go`. `squad cli` only picks up the REST-related and logging flags
(see [squad cli](/squad/docs/cli-reference/cli/)); `squad version` takes none.

## Bind & browser flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--addr` | `-a` | `127.0.0.1` | Bind address for the main HTTP listener. |
| `--port` | `-p` | `7071` | Port for the main HTTP listener. |
| `--open` | `-o` | `true` | Auto-open the default browser on start. |
| `--token` | `-t` | `""` | Optional bearer token gate for `/api/*`. Off by default. Does **not** gate `/rest/*`. |
| `--log-level` | `-l` | `info` | Log level: `debug`/`info`/`warn`/`error`. |

## Auto-REST flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--rest` | `-r` | `false` | Unlock the auto-REST capability for this process. Does **not** auto-mount tables or start the listener — see [Auto-REST](/squad/docs/rest-api/auto-rest/). |
| `--rest-port` | | `7072` | Port for the separate REST listener (distinct from `--port`). |
| `--rest-bind-addr` | | `127.0.0.1` | Bind address for the REST listener. |

Binding either listener to `0.0.0.0` prints the same broadcast warning,
recommending you also set `--token`.

## Environment variable overrides

Flags take precedence over environment variables, which take precedence
over defaults. squad currently honors `SQUAD_EXAMPLES`, `SQUAD_REST`,
`SQUAD_REST_PORT`, `SQUAD_REST_BIND_ADDR`, and (for `squad sandbox`)
`SQUAD_SANDBOX_DIR` — each only applied when the corresponding flag was not
explicitly passed on the command line.
