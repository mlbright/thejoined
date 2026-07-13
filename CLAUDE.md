# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build (binary is named rna)
go build -o rna .

# Run all tests
go test ./...

# Run tests with race detector
go test -race ./...

# Run a single test
go test -run TestName ./...

# Static analysis
go vet ./...
```

## Architecture

`thejoined` is a single-package Go HTTP server (`main.go`) with no external dependencies. The entire server logic lives in one file.

**Request flow:**
1. Any path is accepted via a catch-all handler (`mux.HandleFunc("/", handler)`)
2. `setRequestHeaders()` copies the remote address, method, URL, and all incoming request headers onto the response as `X-Request-*` headers (the response body itself contains no request metadata)
3. `parseSize()` reads the `X-Payload-Size` request header (default 1 KB) to determine total payload size
4. `nucleotidePattern()` reads the optional `X-Nucleotide-Order` header (any pattern, truncated to 8 chars) and returns it, falling back to a randomly shuffled `GUAC`
5. `writePadding()` streams the repeating pattern in 32 KB chunks until the total size is reached — this avoids buffering large payloads in memory; the body is pure padding sized exactly to `X-Payload-Size`
6. `computeChecksum()` calculates a CRC32/IEEE checksum of the padding and sets it in the `X-Payload-Checksum` response header (8-character hex) before writing the body

### Modes

The binary runs in one of two modes, fixed at startup by `RNA_MODE` (default `server`):

- **Server** (`server.go`) — the diagnostic server described above (`runServer`).
- **Client** (`clientapi.go`, `manager.go`, `engine.go`, `runspec.go`, `paramspec.go`, `metrics.go`) — a closed-loop load generator exposing a REST API (`runClient`). A `Manager` holds in-memory `Run`s; each `Run` drives N worker goroutines that build requests via per-parameter `Selector`s (round-robin default), verify responses against `X-Payload-Checksum`, and record per-payload-size metrics.

See `CONTEXT.md` for the domain vocabulary and `docs/adr/` for recorded decisions.

**Configuration** is via the `RNA_PORT` environment variable (default `8080`; set to `80` inside the Docker image).

## Docker & publishing

The Dockerfile is a two-stage build (`golang:1.26-alpine` → `alpine:3.21`) producing a static binary. The builder stage is pinned to `$BUILDPLATFORM` and cross-compiles to `$TARGETARCH`, and the runtime stage is deliberately RUN-free, so multi-arch builds never use QEMU emulation. Images are tagged with the git-describe version plus `latest`:

```bash
make build         # single-arch build, tagged for GHCR + Docker Hub
make publish       # build + push to GHCR and Docker Hub (manual flow)
make publish-ghcr  # multi-arch (amd64+arm64) build + push to GHCR only;
                   # GHCR_OWNER=<owner> overrides the cpacketnetworks default
```

CI (`.github/workflows/publish-ghcr.yml`) runs vet + tests, then `make publish-ghcr` on every push to `main` (and on manual dispatch). The GHCR namespace is derived from the repository owner, so the same workflow publishes `ghcr.io/cpacketnetworks/thejoined` from the cPacketNetworks mirror and `ghcr.io/mlbright/thejoined` from the mlbright mirror using only `GITHUB_TOKEN` — see `docs/adr/0001-owner-derived-ghcr-publishing.md`.

## Go version

This project targets **Go 1.26**. Use the idioms documented in `.github/skills/use-modern-go/SKILL.md`, notably:
- `t.Context()` instead of `context.WithCancel(context.Background())` in tests
- `b.Loop()` instead of `for i := 0; i < b.N; i++` in benchmarks
- `for i := range n` instead of `for i := 0; i < n; i++`
- `new(val)` instead of `x := val; &x` for pointer literals
- `errors.AsType[T](err)` instead of `errors.As(err, &target)`
- `wg.Go(fn)` instead of `wg.Add(1)` + manual goroutine wrapper
