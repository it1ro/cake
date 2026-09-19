package tokens

import (
	"strings"
	"testing"
)

func TestEstimate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"one_byte", "a", 1},
		{"four_bytes", "abcd", 1},
		{"five_bytes", "abcde", 2},
		{"hundred_x", strings.Repeat("x", 100), 25},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Estimate([]byte(tc.in)); got != tc.want {
				t.Errorf("Estimate(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestEstimateSize(t *testing.T) {
	tests := []struct {
		n    int64
		want int
	}{
		{0, 0},
		{-1, 0},
		{1, 1},
		{4, 1},
		{5, 2},
		{100, 25},
	}
	for _, tc := range tests {
		if got := EstimateSize(tc.n); got != tc.want {
			t.Errorf("EstimateSize(%d) = %d, want %d", tc.n, got, tc.want)
		}
	}
}

func TestEstimate_NonDecreasing(t *testing.T) {
	prev := 0
	for n := 0; n < 1000; n += 7 {
		got := Estimate([]byte(strings.Repeat("x", n)))
		if got < prev {
			t.Fatalf("Estimate not monotonic at n=%d: %d < %d", n, got, prev)
		}
		prev = got
	}
}
