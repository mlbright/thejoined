# Copilot Instructions for thejoined

## Project Summary

`thejoined` is a web server that assists with network traffic diagnostics and debugging.

HTTP requests sent to the server are logged and the request details are returned in the response body, along with a variable number of `G`, `U`, `A`, and `C` characters to round out the desired payload size.

The payload size can be configured by the client via a request header, allowing users to control the size of the response. Sizes can be specified in bytes, kilobytes, megabytes, or gigabytes using the standard suffixes (B, KB, MB, GB). For example, a client could request a 1 MB response by including the header `X-Payload-Size: 1MB`.

By default, the server returns a payload of **10 MB** if no size is specified.

The response body begins with the **request information section**: the requester's remote address, method, URL, and all request headers. This is followed by `G`, `U`, `A`, `C` padding characters until the total payload size is reached. The minimum payload is always the request information section, even if a smaller size is requested.

The nucleotide order can be fixed by the client via the `X-Nucleotide-Order` request header (e.g. `X-Nucleotide-Order: UCAG`). If omitted, the order is shuffled randomly per request.

The `X-Payload-Checksum` response header carries a CRC32/IEEE checksum (hex) of the full response payload.

The server also logs request details to the console.

It is a Go project with no external dependencies beyond the Go standard library.

The container image is published to Docker Hub and GHCR. A systemd unit file and manual build instructions are provided for users who prefer to run it directly on their machines.

## Language and Runtime

- **Language**: Go
- **Module**: `github.com/mlbright/thejoined`
- **Minimum Go version**: 1.26 (declared in `go.mod`)

## Repository Layout

```
.
├── .github/
│   ├── copilot-instructions.md        # this file
│   └── skills/
│       └── use-modern-go/SKILL.md     # Go version-specific feature guidelines
├── .gitignore                         # Go-standard ignores (binaries, test artifacts, coverage)
├── Dockerfile                         # Multi-stage build: golang:1.24-alpine → alpine:3.21
├── LICENSE                            # Apache 2.0
├── Makefile                           # Docker build/push automation (GHCR + Docker Hub)
├── README.md                          # Project documentation and usage examples
├── go.mod                             # Module definition (github.com/mlbright/thejoined, go 1.24)
├── main.go                            # HTTP server implementation
├── main_test.go                       # Test suite
└── rna.service                        # systemd unit file
```

## Configuration

### Environment Variables

| Variable   | Default | Description      |
|------------|---------|------------------|
| `RNA_PORT` | `8080`  | Listening port (set to `80` inside the Docker image) |

### Request Headers (client-controlled)

| Header              | Default | Description |
|---------------------|---------|-------------|
| `X-Payload-Size`    | `10MB`  | Desired response size (e.g. `512B`, `64KB`, `5MB`, `1GB`) |
| `X-Nucleotide-Order`| random  | Fixed GUAC pattern (e.g. `GUAC`, `UCAG`) |

### Response Headers

| Header               | Description |
|----------------------|-------------|
| `X-Payload-Checksum` | CRC32/IEEE hex checksum of the full response payload |
| `Content-Length`     | Actual response byte count |
| `Content-Type`       | `text/plain; charset=utf-8` |

## Build, Test, and Lint

```bash
# Build (binary is named rna)
go build -o rna .

# Run tests
go test ./...

# Run tests with race detector (recommended)
go test -race ./...

# Static analysis
go vet ./...
```

## Docker

The Dockerfile is a multi-stage build:
- **Builder** (`golang:1.26-alpine`): produces a static, stripped binary at `/build/rna`
- **Runtime** (`alpine:3.21`): copies the binary, sets `RNA_PORT=80`, exposes port 80

```bash
# Build and tag (uses git describe for version tag)
make build

# Push to GHCR (ghcr.io/mlbright/thejoined) and Docker Hub (mlbright/thejoined)
make push

# Build + push in one step
make publish
```

## systemd

```bash
sudo cp rna /usr/local/bin/rna
sudo cp rna.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now rna
```

## Key Conventions

- Follow standard Go idioms and the [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments).
- No external dependencies; use only the Go standard library.
- Keep the public API minimal and well-documented with Go doc comments.
- All exported symbols must have doc comments.
- Prefer table-driven tests using `testing.T`.
- Use `b.Loop()` (Go 1.24+) instead of `for i := 0; i < b.N; i++` in benchmarks.
- Use `t.Context()` (Go 1.24+) instead of `context.WithCancel(context.Background())` in tests.
- Use `new(val)` (Go 1.26+) instead of `x := val; &x` for pointer literals.
- Use `errors.AsType[T](err)` (Go 1.26+) instead of `errors.As(err, &target)`.
- Use `wg.Go(fn)` (Go 1.25+) instead of `wg.Add(1)` + manual goroutine wrapper.
- Do not vendor dependencies; use Go modules (`go.mod` / `go.sum`).
- Large responses are streamed in 32 KB chunks to avoid buffering in memory.
- The `.gitignore` already excludes `rna`, `*.exe`, `*.out`, `coverage.*`, and `go.work`.

Refer to `.github/skills/use-modern-go/SKILL.md` for the full list of Go version-specific features available in this project.

## CI / Validation

There are no GitHub Actions workflows yet. When added they will live in `.github/workflows/`. Until then, validate changes by running `go build -o rna .`, `go test -race ./...`, and `go vet ./...` locally before opening a PR.
