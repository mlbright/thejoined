# Copilot Instructions for thejoined

## Project Summary

`thejoined` is a Go project that generates the Pluribus sequence. It is a small, focused library/command-line tool with no external dependencies beyond the Go standard library.

## Language and Runtime

- **Language**: Go
- **Minimum Go version**: Check `go.mod` for the declared version once it exists; otherwise use the latest stable Go release.

## Repository Layout

```
.
├── .github/
│   └── copilot-instructions.md   # this file
├── .gitignore                    # Go-standard ignores (binaries, test artifacts, coverage)
├── LICENSE                       # Apache 2.0
└── README.md                     # One-line description
```

Source files and a `go.mod` will be added as the project develops. Expect a typical flat Go package layout or a `cmd/` + internal package structure.

## Build, Test, and Lint

Once `go.mod` exists, use the following commands from the repository root:

```bash
# Bootstrap (only needed once, or after adding a new dependency)
go mod tidy

# Build
go build ./...

# Run tests
go test ./...

# Run tests with race detector (recommended)
go test -race ./...

# Vet (static analysis bundled with Go)
go vet ./...
```

There is currently no additional linter configuration. If a `.golangci.yml` or `Makefile` is added in the future, prefer those over raw `go` commands.

## Key Conventions

- Follow standard Go idioms and the [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments).
- Keep the public API minimal and well-documented with Go doc comments.
- All exported symbols must have doc comments.
- Prefer table-driven tests using `testing.T`.
- Do not vendor dependencies; use Go modules (`go.mod` / `go.sum`).
- The `.gitignore` already excludes `*.exe`, `*.out`, `coverage.*`, and `go.work`.

## CI / Validation

There are no GitHub Actions workflows yet. When added they will live in `.github/workflows/`. Until then, validate changes by running `go build ./...`, `go test -race ./...`, and `go vet ./...` locally before opening a PR.
