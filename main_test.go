package main

import (
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// --- parseSize ---

func TestParseSize(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"100", 100, false},
		{"100B", 100, false},
		{"1KB", 1024, false},
		{"2MB", 2 * 1024 * 1024, false},
		{"1GB", 1024 * 1024 * 1024, false},
		{"1mb", 1024 * 1024, false}, // case-insensitive
		{"abc", 0, true},
		{"1TB", 0, true}, // unknown suffix treated as numeric
	}
	for _, tt := range tests {
		got, err := parseSize(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseSize(%q) expected error, got %d", tt.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSize(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// --- handler helpers ---

func get(t *testing.T, ts *httptest.Server, path string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(ts.Close)
	return ts
}

// --- response structure ---

func TestHandler_DefaultPayloadSize(t *testing.T) {
	ts := newServer(t)
	resp := get(t, ts, "/", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if int64(len(body)) != defaultPayloadSize {
		t.Errorf("body length = %d, want %d", len(body), defaultPayloadSize)
	}
}

func TestHandler_CustomPayloadSize(t *testing.T) {
	ts := newServer(t)
	tests := []struct {
		header string
		want   int64
	}{
		{"512B", 512},
		{"1KB", 1024},
		{"2MB", 2 * 1024 * 1024},
	}
	for _, tt := range tests {
		resp := get(t, ts, "/", map[string]string{payloadSizeHeader: tt.header})
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if int64(len(body)) != tt.want {
			t.Errorf("size %s: body length = %d, want %d", tt.header, len(body), tt.want)
		}
	}
}

func TestHandler_ResponseStartsWithRequestInfo(t *testing.T) {
	ts := newServer(t)
	resp := get(t, ts, "/hello?foo=bar", map[string]string{payloadSizeHeader: "512B"})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	s := string(body)

	for _, want := range []string{"Remote-Address:", "Method: GET", "URL: /hello?foo=bar"} {
		if !strings.Contains(s, want) {
			t.Errorf("body does not contain %q", want)
		}
	}
}

func TestHandler_PaddingIsGUAC(t *testing.T) {
	ts := newServer(t)
	resp := get(t, ts, "/", map[string]string{payloadSizeHeader: "4KB"})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// Find the blank line separating info from padding.
	idx := strings.Index(string(body), "\n\n")
	if idx < 0 {
		t.Fatal("could not find end of request info section")
	}
	padding := body[idx+2:]
	pattern := []byte(nucleotides)
	// The padding continues the GUAC pattern from wherever the info section ended.
	// Determine the pattern offset at the start of padding.
	offset := (idx + 2) % len(pattern)
	for i, b := range padding {
		want := pattern[(offset+i)%len(pattern)]
		if b != want {
			t.Errorf("padding[%d] = %q, want %q", i, b, want)
			break
		}
	}
}

func TestHandler_ContentLength(t *testing.T) {
	ts := newServer(t)
	resp := get(t, ts, "/", map[string]string{payloadSizeHeader: "1KB"})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	cl, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		t.Fatalf("Content-Length: %v", err)
	}
	if cl != int64(len(body)) {
		t.Errorf("Content-Length = %d, body = %d", cl, len(body))
	}
}

func TestHandler_Checksum(t *testing.T) {
	ts := newServer(t)
	resp := get(t, ts, "/", map[string]string{payloadSizeHeader: "1KB"})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	got := resp.Header.Get(checksumHeader)
	if got == "" {
		t.Fatal("missing X-Payload-Checksum header")
	}
	want := fmt.Sprintf("%08x", crc32.ChecksumIEEE(body))
	if got != want {
		t.Errorf("checksum = %q, want %q", got, want)
	}
}

func TestHandler_PayloadSmallerThanInfo(t *testing.T) {
	// If the requested size is smaller than the info section, the server
	// returns only the info section (no truncation).
	ts := newServer(t)
	resp := get(t, ts, "/", map[string]string{payloadSizeHeader: "1B"})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if !strings.Contains(string(body), "Remote-Address:") {
		t.Error("response does not contain request info")
	}
}

func TestHandler_AnyPath(t *testing.T) {
	ts := newServer(t)
	for _, path := range []string{"/", "/foo", "/a/b/c"} {
		resp := get(t, ts, path, map[string]string{payloadSizeHeader: "256B"})
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: status = %d", path, resp.StatusCode)
		}
		if !strings.Contains(string(body), "URL: "+path) {
			t.Errorf("%s: body does not echo path", path)
		}
	}
}
