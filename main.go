// Package main provides an HTTP server for network diagnostics.
//
// Any request to the server is logged and echoed back in the response body,
// followed by enough G, U, A, C characters to reach the requested payload size.
// The payload size is controlled by the X-Payload-Size request header using
// standard suffixes (B, KB, MB, GB). The default is 10 MB.
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
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	defaultPayloadSize = 10 * 1024 * 1024 // 10 MB
	nucleotides        = "GUAC"
	payloadSizeHeader  = "X-Payload-Size"
	checksumHeader     = "X-Payload-Checksum"
	chunkSize          = 32 * 1024
)

// parseSize parses a human-readable size string with an optional suffix
// (B, KB, MB, GB). With no suffix the value is treated as bytes.
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	suffixes := []struct {
		suffix string
		mult   int64
	}{
		{"GB", 1 << 30},
		{"MB", 1 << 20},
		{"KB", 1 << 10},
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

// buildRequestInfo returns a formatted summary of the incoming request.
func buildRequestInfo(r *http.Request) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Remote-Address: %s\n", r.RemoteAddr)
	fmt.Fprintf(&sb, "Method: %s\n", r.Method)
	fmt.Fprintf(&sb, "URL: %s\n", r.URL.String())
	for name, values := range r.Header {
		for _, v := range values {
			fmt.Fprintf(&sb, "%s: %s\n", name, v)
		}
	}
	fmt.Fprintln(&sb)
	return sb.String()
}

// writePadding writes size bytes of repeating GUAC to w, starting at
// startOffset within the pattern so the stream is seamlessly continuous.
func writePadding(w io.Writer, startOffset, size int64) error {
	if size <= 0 {
		return nil
	}
	pattern := []byte(nucleotides)
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

// computeChecksum returns the CRC32/IEEE checksum of the full response
// payload (info section + GUAC padding) without buffering it.
func computeChecksum(info []byte, paddingSize int64) uint32 {
	h := crc32.NewIEEE()
	h.Write(info)
	writePadding(h, int64(len(info)), paddingSize) //nolint: errcheck — hash.Hash.Write never errors
	return h.Sum32()
}

// handler serves every request with the request info section followed by
// GUAC padding to fill the desired payload size.
func handler(w http.ResponseWriter, r *http.Request) {
	targetSize := int64(defaultPayloadSize)
	if s := r.Header.Get(payloadSizeHeader); s != "" {
		if n, err := parseSize(s); err == nil && n > 0 {
			targetSize = n
		}
	}

	info := []byte(buildRequestInfo(r))
	paddingSize := max(targetSize-int64(len(info)), 0)
	actualSize := int64(len(info)) + paddingSize

	log.Printf("%s %s %s payload=%d", r.RemoteAddr, r.Method, r.URL, actualSize)

	w.Header().Set(checksumHeader, fmt.Sprintf("%08x", computeChecksum(info, paddingSize)))
	w.Header().Set("Content-Length", strconv.FormatInt(actualSize, 10))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	w.Write(info)
	writePadding(w, int64(len(info)), paddingSize) //nolint: errcheck — client disconnect is non-fatal
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
