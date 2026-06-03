package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunCountTermination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()

	spec := RunSpec{
		Target:      ts.URL,
		Workers:     4,
		MaxRequests: 50,
		PayloadSize: &ParamSpec{Set: []string{"256B", "1KB"}},
	}
	ns, err := spec.normalize()
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	run := newRun(t.Context(), "test-1", spec, ns, time.Now())
	run.start()
	run.wait()

	if run.State() != StateCompleted {
		t.Errorf("state = %s, want completed", run.State())
	}
	snap := run.Snapshot()
	if snap.Completed != 50 {
		t.Errorf("completed = %d, want 50", snap.Completed)
	}
	if snap.TransportErrors != 0 {
		t.Errorf("transportErrors = %d, want 0", snap.TransportErrors)
	}
	if snap.VerifyFailures["checksum"] != 0 {
		t.Errorf("checksum failures = %d, want 0", snap.VerifyFailures["checksum"])
	}
	if len(snap.BySize) != 2 {
		t.Errorf("bySize = %d, want 2", len(snap.BySize))
	}
	if snap.InFlight != 0 {
		t.Errorf("inFlight = %d, want 0 after completion", snap.InFlight)
	}

	// Per-size buckets must actually populate (regression: suffixed sizes
	// like "256B" must be parsed with parseSize, not strconv.ParseInt).
	var totalBySize uint64
	for _, b := range snap.BySize {
		totalBySize += b.Requests
	}
	if totalBySize != 50 {
		t.Errorf("sum of per-size requests = %d, want 50", totalBySize)
	}
	if snap.VerifyFailures["size"] != 0 {
		t.Errorf("size verify failures = %d, want 0", snap.VerifyFailures["size"])
	}
	if snap.VerifyFailures["length"] != 0 {
		t.Errorf("length verify failures = %d, want 0", snap.VerifyFailures["length"])
	}
}

func TestRunDurationAndStop(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()

	spec := RunSpec{Target: ts.URL, Workers: 2, Duration: "200ms", PayloadSize: &ParamSpec{Value: "256B"}}
	ns, _ := spec.normalize()
	run := newRun(t.Context(), "test-2", spec, ns, time.Now())
	run.start()
	run.wait()
	if run.State() != StateCompleted {
		t.Errorf("state = %s, want completed", run.State())
	}
	if run.Snapshot().Completed == 0 {
		t.Error("expected some completed requests in 200ms")
	}

	// Now a long run we stop early.
	spec2 := RunSpec{Target: ts.URL, Workers: 2, Duration: "60s", PayloadSize: &ParamSpec{Value: "256B"}}
	ns2, _ := spec2.normalize()
	run2 := newRun(t.Context(), "test-3", spec2, ns2, time.Now())
	run2.start()
	time.Sleep(150 * time.Millisecond)
	run2.stop()
	run2.wait()
	if run2.State() != StateStopped {
		t.Errorf("state = %s, want stopped", run2.State())
	}
}

func TestRunDurationNoPhantomErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()
	spec := RunSpec{Target: ts.URL, Workers: 8, Duration: "300ms", PayloadSize: &ParamSpec{Value: "256B"}}
	ns, err := spec.normalize()
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	run := newRun(t.Context(), "no-phantom", spec, ns, time.Now())
	run.start()
	run.wait()
	snap := run.Snapshot()
	if snap.TransportErrors != 0 {
		t.Errorf("transportErrors = %d, want 0 (duration cancellation must not count as error)", snap.TransportErrors)
	}
	if snap.Completed == 0 {
		t.Error("expected some completed requests")
	}
}

func TestRunReservationUndo(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()
	// Far more workers than the request budget forces many workers through the
	// over-count undo branch; Sent must settle to exactly MaxRequests.
	spec := RunSpec{Target: ts.URL, Workers: 50, MaxRequests: 5, PayloadSize: &ParamSpec{Value: "64B"}}
	ns, err := spec.normalize()
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	run := newRun(t.Context(), "undo", spec, ns, time.Now())
	run.start()
	run.wait()
	snap := run.Snapshot()
	if snap.Completed != 5 {
		t.Errorf("completed = %d, want 5", snap.Completed)
	}
	if snap.Sent != 5 {
		t.Errorf("sent = %d, want 5 (undo must keep sent from drifting above the limit)", snap.Sent)
	}
}

func TestRunStopBeforeStart(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()
	spec := RunSpec{Target: ts.URL, Workers: 4, Duration: "60s", PayloadSize: &ParamSpec{Value: "64B"}}
	ns, _ := spec.normalize()
	run := newRun(t.Context(), "early-stop", spec, ns, time.Now())
	run.stop() // stop before start must not be lost
	run.start()
	run.wait()
	if run.State() != StateStopped {
		t.Errorf("state = %s, want stopped", run.State())
	}
	if run.Snapshot().Completed != 0 {
		t.Errorf("completed = %d, want 0 (workers should exit immediately)", run.Snapshot().Completed)
	}
}
