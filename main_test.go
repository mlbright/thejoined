package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- Unit tests for sequence generators ---

func TestGenerateRepeated(t *testing.T) {
	tests := []struct {
		size int
		want string
	}{
		{0, ""},
		{4, "GUAC"},
		{8, "GUACGUAC"},
		{5, "GUACG"},
	}
	for _, tt := range tests {
		got := string(generateRepeated(tt.size))
		if got != tt.want {
			t.Errorf("generateRepeated(%d) = %q, want %q", tt.size, got, tt.want)
		}
	}
}

func TestGenerateRandom_Length(t *testing.T) {
	for _, n := range []int{0, 1, 100, 4096} {
		got := generateRandom(n)
		if len(got) != n {
			t.Errorf("generateRandom(%d) length = %d", n, len(got))
		}
	}
}

func TestGenerateRandom_ValidChars(t *testing.T) {
	buf := generateRandom(10000)
	for i, b := range buf {
		if !isValidRNA(b) {
			t.Errorf("generateRandom: invalid byte %q at index %d", b, i)
		}
	}
}

// --- Unit tests for isValidRNA ---

func TestIsValidRNA(t *testing.T) {
	valid := []byte{'G', 'U', 'A', 'C'}
	invalid := []byte{'T', 'X', ' ', 'g', 'u', 'a', 'c', 0, '\n'}
	for _, b := range valid {
		if !isValidRNA(b) {
			t.Errorf("isValidRNA(%q) = false, want true", b)
		}
	}
	for _, b := range invalid {
		if isValidRNA(b) {
			t.Errorf("isValidRNA(%q) = true, want false", b)
		}
	}
}

// --- HTTP handler tests ---

func newRNARequest(t *testing.T, body string, meta StreamMetadata) *http.Request {
	t.Helper()
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, apiPath, strings.NewReader(body))
	req.Header.Set("X-RNA-Metadata", string(metaJSON))
	return req
}

func TestHandleRNA_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, apiPath, nil)
	rr := httptest.NewRecorder()
	handleRNA(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleRNA_MissingMetadata(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, apiPath, strings.NewReader("GUAC"))
	rr := httptest.NewRecorder()
	handleRNA(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleRNA_MalformedMetadata(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, apiPath, strings.NewReader("GUAC"))
	req.Header.Set("X-RNA-Metadata", "not-json")
	rr := httptest.NewRecorder()
	handleRNA(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleRNA_ValidRepeatedStream(t *testing.T) {
	seq := string(generateRepeated(4096))
	meta := StreamMetadata{StreamID: 1, Mode: "repeated", Duration: 1, Pattern: repeatedPattern}
	req := newRNARequest(t, seq, meta)
	rr := httptest.NewRecorder()
	handleRNA(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("response status = %v, want ok", resp["status"])
	}
}

func TestHandleRNA_ValidRandomStream(t *testing.T) {
	seq := string(generateRandom(4096))
	meta := StreamMetadata{StreamID: 2, Mode: "random", Duration: 1}
	req := newRNARequest(t, seq, meta)
	rr := httptest.NewRecorder()
	handleRNA(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestHandleRNA_InvalidChars(t *testing.T) {
	meta := StreamMetadata{StreamID: 3, Mode: "random", Duration: 1}
	// Inject a non-RNA character.
	req := newRNARequest(t, "GUACXGUAC", meta)
	rr := httptest.NewRecorder()
	handleRNA(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
}

func TestHandleRNA_PatternMismatch(t *testing.T) {
	// Send a valid RNA sequence but declare a different pattern than what was sent.
	meta := StreamMetadata{StreamID: 4, Mode: "repeated", Duration: 1, Pattern: "GGGG"}
	// Actual body is GUAC...; every byte except G-positions will mismatch.
	req := newRNARequest(t, string(generateRepeated(16)), meta)
	rr := httptest.NewRecorder()
	handleRNA(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
}

// --- Integration test: real client → real test server ---

func TestClientServer_RepeatedStream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(handleRNA))
	defer ts.Close()

	// Extract host and port from the test server URL.
	addr := strings.TrimPrefix(ts.URL, "http://")
	parts := strings.SplitN(addr, ":", 2)
	host, port := parts[0], parts[1]

	done := make(chan error, 1)
	go func() {
		done <- sendStream(host, port, 2, 1, "repeated")
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("sendStream error: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Error("integration test timed out")
	}
}

func TestClientServer_RandomStream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(handleRNA))
	defer ts.Close()

	addr := strings.TrimPrefix(ts.URL, "http://")
	parts := strings.SplitN(addr, ":", 2)
	host, port := parts[0], parts[1]

	done := make(chan error, 1)
	go func() {
		done <- sendStream(host, port, 2, 1, "random")
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("sendStream error: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Error("integration test timed out")
	}
}

func TestClientServer_MultipleStreams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(handleRNA))
	defer ts.Close()

	addr := strings.TrimPrefix(ts.URL, "http://")
	parts := strings.SplitN(addr, ":", 2)
	host, port := parts[0], parts[1]

	// 3 concurrent streams, each running for 2 seconds.
	streams := 3
	errs := make(chan error, streams)
	for id := 1; id <= streams; id++ {
		go func(id int) {
			errs <- sendStream(host, port, 2, id, "repeated")
		}(id)
	}

	deadline := time.After(25 * time.Second)
	for i := 0; i < streams; i++ {
		select {
		case err := <-errs:
			if err != nil {
				t.Errorf("stream error: %v", err)
			}
		case <-deadline:
			t.Error("timed out waiting for streams")
			return
		}
	}
}

// --- Helper / env tests ---

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("TEST_RNA_KEY", "hello")
	if got := envOrDefault("TEST_RNA_KEY", "default"); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
	if got := envOrDefault("TEST_RNA_UNSET_KEY", "default"); got != "default" {
		t.Errorf("got %q, want %q", got, "default")
	}
}

func TestEnvInt(t *testing.T) {
	t.Setenv("TEST_RNA_INT", "42")
	if got := envInt("TEST_RNA_INT", 0); got != 42 {
		t.Errorf("got %d, want 42", got)
	}
	if got := envInt("TEST_RNA_INT_UNSET", 99); got != 99 {
		t.Errorf("got %d, want 99", got)
	}
	t.Setenv("TEST_RNA_INT_BAD", "notanint")
	if got := envInt("TEST_RNA_INT_BAD", 7); got != 7 {
		t.Errorf("got %d, want 7", got)
	}
}

// TestHealthEndpoint verifies the /health endpoint returns 200 OK.
func TestHealthEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(apiPath, handleRNA)
	mux.HandleFunc(healthPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "OK\n")
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + healthPath)
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}
