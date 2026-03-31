// Package main provides the RNA sequence streaming service.
//
// Client mode: generates an RNA sequence (G, U, A, C) and streams it to a
// remote HTTP server for a configurable duration (default 78 s). After each
// transmission the cycle repeats. Multiple concurrent streams are supported.
//
// Server mode: accepts the stream, validates that every byte is a legal RNA
// nucleotide (G, U, A, C), and – for "repeated" streams – verifies that the
// sequence matches the declared repeating pattern.
//
// Configuration is primarily via environment variables; CLI flags override:
//
//	RNA_MODE      client | server            (default: server)
//	RNA_HOST      destination IP or DNS      (required in client mode)
//	RNA_PORT      port                       (default: 8080)
//	RNA_DURATION  transmission seconds       (default: 78)
//	RNA_STREAMS   concurrent streams         (default: 1)
//	RNA_SEQ_MODE  repeated | random          (default: repeated)
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	rnaChars        = "GUAC"
	chunkSize       = 4096
	defaultPort     = "8080"
	defaultDuration = 78
	defaultStreams   = 1
	repeatedPattern = "GUAC"
	apiPath         = "/rna"
	healthPath      = "/health"
)

// StreamMetadata is sent by the client in the X-RNA-Metadata request header
// so that the server can verify the transmission.
type StreamMetadata struct {
	StreamID int    `json:"stream_id"`
	Mode     string `json:"mode"`              // "repeated" or "random"
	Duration int    `json:"duration"`          // expected duration in seconds
	Pattern  string `json:"pattern,omitempty"` // repeating pattern (repeated mode only)
}

// generateRepeated returns size bytes that repeat the canonical GUAC pattern.
func generateRepeated(size int) []byte {
	pat := []byte(repeatedPattern)
	buf := make([]byte, size)
	for i := range buf {
		buf[i] = pat[i%len(pat)]
	}
	return buf
}

// generateRandom returns size bytes chosen uniformly at random from G, U, A, C.
func generateRandom(size int) []byte {
	chars := []byte(rnaChars)
	buf := make([]byte, size)
	for i := range buf {
		buf[i] = chars[rand.Intn(len(chars))]
	}
	return buf
}

// isValidRNA reports whether b is a legal RNA nucleotide character.
func isValidRNA(b byte) bool {
	return b == 'G' || b == 'U' || b == 'A' || b == 'C'
}

// runClient starts streams concurrent HTTP streams to host:port, each sending
// RNA data for duration seconds. When all streams finish it repeats indefinitely.
func runClient(host, port string, duration, streams int, seqMode string) {
	for {
		log.Printf("starting transmission: host=%s port=%s streams=%d duration=%ds mode=%s",
			host, port, streams, duration, seqMode)
		var wg sync.WaitGroup
		for id := 1; id <= streams; id++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := sendStream(host, port, duration, id, seqMode); err != nil {
					log.Printf("stream %d error: %v", id, err)
				}
			}()
		}
		wg.Wait()
		log.Printf("transmission complete, repeating…")
	}
}

