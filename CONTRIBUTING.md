# Contributing to squad

Thanks for your interest in contributing.

## Development setup

```bash
git clone https://github.com/0funct0ry/squad
cd squad
```

squad requires Go 1.26+ and Node 22+ (for the web UI).

## Common tasks

| Task | Command |
|---|---|
| Build the binary | `make build` |
| Run tests | `make test` |
| Run `go vet` | `make vet` |
| Format code | `make fmt` |
| Build the web UI only | `make web` |

Run `make help` for the full list of Makefile targets.

Please run `make vet`, `make test`, and `make fmt` before opening a PR.

## Pull request process

1. Open an issue first for anything beyond a small fix, to discuss the
   approach before investing time.
2. Fork and branch from `main`.
3. Keep PRs focused — one logical change per PR.
4. Make sure `make vet` and `make test` pass locally; CI (`.github/workflows/ci.yml`)
   runs the same checks on every PR.
5. Write commit messages following [Conventional Commits](https://www.conventionalcommits.org/)
   style (`feat:`, `fix:`, `docs:`, etc.) — the release changelog groups by
   these prefixes.

## Releases

Releases are cut manually by a maintainer via:

```bash
gh workflow run release.yml -f version=vX.Y.Z
```

There is no tag-push trigger; this is intentional — see
`.github/workflows/release.yml`'s header comment for the secrets it
requires.

## Project spec

The internal design/spec documents (`SPEC.md`, milestone-by-milestone
development log) live in a maintainer-only `internal-docs/` directory that
isn't part of the public repo. For public-facing reference material, see
the [docs site](https://0funct0ry.github.io/squad/).
