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

// bytes.Buffer — не tty → summary печатается без ANSI.
// Это тот случай, который ловит регрессию, если isTTY сломается.
func TestWriteSummary_Plain(t *testing.T) {
	ctx := types.Context{
		Project: "p",
		Tokens:  500,
		Files:   make([]types.ProcessedFile, 3),
	}
	var buf bytes.Buffer
	writeSummary(&buf, ctx, 2048)
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
	writeSummary(&buf, ctx, 100)
	if !strings.Contains(buf.String(), "dropped 2") {
		t.Errorf("dropped not shown: %q", buf.String())
	}
}

func TestWriteSummary_NoDropped(t *testing.T) {
	ctx := types.Context{Tokens: 100, Dropped: 0}
	var buf bytes.Buffer
	writeSummary(&buf, ctx, 100)
	if strings.Contains(buf.String(), "dropped") {
		t.Errorf("dropped не должен появляться при 0: %q", buf.String())
	}
}
