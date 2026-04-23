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
docker run -p 8080:8080 mlbright/thejoined
```

### Build from source

Requires Go 1.26 or later.

```sh
git clone https://github.com/mlbright/thejoined.git
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
| `RNA_PORT` | `8080` | Listening port |

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
