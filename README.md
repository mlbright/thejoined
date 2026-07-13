# thejoined

An HTTP server for network diagnostics and testing.

Send any request to the server and it will echo your request metadata back as `X-Request-*` response headers and return a body of repeating `G`, `U`, `A`, `C` characters sized to your liking. This makes it easy to inspect the conversation between client and server, and to test how your application handles responses of varying sizes.

## How it works

1. Client sends any HTTP request, optionally including `X-Payload-Size` to control response size (default: 1 KB).
2. Server logs the request and responds with:
   - **`X-Request-*` response headers** — the remote address (`X-Request-Remote-Addr`), method (`X-Request-Method`), URL (`X-Request-Url`), and every incoming request header echoed back as `X-Request-<Header-Name>`
   - **Body** — pure repeating-pattern padding (by default a randomly shuffled `GUAC`) sized exactly to the requested payload size
3. The `X-Payload-Checksum` response header carries a CRC32/IEEE checksum (8-character hex) of the response body.

The default payload size is **1 KB**. The pattern can be fixed with the `X-Nucleotide-Order` request header (any string up to 8 characters, e.g. `UCAG`); otherwise `GUAC` is shuffled randomly per request.

## Install

### Docker (recommended)

```sh
docker run -p 8080:8080 cpacketnetworks/thejoined
```

Multi-arch (amd64/arm64) images are also published to GHCR on every push to `main`:

```sh
docker run -p 8080:8080 ghcr.io/cpacketnetworks/thejoined
```

### Download a release binary

