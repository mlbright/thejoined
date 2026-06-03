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
	run := newRun("test-1", spec, ns, time.Now())
	run.start(t.Context())
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
}

func TestRunDurationAndStop(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()

	spec := RunSpec{Target: ts.URL, Workers: 2, Duration: "200ms", PayloadSize: &ParamSpec{Value: "256B"}}
	ns, _ := spec.normalize()
	run := newRun("test-2", spec, ns, time.Now())
	run.start(t.Context())
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
	run2 := newRun("test-3", spec2, ns2, time.Now())
	run2.start(t.Context())
	time.Sleep(150 * time.Millisecond)
	run2.stop()
	run2.wait()
	if run2.State() != StateStopped {
		t.Errorf("state = %s, want stopped", run2.State())
	}
}
