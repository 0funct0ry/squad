---
title: Installation
description: How to install squad.
---

squad ships as a single statically linked binary with no runtime
dependencies. Choose whichever install path fits your platform.

## Homebrew

```bash
brew install 0funct0ry/squad/squad
```

## curl install script

```bash
curl -fsSL https://raw.githubusercontent.com/0funct0ry/squad/main/scripts/install.sh | sh
```

Installs to `/usr/local/bin` (or `$HOME/.local/bin` if that isn't writable)
without requiring `sudo`. Verifies the downloaded archive against the
release's `checksums.txt` before installing.

## Docker

```bash
docker run --rm -p 7071:7071 -v $(pwd):/data ghcr.io/0funct0ry/squad:latest /data/your.db
```

## Scoop (Windows)

```powershell
scoop bucket add squad https://github.com/0funct0ry/scoop-squad
scoop install squad
```

## Download a release binary

Download the archive for your platform from the
[GitHub Releases page](https://github.com/0funct0ry/squad/releases),
extract it, and place the `squad` binary on your `PATH`.

## Install with Go

If you have a Go toolchain installed (Go 1.26 or later):

```bash
go install github.com/0funct0ry/squad@latest
```

This places a `squad` binary in `$(go env GOPATH)/bin` — make sure that
directory is on your `PATH`.

## Build from source

```bash
git clone https://github.com/0funct0ry/squad
cd squad
make build
```

This produces `bin/squad`, built with version/commit information baked in
via linker flags.

## Verify the install

```bash
squad version
```
