// Package main provides an HTTP server for network diagnostics.
//
// Request metadata (remote address, method, URL, and all incoming headers) is
// echoed back as X-Request-* response headers. The response body contains only
// nucleotide padding characters to fill the requested payload size.
// By default the four nucleotides (G, U, A, C) are shuffled randomly on each
// request. Supply an X-Nucleotide-Order header with any pattern of up to 8
// characters (e.g. "AB", "UCAG", "12345678") to use it as the repeating
// sequence instead.
// The payload size is controlled by the X-Payload-Size request header using
// standard suffixes (B, K/KB, M/MB, G/GB). The default is 1 KB.
//
// Configuration via environment variable:
//
//	RNA_PORT   listening port (default: 8080)
package main

import (
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	defaultPayloadSize    = 1024 // 1 KB
	nucleotides           = "GUAC"
	payloadSizeHeader     = "X-Payload-Size"
	nucleotideOrderHeader = "X-Nucleotide-Order"
	checksumHeader        = "X-Payload-Checksum"
	chunkSize             = 32 * 1024
)

// nucleotidePattern returns the pattern to use for padding.
// If hdr is non-empty it is used as the repeating pattern, truncated to 8
// characters. Otherwise a random shuffle of "GUAC" is returned.
func nucleotidePattern(hdr string) []byte {
	hdr = strings.TrimSpace(hdr)
	if hdr != "" {
		if len(hdr) > 8 {
			hdr = hdr[:8]
		}
		return []byte(hdr)
	}
	p := []byte(nucleotides)
	rand.Shuffle(len(p), func(i, j int) { p[i], p[j] = p[j], p[i] })
	return p
}

// parseSize parses a human-readable size string with an optional suffix
// (B, K/KB, M/MB, G/GB). With no suffix the value is treated as bytes.
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	suffixes := []struct {
		suffix string
		mult   int64
	}{
		{"GB", 1 << 30},
		{"MB", 1 << 20},
		{"KB", 1 << 10},
		{"G", 1 << 30},
		{"M", 1 << 20},
		{"K", 1 << 10},
		{"B", 1},
	}
	for _, e := range suffixes {
		if strings.HasSuffix(s, e.suffix) {
			n, err := strconv.ParseInt(strings.TrimSuffix(s, e.suffix), 10, 64)
			if err != nil {
				return 0, err
			}
			return n * e.mult, nil
		}
	}
	return strconv.ParseInt(s, 10, 64)
}

// setRequestHeaders copies incoming request metadata to the response as
// X-Request-* headers so the body can be pure padding.
func setRequestHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Request-Remote-Addr", r.RemoteAddr)
	w.Header().Set("X-Request-Method", r.Method)
	w.Header().Set("X-Request-Url", r.URL.String())
	for name, values := range r.Header {
		key := "X-Request-" + name
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
}

// writePadding writes size bytes of the repeating pattern to w, starting at
// startOffset within the pattern so the stream is seamlessly continuous.
func writePadding(w io.Writer, startOffset, size int64, pattern []byte) error {
	if size <= 0 {
		return nil
	}
	patLen := int64(len(pattern))
	chunk := make([]byte, chunkSize)
	written := int64(0)
	for written < size {
		toWrite := min(int64(len(chunk)), size-written)
		for i := range toWrite {
			chunk[i] = pattern[(startOffset+written+int64(i))%patLen]
		}
		if _, err := w.Write(chunk[:toWrite]); err != nil {
			return err
		}
		written += toWrite
	}
	return nil
}

// computeChecksum returns the CRC32/IEEE checksum of the response padding
// without buffering it.
func computeChecksum(size int64, pattern []byte) uint32 {
	h := crc32.NewIEEE()
	writePadding(h, 0, size, pattern) //nolint: errcheck — hash.Hash.Write never errors
	return h.Sum32()
}

// handler serves every request with the request info section followed by
// nucleotide padding to fill the desired payload size.
func handler(w http.ResponseWriter, r *http.Request) {
	targetSize := int64(defaultPayloadSize)
	if s := r.Header.Get(payloadSizeHeader); s != "" {
		if n, err := parseSize(s); err == nil && n > 0 {
			targetSize = n
		}
	}

	pattern := nucleotidePattern(r.Header.Get(nucleotideOrderHeader))

	log.Printf("%s %s %s payload=%d pattern=%s", r.RemoteAddr, r.Method, r.URL, targetSize, pattern)

	setRequestHeaders(w, r)
	w.Header().Set(checksumHeader, fmt.Sprintf("%08x", computeChecksum(targetSize, pattern)))
	w.Header().Set("Content-Length", strconv.FormatInt(targetSize, 10))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	writePadding(w, 0, targetSize, pattern) //nolint: errcheck — client disconnect is non-fatal
}

func main() {
	port := os.Getenv("RNA_PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)

	log.Printf("listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
