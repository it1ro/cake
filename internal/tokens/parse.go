package tokens

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseCount разбирает число токенов: "200000", "200k", "1m",
// "128_000", "1.5k". Регистр суффикса не важен.
func ParseCount(s string) (int, error) {
	orig := s
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "")

	mult := 1.0
	switch {
	case strings.HasSuffix(s, "k"):
		mult = 1e3
		s = s[:len(s)-1]
	case strings.HasSuffix(s, "m"):
		mult = 1e6
		s = s[:len(s)-1]
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return 0, fmt.Errorf(
			"неверное число токенов %q (примеры: 200000, 200k, 1m)", orig)
	}
	return int(f*mult + 0.5), nil
}

// ParseReserve разбирает резерв: абсолютное число (как ParseCount)
// или процент от лимита ("10%").
func ParseReserve(s string, limit int) (int, error) {
	t := strings.TrimSpace(s)
	if strings.HasSuffix(t, "%") {
		p, err := strconv.ParseFloat(
			strings.TrimSpace(strings.TrimSuffix(t, "%")), 64)
		if err != nil || p < 0 || p >= 100 {
			return 0, fmt.Errorf("неверный процент резерва %q (0–99%%)", s)
		}
		return int(float64(limit) * p / 100), nil
	}
	return ParseCount(t)
}

// DefaultReserve — резерв под system prompt и ответ модели:
// clamp(limit/10, 1_000, 20_000), но не больше половины лимита.
func DefaultReserve(limit int) int {
	r := limit / 10
	if r < 1000 {
		r = 1000
	}
	if r > 20000 {
		r = 20000
	}
	if r > limit/2 {
		r = limit / 2
	}
	return r
}
