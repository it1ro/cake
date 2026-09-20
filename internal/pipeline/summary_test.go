package pipeline

import (
	"bytes"
	"strings"
	"testing"

	"github.com/it1ro/cake/pkg/types"
)

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1 << 20, "1.0 MB"},
		{1<<20 + 500, "1.0 MB"},
	}
	for _, tc := range tests {
		if got := humanBytes(tc.n); got != tc.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestHumanTokens(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "~0 tok"},
		{500, "~500 tok"},
		{9999, "~9999 tok"},
		{10000, "~10k tok"},
		{18432, "~18k tok"},
	}
	for _, tc := range tests {
		if got := humanTokens(tc.n); got != tc.want {
			t.Errorf("humanTokens(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestHumanTokensCompact(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{500, "500"},
		{999, "999"},
		{1000, "1k"},
		{3200, "3.2k"},
		{171000, "171k"},
		{180000, "180k"},
		{200000, "200k"},
	}
	for _, tc := range tests {
		if got := humanTokensCompact(tc.n); got != tc.want {
			t.Errorf("humanTokensCompact(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// bytes.Buffer — не tty → summary печатается без ANSI.
func TestWriteSummary_Plain(t *testing.T) {
	ctx := types.Context{
		Project: "p",
		Tokens:  500,
		Files:   make([]types.ProcessedFile, 3),
	}
	var buf bytes.Buffer
	writeSummary(&buf, Options{}, ctx, 2048)
	got := buf.String()

	if strings.Contains(got, "\x1b[") {
		t.Errorf("не-TTY writer не должен получать цвета: %q", got)
	}
	for _, want := range []string{"✓", "copied", "3 files", "~500 tok", "2.0 KB", "clipboard"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

func TestWriteSummary_Dropped(t *testing.T) {
	ctx := types.Context{Tokens: 100, Dropped: 2}
	var buf bytes.Buffer
	writeSummary(&buf, Options{}, ctx, 100)
	if !strings.Contains(buf.String(), "dropped 2") {
		t.Errorf("dropped not shown: %q", buf.String())
	}
}

func TestWriteSummary_NoDropped(t *testing.T) {
	ctx := types.Context{Tokens: 100, Dropped: 0}
	var buf bytes.Buffer
	writeSummary(&buf, Options{}, ctx, 100)
	if strings.Contains(buf.String(), "dropped") {
		t.Errorf("dropped не должен появляться при 0: %q", buf.String())
	}
}

func TestWriteSummary_WithLimit(t *testing.T) {
	opts := Options{
		ContextLimit: 200_000,
		Reserve:      20_000, // ceiling = 180k
	}
	ctx := types.Context{
		Project: "p",
		Tokens:  100_000, // 100k / 180k = 55%
		Files:   make([]types.ProcessedFile, 42),
	}
	var buf bytes.Buffer
	writeSummary(&buf, opts, ctx, 234*1024)
	got := buf.String()

	for _, want := range []string{
		"42 files",
		"≈100k / 180k (55%)",
		"limit 200k",
		"clipboard",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "мало запаса") {
		t.Errorf("на 55%% предупреждения быть не должно: %q", got)
	}
}

func TestWriteSummary_LowReserveWarns(t *testing.T) {
	opts := Options{
		ContextLimit: 200_000,
		Reserve:      20_000, // ceiling = 180k
	}
	ctx := types.Context{
		Project: "p",
		Tokens:  171_000, // 95% от 180k
		Files:   make([]types.ProcessedFile, 42),
	}
	var buf bytes.Buffer
	writeSummary(&buf, opts, ctx, 234*1024)
	got := buf.String()

	if !strings.Contains(got, "≈171k / 180k (95%)") {
		t.Errorf("ratio not shown: %q", got)
	}
	if !strings.Contains(got, "мало запаса на ответ") {
		t.Errorf("warning not shown: %q", got)
	}
}

func TestWriteSummary_OSC52Warn(t *testing.T) {
	opts := Options{ContextLimit: 200_000, Reserve: 20_000}
	ctx := types.Context{Tokens: 100_000, Files: make([]types.ProcessedFile, 42)}
	var buf bytes.Buffer
	writeSummary(&buf, opts, ctx, 101*1024) // > 100 KB
	got := buf.String()

	if !strings.Contains(got, "OSC 52") {
		t.Errorf("OSC 52 warning not shown: %q", got)
	}
}
