package main

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// Manager holds Load runs in memory. Finished runs are retained up to maxKept
// (FIFO eviction of the oldest finished run); running runs are never evicted.
type Manager struct {
	ctx     context.Context
	maxKept int

	mu      sync.Mutex
	runs    map[string]*Run
	order   []string // insertion order, for eviction
	counter atomic.Uint64
}

func newManager(ctx context.Context, maxKept int) *Manager {
	if maxKept < 1 {
		maxKept = 1
	}
	return &Manager{ctx: ctx, maxKept: maxKept, runs: make(map[string]*Run)}
}

// Start validates the spec, registers a new run, and launches it.
func (m *Manager) Start(spec RunSpec) (*Run, error) {
	ns, err := spec.normalize()
	if err != nil {
		return nil, fmt.Errorf("invalid run spec: %w", err)
	}
	id := fmt.Sprintf("r%d", m.counter.Add(1))
	run := newRun(m.ctx, id, spec, ns, time.Now())

	m.mu.Lock()
	m.runs[id] = run
	m.order = append(m.order, id)
	m.evictLocked()
	m.mu.Unlock()

	run.start()
	return run, nil
}

// evictLocked drops the oldest finished runs until at most maxKept remain.
// Caller holds m.mu.
func (m *Manager) evictLocked() {
	for len(m.order) > m.maxKept {
		evicted := false
		for i, id := range m.order {
			r := m.runs[id]
			if r != nil && isFinished(r.State()) {
				delete(m.runs, id)
				m.order = slices.Delete(m.order, i, i+1)
				evicted = true
				break
			}
		}
		if !evicted {
			return // nothing finished yet; keep them all
		}
	}
}

func isFinished(s RunState) bool {
	return s == StateCompleted || s == StateStopped || s == StateFailed
}

// Get returns a run by ID.
func (m *Manager) Get(id string) (*Run, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.runs[id]
	return r, ok
}

// List returns runs in insertion order.
func (m *Manager) List() []*Run {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Run, 0, len(m.order))
	for _, id := range m.order {
		if r, ok := m.runs[id]; ok {
			out = append(out, r)
		}
	}
	return out
}

// Stop requests cancellation of a run. Returns false if the ID is unknown.
func (m *Manager) Stop(id string) bool {
	r, ok := m.Get(id)
	if !ok {
		return false
	}
	r.stop()
	return true
}
