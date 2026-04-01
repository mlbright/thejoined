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
2. `buildRequestInfo()` formats the remote address, method, URL, and headers into the response preamble
3. `parseSize()` reads the `X-Payload-Size` request header (default 10 MB) to determine total payload size
4. `nucleotidePattern()` reads the optional `X-Nucleotide-Order` header and returns a validated/shuffled `GUAC` pattern
5. `writePadding()` streams `G`/`U`/`A`/`C` padding in 32 KB chunks until the total size is reached — this avoids buffering large payloads in memory
6. `computeChecksum()` calculates a CRC32/IEEE checksum of the full response and sets it in the `X-Payload-Checksum` response header before writing the body

The minimum payload is always the request information section, even if a smaller size is requested.

**Configuration** is via the `RNA_PORT` environment variable (default `8080`; set to `80` inside the Docker image).

## Docker & publishing

The Dockerfile is a two-stage build (`golang:1.24-alpine` → `alpine:3.21`) producing a static binary. Images are tagged with the git-describe version and pushed to both `ghcr.io/mlbright/thejoined` and `mlbright/thejoined` via the Makefile:

```bash
make build    # build and tag
make publish  # build + push to GHCR and Docker Hub
```

## Go version

This project targets **Go 1.26**. Use the idioms documented in `.github/skills/use-modern-go/SKILL.md`, notably:
- `t.Context()` instead of `context.WithCancel(context.Background())` in tests
- `b.Loop()` instead of `for i := 0; i < b.N; i++` in benchmarks
- `for i := range n` instead of `for i := 0; i < n; i++`
- `new(val)` instead of `x := val; &x` for pointer literals
- `errors.AsType[T](err)` instead of `errors.As(err, &target)`
- `wg.Go(fn)` instead of `wg.Add(1)` + manual goroutine wrapper
