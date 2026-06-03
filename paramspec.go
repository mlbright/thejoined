package main

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// ParamSpec expresses one modulatable parameter's value-space. Exactly one of
// the three fields is set. Range is only meaningful for payload size.
type ParamSpec struct {
	Value string   `json:"value,omitempty"` // single fixed value
	Set   []string `json:"set,omitempty"`   // explicit set, walked in order
	Range *Range   `json:"range,omitempty"` // numeric range (payload size only)
}

// Range is a numeric size range. Step is "xN" (geometric, factor N>1) or "+SIZE"
// (arithmetic, increment SIZE>0). Bounds use the same suffixes as X-Payload-Size.
type Range struct {
	From string `json:"from"`
	To   string `json:"to"`
	Step string `json:"step"`
}

const maxRangeValues = 4096 // guard against runaway ranges

// expandRange turns a Range into the concrete ascending list of byte counts.
func expandRange(r Range) ([]int64, error) {
	from, err := parseSize(r.From)
	if err != nil {
		return nil, fmt.Errorf("range from %q: %w", r.From, err)
	}
	to, err := parseSize(r.To)
	if err != nil {
		return nil, fmt.Errorf("range to %q: %w", r.To, err)
	}
	if from > to {
		return nil, fmt.Errorf("range from (%d) must be <= to (%d)", from, to)
	}

	var out []int64
	switch {
	case strings.HasPrefix(r.Step, "x") || strings.HasPrefix(r.Step, "X"):
		factor, err := strconv.ParseInt(r.Step[1:], 10, 64)
		if err != nil || factor < 2 {
			return nil, fmt.Errorf("geometric step %q must be xN with N>=2", r.Step)
		}
		for v := from; v <= to; v *= factor {
			out = append(out, v)
			if len(out) > maxRangeValues {
				return nil, fmt.Errorf("range expands to more than %d values", maxRangeValues)
			}
		}
	case strings.HasPrefix(r.Step, "+"):
		inc, err := parseSize(r.Step[1:])
		if err != nil || inc < 1 {
			return nil, fmt.Errorf("arithmetic step %q must be +SIZE with SIZE>=1", r.Step)
		}
		for v := from; v <= to; v += inc {
			out = append(out, v)
			if len(out) > maxRangeValues {
				return nil, fmt.Errorf("range expands to more than %d values", maxRangeValues)
			}
		}
	default:
		return nil, fmt.Errorf("step %q must start with 'x' or '+'", r.Step)
	}
	return out, nil
}

// Selector yields the next concrete value for one parameter across the request
// stream. Implementations are safe for concurrent use by all workers.
type Selector interface {
	Pick() string
	Values() []string
}

type roundRobin struct {
	values []string
	n      atomic.Uint64
}

func (r *roundRobin) Pick() string {
	i := r.n.Add(1) - 1
	return r.values[i%uint64(len(r.values))]
}

func (r *roundRobin) Values() []string { return r.values }

type randomSel struct {
	values []string
	mu     sync.Mutex
	rng    *rand.Rand
}

func (r *randomSel) Pick() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.values[r.rng.IntN(len(r.values))]
}

func (r *randomSel) Values() []string { return r.values }

// specValues returns the concrete value list for a spec. Range is expanded to
// decimal byte-count strings (payload size only).
func specValues(spec *ParamSpec) ([]string, error) {
	set := 0
	if spec.Value != "" {
		set++
	}
	if len(spec.Set) > 0 {
		set++
	}
	if spec.Range != nil {
		set++
	}
	if set != 1 {
		return nil, fmt.Errorf("param spec must set exactly one of value/set/range (got %d)", set)
	}
	switch {
	case spec.Value != "":
		return []string{spec.Value}, nil
	case len(spec.Set) > 0:
		return slicesClone(spec.Set), nil
	default:
		bytes, err := expandRange(*spec.Range)
		if err != nil {
			return nil, err
		}
		out := make([]string, len(bytes))
		for i, b := range bytes {
			out[i] = strconv.FormatInt(b, 10)
		}
		return out, nil
	}
}

func slicesClone(s []string) []string {
	out := make([]string, len(s))
	copy(out, s)
	return out
}

// newSelector builds a Selector for a spec. When random is true, seed seeds a
// reproducible PRNG (reproducibility holds for single-goroutine pick order).
func newSelector(spec *ParamSpec, random bool, seed int64) (Selector, error) {
	values, err := specValues(spec)
	if err != nil {
		return nil, err
	}
	if random {
		return &randomSel{values: values, rng: rand.New(rand.NewPCG(uint64(seed), uint64(seed)))}, nil
	}
	return &roundRobin{values: values}, nil
}
