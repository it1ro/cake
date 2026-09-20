package pipeline

import (
	"bytes"
	"errors"
	"io"
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

// При --clipboard без --output в stdout не должно уходить ничего:
// escape-последовательность OSC 52 не должна сама попасть в пайп.
func TestRunWith_Clipboard_NoStdout(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	files, err := Plan(Options{Root: root, UseGitignore: false, MaxSize: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = RunWith(Options{
			Root:         root,
			Mode:         ModeDump,
			Format:       render.FormatXML,
			UseGitignore: false,
			MaxSize:      1 << 20,
			Clipboard:    true,
			Summary:      io.Discard,
		}, files)
		w.Close()
	}()

	var buf bytes.Buffer
	io.Copy(&buf, r)
	<-done

	if buf.Len() != 0 {
		t.Errorf("при --clipboard stdout должен быть пуст, got %d bytes:\n%s",
			buf.Len(), buf.String())
	}
}

// ─── Таблица состояний 2.2 (review §10) ─────────────────────────────

// TestOverflow_StateTable покрывает все шесть строк таблицы 2.2:
// комбинации limit × budget × on-overflow.
//
// Файлы специально крупные (4000 байт = ~1000 токенов), чтобы
// переполнение срабатывало в каждом кейсе с limit.
func TestOverflow_StateTable(t *testing.T) {
	mkFiles := func(t *testing.T) string {
		root := t.TempDir()
		body := strings.Repeat("x", 4000)
		for _, n := range []string{"a.go", "b.go", "c.go"} {
			if err := os.WriteFile(
				filepath.Join(root, n), []byte(body), 0o644,
			); err != nil {
				t.Fatal(err)
			}
		}
		return root
	}

	tests := []struct {
		name       string
		limit      int
		budget     int
		onOverflow OverflowMode
		wantErr    bool
		wantDrop   bool
	}{
		// 1. limit нет, budget нет: никаких проверок.
		{"no_limit_no_budget", 0, 0, OverflowFail, false, false},
		// 2. limit нет, budget N: жёсткая обрезка, без OverflowError.
		{"budget_only", 0, 1500, OverflowFail, false, true},
		// 3. limit L, fail: превышение → OverflowError, exit 3.
		{"limit_fail", 1000, 0, OverflowFail, true, false},
		// 4. limit L, drop: обрезка до потолка, exit 0.
		{"limit_drop", 1000, 0, OverflowDrop, false, true},
		// 5. limit L + budget N (> потолка), fail: всё равно
		//    OverflowError, потому что авторитетная проверка идёт
		//    по набору до бюджетного фильтра.
		{"limit_and_budget_fail", 1000, 1500, OverflowFail, true, false},
		// 6. limit L + budget N (> потолка), drop: потолок =
		//    min(budget, L−reserve) = 900; дополнительно warning.
		{"limit_and_budget_drop", 1000, 1500, OverflowDrop, false, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := mkFiles(t)
			out := filepath.Join(t.TempDir(), "out.xml")
			opts := Options{
				Root:         root,
				Output:       out,
				Format:       render.FormatXML,
				Mode:         ModeDump,
				UseGitignore: false,
				MaxSize:      1 << 20,
				ContextLimit: tc.limit,
				Reserve:      100, // явно, чтобы не зависеть от DefaultReserve
				Budget:       tc.budget,
				OnOverflow:   tc.onOverflow,
				Report:       io.Discard,
			}
			err := Run(opts)

			var oe *OverflowError
			gotErr := errors.As(err, &oe)
			if gotErr != tc.wantErr {
				t.Fatalf("err = %v, want OverflowError=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				// В fail не должно быть ни байта вывода.
				if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
					t.Errorf("fail: output file must not exist")
				}
				return
			}
			data, err := os.ReadFile(out)
			if err != nil {
				t.Fatalf("read output: %v", err)
			}
			hasDropped := strings.Contains(string(data), `dropped="`)
			if hasDropped != tc.wantDrop {
				t.Errorf("dropped attr = %v, want %v\n%s",
					hasDropped, tc.wantDrop, data)
			}
		})
	}
}

// Строка 6 таблицы: budget > L − reserve — предупреждение в stderr.
func TestOverflow_BudgetOverLimitWarns(t *testing.T) {
	root := t.TempDir()
	body := strings.Repeat("x", 4000)
	for _, n := range []string{"a.go", "b.go", "c.go"} {
		if err := os.WriteFile(filepath.Join(root, n), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var stderr bytes.Buffer
	out := filepath.Join(t.TempDir(), "out.xml")
	_ = Run(Options{
		Root:         root,
		Output:       out,
		Format:       render.FormatXML,
		Mode:         ModeDump,
		UseGitignore: false,
		MaxSize:      1 << 20,
		ContextLimit: 1000,
		Reserve:      100,
		Budget:       5000, // > 900
		OnOverflow:   OverflowDrop,
		Report:       &stderr,
	})
	if !strings.Contains(stderr.String(), "--budget") {
		t.Errorf("warn expected, got: %q", stderr.String())
	}
}

// dump pre-flight: переполнение обнаруживается до чтения файлов.
// AbsPath указывает на несуществующий файл — RunWith всё равно
// должен вернуть OverflowError, а не упасть на I/O.
func TestOverflow_DumpPreflight_NoReads(t *testing.T) {
	root := t.TempDir()
	body := strings.Repeat("x", 8000)
	if err := os.WriteFile(
		filepath.Join(root, "big.go"), []byte(body), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	files, err := Plan(Options{Root: root, UseGitignore: false, MaxSize: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	// Портим AbsPath: файл существует, но RunWith не должен его
	// читать, потому что pre-flight срабатывает раньше I/O.
	files[0].AbsPath = "/definitely/does/not/exist"

	err = RunWith(Options{
		Root:         root,
		Mode:         ModeDump,
		Format:       render.FormatXML,
		UseGitignore: false,
		MaxSize:      1 << 20,
		ContextLimit: 100,
		Reserve:      10,
		OnOverflow:   OverflowFail,
		Report:       io.Discard,
	}, files)

	var oe *OverflowError
	if !errors.As(err, &oe) {
		t.Fatalf("want OverflowError before I/O, got %v", err)
	}
}
