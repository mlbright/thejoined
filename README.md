# thejoined

An HTTP server for network diagnostics and testing.

Send any request to the server and it will respond with a summary of your request (remote address, method, URL, and headers) followed by enough `G`, `U`, `A`, `C` characters to fill the desired payload size. This makes it easy to inspect what your client is actually sending, and to test how your application handles responses of varying sizes.

## How it works

1. Client sends any HTTP request, optionally including `X-Payload-Size` to control response size.
2. Server logs the request and responds with:
   - **Request info section** — remote address, method, URL, and all request headers
   - **GUAC padding** — repeating `G`, `U`, `A`, `C` characters appended until the total response reaches the requested size
3. The `X-Payload-Checksum` response header carries a CRC32/IEEE checksum of the full payload.

The default payload size is **10 MB**. The minimum payload is always at least the request info section, even if a smaller size is requested.

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

## Querying with curl

### Basic request (default 10 MB response)

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

### Inspect response headers (including checksum)

```sh
curl -s -D - -H 'X-Payload-Size: 256B' http://localhost:8080/
```

```
HTTP/1.1 200 OK
Content-Length: 256
Content-Type: text/plain; charset=utf-8
X-Payload-Checksum: 4b3a2e1f

Remote-Address: 127.0.0.1:51234
Method: GET
URL: /
X-Payload-Size: 256B

GUACGUACGUAC...
```

### Show only the request info section

```sh
curl -s -H 'X-Payload-Size: 256B' http://localhost:8080/ | head -20
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

### Send custom headers and verify they are echoed

```sh
curl -s \
  -H 'X-Payload-Size: 512B' \
  -H 'X-My-Header: hello' \
  http://localhost:8080/ | head -20
```

### POST with a body

```sh
curl -s -X POST \
  -H 'X-Payload-Size: 512B' \
  -H 'Content-Type: application/json' \
  -d '{"key":"value"}' \
  http://localhost:8080/echo
```

## License

Apache 2.0
