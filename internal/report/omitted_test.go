package report

import (
	"strings"
	"testing"
)

// Базовый контракт: сумма Files по всем OmittedDir равна числу
// входных items. Это то, на что опирается pipeline, когда
// печатает и dropped="N", и <omitted count="N">.
func TestOmitted_CoversAllInput(t *testing.T) {
	items := []Item{
		{Path: "internal/a.go", Tokens: 100},
		{Path: "internal/b.go", Tokens: 200},
		{Path: "cmd/cake/main.go", Tokens: 150},
		{Path: "vendor/k8s.io/api/types.go", Tokens: 300},
		{Path: "README.md", Tokens: 50},
	}
	got := Omitted(items)

	totalFiles := 0
	for _, o := range got {
		totalFiles += o.Files
	}
	if totalFiles != len(items) {
		t.Errorf("сумма Files = %d, want %d\n%+v",
			totalFiles, len(items), got)
	}
}

func TestOmitted_RootFilesInDot(t *testing.T) {
	got := Omitted([]Item{
		{Path: "README.md", Tokens: 50},
		{Path: "go.mod", Tokens: 30},
	})
	if len(got) != 1 || got[0].Path != "./" {
		t.Fatalf("want one bucket ./, got %+v", got)
	}
	if got[0].Files != 2 || got[0].Tokens != 80 {
		t.Errorf("bucket = %+v, want {Files:2 Tokens:80}", got[0])
	}
}

func TestOmitted_CollapsesChain(t *testing.T) {
	// Цепочка vendor → k8s.io → api без ветвления должна слиться
	// в один бакет "vendor/k8s.io/api/".
	got := Omitted([]Item{
		{Path: "vendor/k8s.io/api/types.go", Tokens: 100},
		{Path: "vendor/k8s.io/api/more.go", Tokens: 200},
	})
	if len(got) != 1 {
		t.Fatalf("want 1 bucket, got %d: %+v", len(got), got)
	}
	if got[0].Path != "vendor/k8s.io/api/" {
		t.Errorf("path = %q, want vendor/k8s.io/api/", got[0].Path)
	}
	if got[0].Files != 2 || got[0].Tokens != 300 {
		t.Errorf("bucket = %+v, want {Files:2 Tokens:300}", got[0])
	}
}

// Каталог с собственным файлом и единственным подкаталогом НЕ
// схлопывается: файл «держит» ветвление.
func TestOmitted_StopsAtOwnFile(t *testing.T) {
	got := Omitted([]Item{
		{Path: "internal/x.go", Tokens: 100},
		{Path: "internal/sub/y.go", Tokens: 200},
	})
	// Ожидаем: internal/ (2 файла) — но НЕ internal/sub/.
	// Поскольку internal имеет собственный файл x.go, схлопывание
	// не может перейти в sub.
	if len(got) != 1 {
		t.Fatalf("want 1 bucket, got %+v", got)
	}
	if got[0].Path != "internal/" {
		t.Errorf("path = %q, want internal/", got[0].Path)
	}
	if got[0].Files != 2 {
		t.Errorf("Files = %d, want 2", got[0].Files)
	}
}

func TestOmitted_SortedByTokensDesc(t *testing.T) {
	got := Omitted([]Item{
		{Path: "small/a.go", Tokens: 10},
		{Path: "big/a.go", Tokens: 1000},
		{Path: "mid/a.go", Tokens: 100},
	})
	if len(got) != 3 {
		t.Fatalf("want 3 buckets, got %d", len(got))
	}
	want := []string{"big/", "mid/", "small/"}
	for i, w := range want {
		if got[i].Path != w {
			t.Errorf("bucket[%d] = %q, want %q", i, got[i].Path, w)
		}
	}
}

func TestOmitted_Empty(t *testing.T) {
	if got := Omitted(nil); got != nil {
		t.Errorf("nil → nil, got %+v", got)
	}
	if got := Omitted([]Item{}); got != nil {
		t.Errorf("empty → nil, got %+v", got)
	}
}

// Путь с trailing slash — контракт, на который смотрят рендеры.
func TestOmitted_TrailingSlash(t *testing.T) {
	got := Omitted([]Item{{Path: "internal/a.go"}})
	if len(got) != 1 {
		t.Fatal("want 1 bucket")
	}
	if !strings.HasSuffix(got[0].Path, "/") {
		t.Errorf("path %q должен оканчиваться на /", got[0].Path)
	}
	// Ровно один: не "internal//".
	if strings.HasSuffix(got[0].Path, "//") {
		t.Errorf("двойной слэш в %q", got[0].Path)
	}
}
