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
		{"code_sample", "package main\n\nfunc main() {}\n", 7}, // 29 / 4 → 8? нет: 29+3/4 = 8. см. ниже
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// для code_sample want вычислим честно
			if tc.name == "code_sample" {
				tc.want = (len(tc.in) + 3) / 4
			}
			if got := Estimate([]byte(tc.in)); got != tc.want {
				t.Errorf("Estimate(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
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
