# syntax=docker/dockerfile:1
# Stage 1: build the binary.
# Pinned to $BUILDPLATFORM so the Go toolchain always runs natively and
# cross-compiles to $TARGETARCH — multi-arch builds never fall back to QEMU.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
RUN apk --no-cache add ca-certificates
WORKDIR /build
COPY go.mod ./
RUN go mod download
COPY *.go ./
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o rna .

# Stage 2: minimal runtime image.
# Deliberately RUN-free: any RUN here would execute on $TARGETPLATFORM and
# drag QEMU emulation back into foreign-arch builds, so the CA bundle is
# copied from the builder instead of installed with apk.
FROM alpine:3.21
# org.opencontainers.image.source is what links the GHCR package to a repo
# (and lets the repo's GITHUB_TOKEN publish it). Parameterized because the
# cPacketNetworks and mlbright mirrors each publish to their own namespace.
ARG SOURCE_REPO=cPacketNetworks/thejoined
LABEL org.opencontainers.image.source="https://github.com/${SOURCE_REPO}" \
      org.opencontainers.image.description="RNA diagnostic HTTP server and closed-loop load generator" \
      org.opencontainers.image.licenses="Apache-2.0"
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
WORKDIR /app
COPY --from=builder /build/rna .

ENV RNA_PORT=80

EXPOSE 80
ENTRYPOINT ["/app/rna"]
