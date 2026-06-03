package main

import (
	"cmp"
	"math/bits"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// histogram is a power-of-two latency histogram over nanoseconds. Bucket i holds
// samples whose nanosecond value has bit-length i, i.e. in [2^(i-1), 2^i). It is
// not safe for concurrent use; callers guard it with the owning sizeBucket mutex.
type histogram struct {
	buckets [64]uint64
	count   uint64
	min     uint64
	max     uint64
}

func (h *histogram) record(d time.Duration) {
	n := uint64(d)
	if d < 0 {
		n = 0
	}
	idx := bits.Len64(n) // 0 for n==0, else 1..63
	h.buckets[idx]++
	if h.count == 0 || n < h.min {
		h.min = n
	}
	if n > h.max {
		h.max = n
	}
	h.count++
}

// percentile returns the upper bound of the bucket containing the p-th
// percentile sample (p in [0,1]). Returns 0 for an empty histogram.
func (h *histogram) percentile(p float64) time.Duration {
	if h.count == 0 {
		return 0
	}
	target := uint64(p * float64(h.count))
	if target == 0 {
		target = 1
	}
	var cum uint64
	for idx, c := range h.buckets {
		cum += c
		if cum >= target {
			if idx == 0 {
				return 0
			}
			return time.Duration(uint64(1) << idx) // bucket upper bound in ns
		}
	}
	return time.Duration(h.max)
}

const maxFailureSamples = 20

type sizeBucket struct {
	mu       sync.Mutex
	requests uint64
	bytes    uint64
	status   map[int]uint64
	hist     histogram
}

// FailureSample is a captured verification failure for diagnostics.
type FailureSample struct {
	Kind   string `json:"kind"`
	Size   int64  `json:"size"`
	Detail string `json:"detail"`
}

// Metrics accumulates the results of a Load run. Global counters are atomic; the
// per-size buckets are pre-allocated from the known payload-size value-space so
// the hot path never mutates the map.
type Metrics struct {
	sent            atomic.Uint64
	completed       atomic.Uint64
	inFlight        atomic.Int64
	transportErrors atomic.Uint64

	buckets map[int64]*sizeBucket

	mu             sync.Mutex
	verifyFailures map[string]uint64
	samples        []FailureSample

	started time.Time
}

func newMetrics(sizes []int64, started time.Time) *Metrics {
	buckets := make(map[int64]*sizeBucket, len(sizes))
	for _, s := range sizes {
		buckets[s] = &sizeBucket{status: make(map[int]uint64)}
	}
	return &Metrics{
		buckets:        buckets,
		verifyFailures: make(map[string]uint64),
		started:        started,
	}
}

// reserve claims a request slot, returning the 1-based sequence number. Used to
// enforce the optional request-count limit before the request is actually sent.
func (m *Metrics) reserve() uint64 { return m.sent.Add(1) }

// unreserve releases a slot claimed by reserve when it exceeds the count limit.
func (m *Metrics) unreserve() { m.sent.Add(^uint64(0)) }

// beginRequest marks a reserved request as now in flight.
func (m *Metrics) beginRequest() { m.inFlight.Add(1) }

func (m *Metrics) recordTransportError() {
	m.transportErrors.Add(1)
	m.inFlight.Add(-1)
}

func (m *Metrics) recordResult(size int64, status int, n int64, d time.Duration) {
	m.completed.Add(1)
	m.inFlight.Add(-1)
	b := m.buckets[size]
	if b == nil {
		return // size outside the pre-allocated value-space; drop
	}
	b.mu.Lock()
	b.requests++
	b.bytes += uint64(n)
	b.status[status]++
	b.hist.record(d)
	b.mu.Unlock()
}

func (m *Metrics) recordVerifyFailure(kind string, size int64, detail string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifyFailures[kind]++
	if len(m.samples) < maxFailureSamples {
		m.samples = append(m.samples, FailureSample{Kind: kind, Size: size, Detail: detail})
	}
}

// SizeMetrics is the per-payload-size view in a snapshot.
type SizeMetrics struct {
	Size        int64          `json:"size"`
	Requests    uint64         `json:"requests"`
	Bytes       uint64         `json:"bytes"`
	BytesPerSec float64        `json:"bytesPerSec"`
	Status      map[int]uint64 `json:"status"`
	P50Ms       float64        `json:"p50Ms"`
	P90Ms       float64        `json:"p90Ms"`
	P99Ms       float64        `json:"p99Ms"`
	MinMs       float64        `json:"minMs"`
	MaxMs       float64        `json:"maxMs"`
}

// MetricsSnapshot is the JSON-serialisable view returned by the API.
type MetricsSnapshot struct {
	Sent            uint64            `json:"sent"`
	Completed       uint64            `json:"completed"`
	InFlight        int64             `json:"inFlight"`
	TransportErrors uint64            `json:"transportErrors"`
	RequestsPerSec  float64           `json:"requestsPerSec"`
	VerifyFailures  map[string]uint64 `json:"verifyFailures"`
	FailureSamples  []FailureSample   `json:"failureSamples"`
	BySize          []SizeMetrics     `json:"bySize"`
	ElapsedSec      float64           `json:"elapsedSec"`
}

func nsToMs(n uint64) float64 { return float64(n) / 1e6 }

// snapshot renders the current metrics as of time now.
func (m *Metrics) snapshot(now time.Time) MetricsSnapshot {
	elapsed := now.Sub(m.started).Seconds()
	if elapsed <= 0 {
		elapsed = 1e-9
	}
	completed := m.completed.Load()

	snap := MetricsSnapshot{
		Sent:            m.sent.Load(),
		Completed:       completed,
		InFlight:        m.inFlight.Load(),
		TransportErrors: m.transportErrors.Load(),
		RequestsPerSec:  float64(completed) / elapsed,
		VerifyFailures:  map[string]uint64{},
		ElapsedSec:      elapsed,
	}

	m.mu.Lock()
	for k, v := range m.verifyFailures {
		snap.VerifyFailures[k] = v
	}
	snap.FailureSamples = append([]FailureSample(nil), m.samples...)
	m.mu.Unlock()

	for size, b := range m.buckets {
		b.mu.Lock()
		sm := SizeMetrics{
			Size:        size,
			Requests:    b.requests,
			Bytes:       b.bytes,
			BytesPerSec: float64(b.bytes) / elapsed,
			Status:      make(map[int]uint64, len(b.status)),
			P50Ms:       nsToMs(uint64(b.hist.percentile(0.50))),
			P90Ms:       nsToMs(uint64(b.hist.percentile(0.90))),
			P99Ms:       nsToMs(uint64(b.hist.percentile(0.99))),
			MinMs:       nsToMs(b.hist.min),
			MaxMs:       nsToMs(b.hist.max),
		}
		for k, v := range b.status {
			sm.Status[k] = v
		}
		b.mu.Unlock()
		snap.BySize = append(snap.BySize, sm)
	}
	slices.SortFunc(snap.BySize, func(a, b SizeMetrics) int { return cmp.Compare(a.Size, b.Size) })
	return snap
}
