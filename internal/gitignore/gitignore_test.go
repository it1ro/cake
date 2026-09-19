package gitignore

import (
	"os"
	"path/filepath"
	"testing"
)

// helper: создаёт matcher, читая .gitignore из root.
func newTestMatcher(t *testing.T, root string) *Matcher {
	t.Helper()
	m := New(root)
	if err := m.Load(root); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSimpleGlob(t *testing.T) {
	root := t.TempDir()
	writeGitignore(t, root, "*.log\n")

	m := newTestMatcher(t, root)

	cases := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"app.log", false, true},
		{"logs/app.log", false, true}, // без "/" матчит на любом уровне
		{"app.txt", false, false},
		{"app.log", true, true}, // dirOnly=false, матчит директорию с таким именем
	}
	for _, c := range cases {
		if got := m.Ignore(c.path, c.isDir); got != c.want {
			t.Errorf("Ignore(%q, isDir=%v) = %v, want %v", c.path, c.isDir, got, c.want)
		}
	}
}

func TestAnchoredPattern(t *testing.T) {
	root := t.TempDir()
	writeGitignore(t, root, "/build\n")

	m := newTestMatcher(t, root)

	if !m.Ignore("build", true) {
		t.Error("anchored /build should match top-level build/")
	}
	if m.Ignore("sub/build", true) {
		t.Error("anchored /build should NOT match sub/build/")
	}
}

func TestDirOnly(t *testing.T) {
	root := t.TempDir()
	writeGitignore(t, root, "vendor/\n")

	m := newTestMatcher(t, root)

	if !m.Ignore("vendor", true) {
		t.Error("vendor/ should match dir vendor")
	}
	if m.Ignore("vendor", false) {
		t.Error("vendor/ should NOT match file named vendor")
	}
}

func TestNegation(t *testing.T) {
	root := t.TempDir()
	writeGitignore(t, root, "*.log\n!keep.log\n")

	m := newTestMatcher(t, root)

	if !m.Ignore("debug.log", false) {
		t.Error("*.log should ignore debug.log")
	}
	if m.Ignore("keep.log", false) {
		t.Error("!keep.log should un-ignore keep.log")
	}
}

func TestLastMatchWins(t *testing.T) {
	root := t.TempDir()
	writeGitignore(t, root, "foo\n!foo\nfoo\n")

	m := newTestMatcher(t, root)

	if !m.Ignore("foo", false) {
		t.Error("last match 'foo' (not negated) should win")
	}
}

func TestDoubleStar(t *testing.T) {
	root := t.TempDir()
	writeGitignore(t, root, "**/*.tmp\n")

	m := newTestMatcher(t, root)

	if !m.Ignore("a/b/c/file.tmp", false) {
		t.Error("**/*.tmp should match deeply nested")
	}
	if !m.Ignore("file.tmp", false) {
		t.Error("**/*.tmp should match at root")
	}
	if m.Ignore("file.txt", false) {
		t.Error("**/*.tmp should not match .txt")
	}
}

func TestNestedGitignore(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeGitignore(t, root, "*.log\n")
	writeGitignore(t, sub, "*.tmp\n")

	m := New(root)
	if err := m.Load(root); err != nil {
		t.Fatal(err)
	}
	if err := m.Load(sub); err != nil {
		t.Fatal(err)
	}

	if !m.Ignore("sub/a.tmp", false) {
		t.Error("sub/.gitignore should ignore sub/a.tmp")
	}
	if m.Ignore("a.tmp", false) {
		t.Error("sub/.gitignore must not leak to root")
	}
	if !m.Ignore("sub/a.log", false) {
		t.Error("root .gitignore should still apply inside sub")
	}
}

func TestCommentsAndBlankLines(t *testing.T) {
	root := t.TempDir()
	writeGitignore(t, root, "# comment\n\n   \n*.log\n")

	m := newTestMatcher(t, root)

	if !m.Ignore("a.log", false) {
		t.Error("comments and blanks must be skipped, *.log applied")
	}
}

// helper
func writeGitignore(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
