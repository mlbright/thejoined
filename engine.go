package main

import (
	"context"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// RunState is the lifecycle state of a Load run.
type RunState string

const (
	StatePending   RunState = "pending"
	StateRunning   RunState = "running"
	StateCompleted RunState = "completed"
	StateStopped   RunState = "stopped"
	StateFailed    RunState = "failed"
)

// Run is a single Load run: its spec, derived selectors, metrics, and lifecycle.
type Run struct {
	ID      string
	Spec    RunSpec
	Started time.Time

	ns      *normalizedSpec
	metrics *Metrics

	state    atomic.Value // RunState
	stopped  atomic.Bool  // distinguishes stop from natural completion
	cancel   context.CancelFunc
	done     chan struct{}
	doneOnce sync.Once
}

func newRun(id string, spec RunSpec, ns *normalizedSpec, started time.Time) *Run {
	r := &Run{
		ID:      id,
		Spec:    spec,
		Started: started,
		ns:      ns,
		metrics: newMetrics(ns.sizeBytes, started),
		done:    make(chan struct{}),
	}
	r.state.Store(StatePending)
	return r
}

// State returns the current lifecycle state.
func (r *Run) State() RunState { return r.state.Load().(RunState) }

// Snapshot returns the current metrics view.
func (r *Run) Snapshot() MetricsSnapshot { return r.metrics.snapshot(time.Now()) }

// wait blocks until the run finishes.
func (r *Run) wait() { <-r.done }

// stop requests cancellation; the run transitions to stopped when workers exit.
func (r *Run) stop() {
	r.stopped.Store(true)
	if r.cancel != nil {
		r.cancel()
	}
}

// newTransport builds the HTTP transport for a run. Keep-alive is off by default;
// when on, the pool is sized to the worker count so it never bottlenecks.
func newTransport(ns *normalizedSpec) *http.Transport {
	tr := &http.Transport{
		DisableKeepAlives:   !ns.keepAlive,
		ForceAttemptHTTP2:   false, // HTTP/1.1 only in v1
		MaxConnsPerHost:     ns.workers,
		MaxIdleConnsPerHost: ns.workers,
	}
	return tr
}

// buildRequest constructs the next request for a worker from the selectors.
func (r *Run) buildRequest(ctx context.Context) (*http.Request, string, error) {
	method := r.ns.methodSel.Pick()
	path := r.ns.pathSel.Pick()
	sizeStr := r.ns.sizeSel.Pick()

	req, err := http.NewRequestWithContext(ctx, method, r.ns.target+path, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set(payloadSizeHeader, sizeStr)
	if r.ns.nucleotideSel != nil {
		req.Header.Set(nucleotideOrderHeader, r.ns.nucleotideSel.Pick())
	}
	for k, v := range r.ns.headers {
		req.Header.Set(k, v)
	}
	return req, sizeStr, nil
}

// consume reads the response body. When verify is true it recomputes the CRC32
// while counting bytes; otherwise it drains without hashing. It returns the byte
// count and records any verification failures.
func (r *Run) consume(resp *http.Response, requestedSize int64) (int64, error) {
	if !r.ns.verify {
		n, err := io.Copy(io.Discard, resp.Body)
		return n, err
	}

	h := crc32.NewIEEE()
	n, err := io.Copy(h, resp.Body)
	if err != nil {
		return n, err
	}

	// length: Content-Length vs received byte count
	if resp.ContentLength >= 0 && resp.ContentLength != n {
		r.metrics.recordVerifyFailure("length", requestedSize,
			fmt.Sprintf("Content-Length=%d received=%d", resp.ContentLength, n))
	}
	// size: requested payload size vs received byte count
	if n != requestedSize {
		r.metrics.recordVerifyFailure("size", requestedSize,
			fmt.Sprintf("requested=%d received=%d", requestedSize, n))
	}
	// checksum: recomputed CRC32 vs server's X-Payload-Checksum
	if want := resp.Header.Get(checksumHeader); want != "" {
		got := fmt.Sprintf("%08x", h.Sum32())
		if got != want {
			r.metrics.recordVerifyFailure("checksum", requestedSize,
				fmt.Sprintf("server=%s computed=%s", want, got))
		}
	}
	return n, nil
}

// start launches the run's workers and a watcher that finalizes state. It
// returns immediately; callers use wait()/State()/Snapshot() to observe it.
func (r *Run) start(parent context.Context) {
	var ctx context.Context
	if r.ns.duration > 0 {
		ctx, r.cancel = context.WithTimeout(parent, r.ns.duration)
	} else {
		ctx, r.cancel = context.WithCancel(parent)
	}
	r.state.Store(StateRunning)

	client := &http.Client{Transport: newTransport(r.ns), Timeout: r.ns.timeout}

	var wg sync.WaitGroup
	for range r.ns.workers {
		wg.Go(func() { r.worker(ctx, client) })
	}

	go func() {
		wg.Wait()
		client.CloseIdleConnections()
		switch {
		case r.stopped.Load():
			r.state.Store(StateStopped)
		default:
			r.state.Store(StateCompleted)
		}
		r.doneOnce.Do(func() { close(r.done) })
	}()
}

// worker is one closed-loop request issuer: fire, await full response, repeat,
// until the context ends or the global request count limit is reached.
func (r *Run) worker(ctx context.Context, client *http.Client) {
	for {
		if ctx.Err() != nil {
			return
		}
		// Reserve a request slot under the optional count limit.
		seq := r.metrics.sent.Add(1)
		if r.ns.maxRequests > 0 && int64(seq) > r.ns.maxRequests {
			r.metrics.sent.Add(^uint64(0)) // undo the over-count reservation
			return
		}
		r.metrics.inFlight.Add(1)

		req, sizeStr, err := r.buildRequest(ctx)
		if err != nil {
			r.metrics.transportErrors.Add(1)
			r.metrics.inFlight.Add(-1)
			continue
		}
		requestedSize, _ := strconv.ParseInt(sizeStr, 10, 64)

		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			r.metrics.transportErrors.Add(1)
			r.metrics.inFlight.Add(-1)
			continue
		}
		n, cerr := r.consume(resp, requestedSize)
		resp.Body.Close()
		elapsed := time.Since(start)
		if cerr != nil {
			r.metrics.transportErrors.Add(1)
			r.metrics.inFlight.Add(-1)
			continue
		}
		r.metrics.recordResult(requestedSize, resp.StatusCode, n, elapsed)
	}
}