// sendStream opens a single HTTP POST to the server and writes RNA chunks for
// duration seconds using chunked transfer encoding.
func sendStream(host, port string, duration, streamID int, seqMode string) error {
	url := fmt.Sprintf("http://%s:%s%s", host, port, apiPath)

	meta := StreamMetadata{
		StreamID: streamID,
		Mode:     seqMode,
		Duration: duration,
	}
	if seqMode == "repeated" {
		meta.Pattern = repeatedPattern
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	pr, pw := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(duration+30)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, pr)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-RNA-Metadata", string(metaJSON))
	req.ContentLength = -1 // force chunked transfer encoding

	// Write RNA chunks until the duration expires, then close the pipe.
	deadline := time.Now().Add(time.Duration(duration) * time.Second)
	go func() {
		defer pw.Close()
		for time.Now().Before(deadline) {
			var chunk []byte
			if seqMode == "random" {
				chunk = generateRandom(chunkSize)
			} else {
				chunk = generateRepeated(chunkSize)
			}
			if _, err := pw.Write(chunk); err != nil {
				return
			}
		}
	}()

	client := &http.Client{Timeout: time.Duration(duration+30) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	log.Printf("stream %d: %s – %s", streamID, resp.Status, body)
	return nil
}

// runServer starts the HTTP validation server and blocks.
func runServer(port string) {
	mux := http.NewServeMux()
	mux.HandleFunc(apiPath, handleRNA)
	mux.HandleFunc(healthPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})
	log.Printf("server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// handleRNA is the HTTP handler for incoming RNA streams. It reads the full
// body and validates every byte. For "repeated" streams it also checks that
// each byte matches the declared pattern at the correct position.
func handleRNA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metaHeader := r.Header.Get("X-RNA-Metadata")
	if metaHeader == "" {
		http.Error(w, "missing X-RNA-Metadata header", http.StatusBadRequest)
		return
	}
	var meta StreamMetadata
	if err := json.Unmarshal([]byte(metaHeader), &meta); err != nil {
		http.Error(w, fmt.Sprintf("invalid metadata: %v", err), http.StatusBadRequest)
		return
	}

	start := time.Now()
	var totalBytes int64
	invalidChars := 0
	patternErrors := 0
	pat := []byte(meta.Pattern)
	buf := make([]byte, chunkSize)
	checksum := crc32.NewIEEE()

	for {
		n, err := r.Body.Read(buf)
		if n > 0 {
			checksum.Write(buf[:n])
			for i, b := range buf[:n] {
				if !isValidRNA(b) {
					invalidChars++
				}
				// For repeated mode verify each byte against the pattern position.
				if len(pat) > 0 {
					pos := (totalBytes + int64(i)) % int64(len(pat))
					if b != pat[pos] {
						patternErrors++
					}
				}
			}
			totalBytes += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("[stream %d] read error: %v", meta.StreamID, err)
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}
	}

	elapsed := time.Since(start)
	w.Header().Set("X-Payload-Checksum", fmt.Sprintf("%08x", checksum.Sum32()))

	if invalidChars > 0 || patternErrors > 0 {
		log.Printf("[stream %d] FAIL: invalidChars=%d patternErrors=%d totalBytes=%d elapsed=%.1fs mode=%s",
			meta.StreamID, invalidChars, patternErrors, totalBytes, elapsed.Seconds(), meta.Mode)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `{"status":"error","stream_id":%d,"invalid_chars":%d,"pattern_errors":%d}`,
			meta.StreamID, invalidChars, patternErrors)
		return
	}

	log.Printf("[stream %d] OK: totalBytes=%d elapsed=%.1fs mode=%s",
		meta.StreamID, totalBytes, elapsed.Seconds(), meta.Mode)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","stream_id":%d,"bytes_received":%d,"elapsed_seconds":%.1f}`,
		meta.StreamID, totalBytes, elapsed.Seconds())
}

// envOrDefault returns the value of the environment variable key, or def if
// the variable is unset or empty.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envInt returns the integer value of the environment variable key, or def on
// any error.
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func main() {
	mode := flag.String("mode", envOrDefault("RNA_MODE", "server"), "Mode: client or server")
	host := flag.String("host", envOrDefault("RNA_HOST", ""), "Destination host (client mode)")
	port := flag.String("port", envOrDefault("RNA_PORT", defaultPort), "Port")
	duration := flag.Int("duration", envInt("RNA_DURATION", defaultDuration), "Transmission duration in seconds")
	streams := flag.Int("streams", envInt("RNA_STREAMS", defaultStreams), "Number of concurrent streams")
	seqMode := flag.String("seq-mode", envOrDefault("RNA_SEQ_MODE", "repeated"), "Sequence mode: repeated or random")
	flag.Parse()

	switch *mode {
	case "client":
		if *host == "" {
			log.Fatal("client mode requires a destination host: set RNA_HOST or use -host")
		}
		runClient(*host, *port, *duration, *streams, *seqMode)
	case "server":
		runServer(*port)
	default:
		log.Fatalf("unknown mode %q: use 'client' or 'server'", *mode)
	}
}
