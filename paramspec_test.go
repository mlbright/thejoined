package main

import (
	"slices"
	"testing"
)

func TestExpandRange(t *testing.T) {
	tests := []struct {
		name    string
		r       Range
		want    []int64
		wantErr bool
	}{
		{"geometric", Range{From: "1KB", To: "8KB", Step: "x2"}, []int64{1024, 2048, 4096, 8192}, false},
		{"arithmetic", Range{From: "0", To: "3KB", Step: "+1KB"}, []int64{0, 1024, 2048, 3072}, false},
		{"single step lands past to", Range{From: "1KB", To: "1KB", Step: "x2"}, []int64{1024}, false},
		{"bad factor", Range{From: "1KB", To: "8KB", Step: "x1"}, nil, true},
		{"bad increment", Range{From: "1KB", To: "8KB", Step: "+0"}, nil, true},
		{"from after to", Range{From: "8KB", To: "1KB", Step: "x2"}, nil, true},
		{"bad step syntax", Range{From: "1KB", To: "8KB", Step: "2"}, nil, true},
	}
	for _, tt := range tests {
		got, err := expandRange(tt.r)
		if tt.wantErr {
			if err == nil {
				t.Errorf("%s: expected error, got %v", tt.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tt.name, err)
			continue
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestSelectorRoundRobin(t *testing.T) {
	s, err := newSelector(&ParamSpec{Set: []string{"a", "b", "c"}}, false, 0)
	if err != nil {
		t.Fatalf("newSelector: %v", err)
	}
	var got []string
	for range 7 {
		got = append(got, s.Pick())
	}
	want := []string{"a", "b", "c", "a", "b", "c", "a"}
	if !slices.Equal(got, want) {
		t.Errorf("round-robin: got %v, want %v", got, want)
	}
}

func TestSelectorSingleValue(t *testing.T) {
	s, err := newSelector(&ParamSpec{Value: "GET"}, false, 0)
	if err != nil {
		t.Fatalf("newSelector: %v", err)
	}
	for range 3 {
		if v := s.Pick(); v != "GET" {
			t.Errorf("got %q, want GET", v)
		}
	}
}

func TestSelectorRandomSeedReproducible(t *testing.T) {
	mk := func() []string {
		s, _ := newSelector(&ParamSpec{Set: []string{"a", "b", "c", "d"}}, true, 42)
		var out []string
		for range 10 {
			out = append(out, s.Pick())
		}
		return out
	}
	if !slices.Equal(mk(), mk()) {
		t.Error("random selector with same seed should be reproducible (single-goroutine)")
	}
}

func TestSelectorValues(t *testing.T) {
	s, err := newSelector(&ParamSpec{Value: "1KB"}, false, 0)
	if err != nil {
		t.Fatalf("newSelector: %v", err)
	}
	if !slices.Equal(s.Values(), []string{"1KB"}) {
		t.Errorf("Values() = %v", s.Values())
	}
}

func TestNewSelectorErrors(t *testing.T) {
	if _, err := newSelector(&ParamSpec{}, false, 0); err == nil {
		t.Error("empty spec should error")
	}
	if _, err := newSelector(&ParamSpec{Value: "x", Set: []string{"y"}}, false, 0); err == nil {
		t.Error("two fields set should error")
	}
}
