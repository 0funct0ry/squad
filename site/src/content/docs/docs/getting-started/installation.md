---
title: Installation
description: How to install squad.
---

squad ships as a single statically linked binary with no runtime
dependencies. There is currently one supported install path.

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

No packaged binaries (Homebrew, apt, standalone release archives) exist yet
— `go install` or building from source are the only options for now.
