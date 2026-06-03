package main

import (
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// RunSpec is the JSON request body for POST /runs. Selectors and effective
// settings are derived by normalize() into a normalizedSpec.
type RunSpec struct {
	Target      string            `json:"target"`
	Workers     int               `json:"workers"`
	Duration    string            `json:"duration,omitempty"`    // Go duration, e.g. "30s"
	MaxRequests int64             `json:"maxRequests,omitempty"` // total across all workers
	PayloadSize *ParamSpec        `json:"payloadSize,omitempty"`
	Nucleotide  *ParamSpec        `json:"nucleotideOrder,omitempty"`
	Method      *ParamSpec        `json:"method,omitempty"`
	Path        *ParamSpec        `json:"path,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Selection   string            `json:"selection,omitempty"` // "round-robin" (default) | "random"
	Seed        int64             `json:"seed,omitempty"`
	KeepAlive   bool              `json:"keepAlive,omitempty"` // default false
	TimeoutMs   int               `json:"timeoutMs,omitempty"` // default 5000
	Verify      *bool             `json:"verify,omitempty"`    // default true
}

// normalizedSpec is the validated, selector-bearing form used by the engine.
type normalizedSpec struct {
	target        string
	workers       int
	duration      time.Duration // 0 means no duration limit
	maxRequests   int64         // 0 means no count limit
	keepAlive     bool
	timeout       time.Duration
	verify        bool
	headers       map[string]string
	sizeSel       Selector
	sizeBytes     []int64  // value-space of sizeSel, parsed to bytes (for metric buckets)
	nucleotideSel Selector // nil => omit X-Nucleotide-Order (server randomizes)
	methodSel     Selector
	pathSel       Selector
}

func (s RunSpec) normalize() (*normalizedSpec, error) {
	if s.Target == "" {
		return nil, fmt.Errorf("target is required")
	}
	if _, err := url.ParseRequestURI(s.Target); err != nil {
		return nil, fmt.Errorf("invalid target %q: %w", s.Target, err)
	}
	if s.Workers < 1 {
		return nil, fmt.Errorf("workers must be >= 1")
	}

	ns := &normalizedSpec{
		target:      s.Target,
		workers:     s.Workers,
		maxRequests: s.MaxRequests,
		keepAlive:   s.KeepAlive,
		timeout:     5 * time.Second,
		verify:      true,
		headers:     s.Headers,
	}
	if s.Verify != nil {
		ns.verify = *s.Verify
	}
	if s.TimeoutMs > 0 {
		ns.timeout = time.Duration(s.TimeoutMs) * time.Millisecond
	}
	if s.Duration != "" {
		d, err := time.ParseDuration(s.Duration)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("invalid duration %q", s.Duration)
		}
		ns.duration = d
	}
	if ns.duration == 0 && ns.maxRequests == 0 {
		return nil, fmt.Errorf("a termination condition is required: set duration, maxRequests, or both")
	}

	random := false
	switch s.Selection {
	case "", "round-robin":
		random = false
	case "random":
		random = true
	default:
		return nil, fmt.Errorf("selection must be \"round-robin\" or \"random\", got %q", s.Selection)
	}

	var err error
	// Payload size — defaults to the server's 1KB default, expressed in bytes.
	sizeSpec := s.PayloadSize
	if sizeSpec == nil {
		sizeSpec = &ParamSpec{Value: strconv.Itoa(defaultPayloadSize)}
	}
	if ns.sizeSel, err = newSelector(sizeSpec, random, s.Seed); err != nil {
		return nil, fmt.Errorf("payloadSize: %w", err)
	}
	for _, v := range ns.sizeSel.Values() {
		b, perr := parseSize(v)
		if perr != nil {
			return nil, fmt.Errorf("payloadSize value %q: %w", v, perr)
		}
		ns.sizeBytes = append(ns.sizeBytes, b)
	}

	// Nucleotide order — omitted by default so the server randomizes.
	if s.Nucleotide != nil {
		if ns.nucleotideSel, err = newSelector(s.Nucleotide, random, s.Seed); err != nil {
			return nil, fmt.Errorf("nucleotideOrder: %w", err)
		}
	}

	// Method — defaults to GET.
	methodSpec := s.Method
	if methodSpec == nil {
		methodSpec = &ParamSpec{Value: "GET"}
	}
	if ns.methodSel, err = newSelector(methodSpec, random, s.Seed); err != nil {
		return nil, fmt.Errorf("method: %w", err)
	}

	// Path — defaults to "/".
	pathSpec := s.Path
	if pathSpec == nil {
		pathSpec = &ParamSpec{Value: "/"}
	}
	if ns.pathSel, err = newSelector(pathSpec, random, s.Seed); err != nil {
		return nil, fmt.Errorf("path: %w", err)
	}

	return ns, nil
}
