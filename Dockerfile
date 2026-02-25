# syntax=docker/dockerfile:1
# Stage 1: build the binary
FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o rna .

# Stage 2: minimal runtime image
FROM alpine:3.21
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/rna .

# Default to server mode; override with RNA_MODE=client + RNA_HOST=<host>.
ENV RNA_MODE=server
ENV RNA_PORT=8080
ENV RNA_DURATION=78
ENV RNA_STREAMS=1
ENV RNA_SEQ_MODE=repeated

EXPOSE 8080
ENTRYPOINT ["/app/rna"]
