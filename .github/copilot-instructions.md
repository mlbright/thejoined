# Copilot Instructions for thejoined

## Project Summary

`thejoined` is a web server that assists with network traffic diagnostics and debugging.

HTTP or HTTPS requests sent to the server will be logged and the request details will be returned in the response body, along with a variable number of 'G','U','A', and 'C' characters to round out the desired payload size.

The payload size can be configured by the client via a HEADER, allowing users to test how their applications handle different response sizes.
Sizes can be specified in bytes, kilobytes, megabytes, or gigabytes, using the standard suffixes (B, KB, MB, GB). For example, a client could request a 1MB response by including the header `X-Payload-Size: 1MB`.

By default, the server will return a payload of 10MB if no size is specified.

By default, the IP address and port of the requester will be specified at the beginning of the response body, followed by the request method, URL, and headers.
This will be refered to as the "request information section" of the payload.
To round out the size request, an assortment of 'G','U','A', and 'C' characters will be appended to the end of the request information section until the total payload size is met.
The minimum size of the request is the "request information section".

The server will also log this information to the console for easy access.

This allows users to easily inspect the contents of their requests and responses, making it a useful tool for debugging and troubleshooting network issues.

It is a Go project that is a small, focused server with no external dependencies beyond the Go standard library.

By default, it is published as a docker image on Docker Hub, but a systemd unit file and manual build instructions will be provided for users who prefer to run it directly on their machines.

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
