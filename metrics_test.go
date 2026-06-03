package main

import (
	"sync"
	"testing"
	"time"
)

func TestHistogramPercentile(t *testing.T) {
	var h histogram
	// 100 samples, ~1ms each.
	for range 100 {
		h.record(1 * time.Millisecond)
	}
	if h.count != 100 {
		t.Fatalf("count = %d, want 100", h.count)
	}
	p50 := h.percentile(0.50)
	// 1ms = 1_000_000ns; bucket upper bound is the next power of two (2^20 = 1048576ns).
	if p50 < 1*time.Millisecond || p50 > 2*time.Millisecond {
		t.Errorf("p50 = %v, want ~1-2ms", p50)
	}
}

func TestHistogramEmpty(t *testing.T) {
	var h histogram
	if got := h.percentile(0.99); got != 0 {
		t.Errorf("empty percentile = %v, want 0", got)
	}
}

func TestHistogramMinMax(t *testing.T) {
	var h histogram
	h.record(5 * time.Millisecond)
	h.record(1 * time.Millisecond)
	h.record(50 * time.Millisecond)
	if h.min != uint64(1*time.Millisecond) {
		t.Errorf("min = %d", h.min)
	}
	if h.max != uint64(50*time.Millisecond) {
		t.Errorf("max = %d", h.max)
	}
}

func TestMetricsRecordAndSnapshot(t *testing.T) {
	m := newMetrics([]int64{1024, 4096}, time.Unix(0, 0))

	m.reserve()
	m.beginRequest()
	m.reserve()
	m.beginRequest()
	m.recordResult(1024, 200, 1024, 2*time.Millisecond)
	m.recordResult(4096, 200, 4096, 3*time.Millisecond)
	m.recordResult(4096, 500, 4096, 4*time.Millisecond)
	m.recordVerifyFailure("checksum", 4096, "crc mismatch")
	m.recordTransportError()

	snap := m.snapshot(time.Unix(0, 0).Add(time.Second))

	if snap.Sent != 2 {
		t.Errorf("Sent = %d, want 2", snap.Sent)
	}
	if snap.Completed != 3 {
		t.Errorf("Completed = %d, want 3", snap.Completed)
	}
	if snap.TransportErrors != 1 {
		t.Errorf("TransportErrors = %d, want 1", snap.TransportErrors)
	}
	if snap.VerifyFailures["checksum"] != 1 {
		t.Errorf("VerifyFailures[checksum] = %d, want 1", snap.VerifyFailures["checksum"])
	}
	if len(snap.FailureSamples) != 1 {
		t.Errorf("FailureSamples = %d, want 1", len(snap.FailureSamples))
	}
	if len(snap.BySize) != 2 {
		t.Fatalf("BySize buckets = %d, want 2", len(snap.BySize))
	}

	var b4k *SizeMetrics
	for i := range snap.BySize {
		if snap.BySize[i].Size == 4096 {
			b4k = &snap.BySize[i]
		}
	}
	if b4k == nil {
		t.Fatal("missing 4096 bucket")
	}
	if b4k.Requests != 2 {
		t.Errorf("4096 requests = %d, want 2", b4k.Requests)
	}
	if b4k.Status[200] != 1 || b4k.Status[500] != 1 {
		t.Errorf("4096 status = %v", b4k.Status)
	}
	if b4k.BytesPerSec <= 0 {
		t.Errorf("4096 BytesPerSec = %v, want > 0", b4k.BytesPerSec)
	}
	if snap.InFlight != -2 {
		t.Errorf("InFlight = %d, want -2", snap.InFlight)
	}
}

func TestMetricsConcurrent(t *testing.T) {
	m := newMetrics([]int64{1024}, time.Unix(0, 0))
	var wg sync.WaitGroup
	for range 200 {
		wg.Go(func() {
			m.reserve()
			m.beginRequest()
			m.recordResult(1024, 200, 1024, time.Millisecond)
			m.recordVerifyFailure("checksum", 1024, "x")
		})
	}
	wg.Wait()
	snap := m.snapshot(time.Unix(0, 0).Add(time.Second))
	if snap.Sent != 200 {
		t.Errorf("Sent = %d, want 200", snap.Sent)
	}
	if snap.Completed != 200 {
		t.Errorf("Completed = %d, want 200", snap.Completed)
	}
	if snap.InFlight != 0 {
		t.Errorf("InFlight = %d, want 0", snap.InFlight)
	}
	if len(snap.BySize) != 1 || snap.BySize[0].Requests != 200 {
		t.Errorf("bucket requests wrong: %+v", snap.BySize)
	}
}

func TestMetricsUnknownSizeBucketIgnored(t *testing.T) {
	m := newMetrics([]int64{1024}, time.Unix(0, 0))
	// Recording an un-pre-allocated size must not panic; it is dropped.
	m.recordResult(9999, 200, 9999, time.Millisecond)
	snap := m.snapshot(time.Unix(0, 0).Add(time.Second))
	if len(snap.BySize) != 1 {
		t.Errorf("BySize = %d, want 1", len(snap.BySize))
	}
}
