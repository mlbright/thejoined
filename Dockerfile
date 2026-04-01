# syntax=docker/dockerfile:1
# Stage 1: build the binary
FROM golang:1.26-alpine AS builder
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

ENV RNA_PORT=80

EXPOSE 80
ENTRYPOINT ["/app/rna"]
