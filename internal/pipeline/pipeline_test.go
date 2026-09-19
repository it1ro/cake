package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/it1ro/cake/internal/render"
)

func TestPipeline_Budget(t *testing.T) {
	root := t.TempDir()

	// Три файла по ~400 байт → ~100 токенов каждый.
	body := strings.Repeat("x", 400) + "\n"
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Бюджет, влезающий ровно в два файла.
	out := filepath.Join(t.TempDir(), "out.xml")
	err := Run(Options{
		Root:         root,
		Output:       out,
		Format:       render.FormatXML,
		MaxSize:      1 << 20,
		UseGitignore: false,
		Budget:       210, // ~2 файла по 100 токенов + запас
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)

	if !strings.Contains(got, `dropped="1"`) {
		t.Errorf("want dropped=\"1\"\n--- got ---\n%s", got)
	}
	if strings.Count(got, "<file ") != 2 {
		t.Errorf("want 2 files, got %d\n--- got ---\n%s",
			strings.Count(got, "<file "), got)
	}
	// Убеждаемся, что не осталось tokens="0".
	if strings.Contains(got, `tokens="0"`) {
		t.Errorf("tokens not estimated:\n%s", got)
	}
}

func TestPipeline_NoBudget_KeepsAll(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(t.TempDir(), "out.xml")
	if err := Run(Options{
		Root: root, Output: out, Format: render.FormatMarkdown,
		MaxSize: 1 << 20, UseGitignore: false,
	}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	if strings.Count(string(data), "## `") != 3 {
		t.Errorf("want 3 files, got:\n%s", data)
	}
	if strings.Contains(string(data), "dropped") {
		t.Errorf("no budget — no dropped attr:\n%s", data)
	}
}
