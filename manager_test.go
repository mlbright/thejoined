package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestManagerStartGetList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()
	m := newManager(t.Context(), 3)

	run, err := m.Start(RunSpec{Target: ts.URL, Workers: 2, MaxRequests: 10, PayloadSize: &ParamSpec{Value: "256B"}})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if run.ID == "" {
		t.Error("run ID should be set")
	}
	got, ok := m.Get(run.ID)
	if !ok || got.ID != run.ID {
		t.Error("Get should return the started run")
	}
	if len(m.List()) != 1 {
		t.Errorf("List len = %d, want 1", len(m.List()))
	}
	run.wait()
}

func TestManagerStartValidationError(t *testing.T) {
	m := newManager(t.Context(), 3)
	if _, err := m.Start(RunSpec{Workers: 1}); err == nil {
		t.Error("expected validation error for missing target")
	}
}

func TestManagerRetentionEvictsFinished(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()
	m := newManager(t.Context(), 2)

	var ids []string
	for range 4 {
		run, err := m.Start(RunSpec{Target: ts.URL, Workers: 1, MaxRequests: 3, PayloadSize: &ParamSpec{Value: "64B"}})
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		run.wait()
		ids = append(ids, run.ID)
	}
	if len(m.List()) > 2 {
		t.Errorf("List len = %d, want <= 2 (retention)", len(m.List()))
	}
	// The two oldest finished runs should have been evicted.
	if _, ok := m.Get(ids[0]); ok {
		t.Error("oldest run should have been evicted")
	}
	if _, ok := m.Get(ids[3]); !ok {
		t.Error("newest run should be retained")
	}
}

func TestManagerStopUnknown(t *testing.T) {
	m := newManager(t.Context(), 2)
	if m.Stop("nope") {
		t.Error("Stop on unknown id should return false")
	}
}