Static Linux binaries (amd64, arm64) are attached to [GitHub Releases](https://github.com/cPacketNetworks/thejoined/releases):

```sh
curl -LO https://github.com/cPacketNetworks/thejoined/releases/latest/download/rna-linux-amd64
curl -LO https://github.com/cPacketNetworks/thejoined/releases/latest/download/SHA256SUMS
sha256sum --check --ignore-missing SHA256SUMS
chmod +x rna-linux-amd64
```

### Build from source

Requires Go 1.26 or later.

```sh
git clone https://github.com/cpacketnetworks/thejoined.git
cd thejoined
go build -o rna .
./rna
```

### systemd

1. Copy the binary to `/usr/local/bin/rna`.
2. Copy `rna.service` to `/etc/systemd/system/`.

```sh
sudo cp rna /usr/local/bin/rna
sudo cp rna.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now rna
```

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `RNA_MODE` | `server` | `server` (diagnostic server) or `client` (load generator). |
| `RNA_PORT` | `8080` | Listening port — the server port, or the client control-API port. |

### Request headers

| Header | Default | Description |
|--------|---------|-------------|
| `X-Payload-Size` | `1KB` | Desired response body size. Accepts `B`, `K`/`KB`, `M`/`MB`, `G`/`GB` suffixes (e.g. `512B`, `64KB`, `5MB`, `1GB`); a bare number is treated as bytes. |
| `X-Nucleotide-Order` | random `GUAC` | Repeating padding pattern (any string, truncated to 8 characters). |

### Response headers

| Header | Description |
|--------|-------------|
| `X-Request-Remote-Addr` | Client remote address as seen by the server. |
| `X-Request-Method` | HTTP method of the request. |
| `X-Request-Url` | Request URL. |
| `X-Request-<Header>` | One entry per incoming request header. |
| `X-Payload-Checksum` | CRC32/IEEE checksum of the response body, 8-character hex. |
| `Content-Length` | Response body byte count. |
| `Content-Type` | `text/plain; charset=utf-8`. |

## Querying with curl

### Basic request (default 1 KB response)

```sh
curl http://localhost:8080/
```

### Request a specific payload size

```sh
# 512 bytes
curl -H 'X-Payload-Size: 512B' http://localhost:8080/

# 64 KB
curl -H 'X-Payload-Size: 64KB' http://localhost:8080/

# 5 MB
curl -H 'X-Payload-Size: 5MB' http://localhost:8080/

# 1 GB
curl -H 'X-Payload-Size: 1GB' http://localhost:8080/
```

### Fix the padding pattern

```sh
curl -H 'X-Nucleotide-Order: UCAG' -H 'X-Payload-Size: 32B' http://localhost:8080/
# UCAGUCAGUCAGUCAGUCAGUCAGUCAGUCAG
```

### Inspect response headers (including the echoed request and checksum)

```sh
curl -s -D - -H 'X-Payload-Size: 32B' http://localhost:8080/
```

```
HTTP/1.1 200 OK
Content-Length: 32
Content-Type: text/plain; charset=utf-8
X-Payload-Checksum: 4b3a2e1f
X-Request-Method: GET
X-Request-Remote-Addr: 127.0.0.1:51234
X-Request-Url: /
X-Request-Accept: */*
X-Request-User-Agent: curl/8.5.0
X-Request-X-Payload-Size: 32B

GUACGUACGUACGUACGUACGUACGUACGUAC
```

### Measure download throughput

```sh
curl -o /dev/null -H 'X-Payload-Size: 100MB' http://localhost:8080/
```

### Extract only the checksum

```sh
curl -s -o /dev/null -D - -H 'X-Payload-Size: 1KB' http://localhost:8080/ \
  | grep -i x-payload-checksum
# X-Payload-Checksum: a1b2c3d4
```

## Client mode (load generator)

Run the same binary as a closed-loop load generator with `RNA_MODE=client`. It exposes a REST control API (on `RNA_PORT`) for starting runs that drive traffic at an RNA server, modulating the server's parameters per request.

```sh
RNA_MODE=client RNA_PORT=9090 ./rna
```

### Start a run

`POST /runs` with a JSON body:

```sh
curl -X POST http://localhost:9090/runs -d '{
  "target": "http://server:8080",
  "workers": 8,
  "duration": "30s",
  "payloadSize": { "range": { "from": "1KB", "to": "1GB", "step": "x2" } },
  "nucleotideOrder": { "set": ["GUAC", "UCAG"] },
  "method": { "value": "GET" },
  "path": { "value": "/" }
}'
```

Returns `201` with `{ "id": "...", "state": "running", ... }`.

| Field | Default | Description |
|-------|---------|-------------|
| `target` | — (required) | Server base URL (http/https, must include host). |
| `workers` | — (required) | Concurrent closed-loop workers. |
| `duration` | — | Go duration (e.g. `30s`). Run ends when it elapses. |
| `maxRequests` | — | Total request cap across all workers. |
| `payloadSize` / `nucleotideOrder` / `method` / `path` | size `1KB`, order random, method `GET`, path `/` | Parameter specs (see below). |
| `headers` | none | Fixed extra headers attached to every request. |
| `selection` | `round-robin` | `round-robin` or `random`. |
| `seed` | `0` | Seed for `random` selection. |
| `keepAlive` | `false` | Reuse connections (fresh connection per request when false). |
| `timeoutMs` | `5000` | Per-request timeout. |
| `verify` | `true` | Verify size / Content-Length / `X-Payload-Checksum` per response. |

At least one of `duration` or `maxRequests` is required.

A **parameter spec** sets exactly one of: `{"value":"1KB"}` (fixed), `{"set":["1KB","1MB"]}` (round-robined), or `{"range":{"from":"1KB","to":"1GB","step":"x2"}}` (payload size only; `step` is `xN` geometric or `+SIZE` arithmetic).

### Inspect, list, and stop

```sh
curl http://localhost:9090/runs/<id>       # status + live/final metrics (bucketed by payload size)
curl http://localhost:9090/runs            # list runs
curl -X POST http://localhost:9090/runs/<id>/stop   # stop a running run
```

Metrics are reported per payload size: request count, bytes, bytes/sec, status distribution, and latency percentiles (p50/p90/p99). Runs are kept in memory only.
