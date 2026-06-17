package main

import (
	"slices"
	"testing"
	"time"
)

func TestRunSpecDefaultsAndValidate(t *testing.T) {
	spec := RunSpec{Target: "http://x:8080", Workers: 4, Duration: "10s"}
	got, err := spec.normalize()
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if got.verify != true {
		t.Error("verify should default to true")
	}
	if got.timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", got.timeout)
	}
	if got.duration != 10*time.Second {
		t.Errorf("duration = %v, want 10s", got.duration)
	}
	// Default payload size selector yields the 1KB default in bytes.
	if v := got.sizeSel.Values(); !slices.Equal(v, []string{"1024"}) {
		t.Errorf("default size values = %v, want [1024]", v)
	}
	if v := got.methodSel.Pick(); v != "GET" {
		t.Errorf("default method = %q, want GET", v)
	}
	if v := got.pathSel.Pick(); v != "/" {
		t.Errorf("default path = %q, want /", v)
	}
	if got.nucleotideSel != nil {
		t.Error("nucleotide selector should be nil by default (server randomizes)")
	}
}

func TestRunSpecVerifyExplicitFalse(t *testing.T) {
	f := false
	spec := RunSpec{Target: "http://x", Workers: 1, MaxRequests: 10, Verify: &f}
	got, err := spec.normalize()
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if got.verify {
		t.Error("verify should be false when explicitly set false")
	}
}

func TestRunSpecValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		spec RunSpec
	}{
		{"no target", RunSpec{Workers: 1, Duration: "5s"}},
		{"zero workers", RunSpec{Target: "http://x", Duration: "5s"}},
		{"no termination", RunSpec{Target: "http://x", Workers: 1}},
		{"bad duration", RunSpec{Target: "http://x", Workers: 1, Duration: "soon"}},
		{"bad selection", RunSpec{Target: "http://x", Workers: 1, Duration: "5s", Selection: "spiral"}},
		{"bad size spec", RunSpec{Target: "http://x", Workers: 1, Duration: "5s", PayloadSize: &ParamSpec{}}},
		{"non-http scheme", RunSpec{Target: "ftp://host", Workers: 1, Duration: "5s"}},
		{"no host", RunSpec{Target: "http://", Workers: 1, Duration: "5s"}},
		{"negative maxRequests", RunSpec{Target: "http://x", Workers: 1, MaxRequests: -5}},
		{"zero payload size", RunSpec{Target: "http://x", Workers: 1, Duration: "5s", PayloadSize: &ParamSpec{Value: "0"}}},
		{"negative payload size", RunSpec{Target: "http://x", Workers: 1, Duration: "5s", PayloadSize: &ParamSpec{Value: "-1"}}},
	}
	for _, tt := range tests {
		if _, err := tt.spec.normalize(); err == nil {
			t.Errorf("%s: expected validation error", tt.name)
		}
	}
}

func TestRunSpecSizeBytes(t *testing.T) {
	spec := RunSpec{
		Target: "http://x", Workers: 1, MaxRequests: 5,
		PayloadSize: &ParamSpec{Set: []string{"1KB", "2KB"}},
	}
	got, err := spec.normalize()
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if !slices.Equal(got.sizeBytes, []int64{1024, 2048}) {
		t.Errorf("sizeBytes = %v, want [1024 2048]", got.sizeBytes)
	}
}
