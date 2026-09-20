package tokens

import "testing"

func TestParseCount(t *testing.T) {
	ok := map[string]int{
		"200000": 200000, "200k": 200000, "200K": 200000,
		"1m": 1000000, "128_000": 128000, "1.5k": 1500,
		" 32k ": 32000, "0": 0,
	}
	for in, want := range ok {
		got, err := ParseCount(in)
		if err != nil || got != want {
			t.Errorf("ParseCount(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "-5", "k", "1x"} {
		if _, err := ParseCount(in); err == nil {
			t.Errorf("ParseCount(%q): want error", in)
		}
	}
}

func TestParseReserve(t *testing.T) {
	if got, err := ParseReserve("10%", 200000); err != nil || got != 20000 {
		t.Errorf("10%% of 200k = %d, %v", got, err)
	}
	if got, err := ParseReserve("5000", 200000); err != nil || got != 5000 {
		t.Errorf("5000 = %d, %v", got, err)
	}
	if got, err := ParseReserve("2k", 200000); err != nil || got != 2000 {
		t.Errorf("2k = %d, %v", got, err)
	}
	for _, in := range []string{"100%", "abc%", "-1%"} {
		if _, err := ParseReserve(in, 1000); err == nil {
			t.Errorf("ParseReserve(%q): want error", in)
		}
	}
}

func TestDefaultReserve(t *testing.T) {
	tests := []struct{ limit, want int }{
		{200_000, 20_000},
		{32_000, 3_200},
		{4_096, 1_000},
		{1_000, 500},
		{1_000_000, 20_000},
	}
	for _, tc := range tests {
		if got := DefaultReserve(tc.limit); got != tc.want {
			t.Errorf("DefaultReserve(%d) = %d, want %d",
				tc.limit, got, tc.want)
		}
	}
}
