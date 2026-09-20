package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// isolate отводит XDG_CONFIG_HOME в tmp, чтобы глобальный конфиг
// с реальной машины не подмешивался в тесты.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func TestLoad_NoConfig(t *testing.T) {
	isolate(t)
	got, err := Load(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}

func TestLoad_LocalDefault(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cake.toml"), `
version = 1

[default]
context-limit = "200k"
on-overflow = "fail"
`)
	got, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("want config, got nil")
	}
	if got.Resolved.ContextLimit != "200k" {
		t.Errorf("ContextLimit = %q", got.Resolved.ContextLimit)
	}
	if got.Source != "local" || got.Profile != "default" {
		t.Errorf("Source/Profile = %q/%q", got.Source, got.Profile)
	}
}

func TestLoad_ProfileInheritsDefault(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cake.toml"), `
version = 1

[default]
context-limit = "200k"
format = "xml"
exclude = ["a/**"]

[profiles.cheap]
context-limit = "32k"
exclude = ["b/**"]
`)
	got, err := Load(root, "cheap")
	if err != nil {
		t.Fatal(err)
	}
	if got.Resolved.ContextLimit != "32k" {
		t.Errorf("ContextLimit = %q, want 32k (profile overrides)",
			got.Resolved.ContextLimit)
	}
	if got.Resolved.Format != "xml" {
		t.Errorf("Format = %q, want xml (inherited)", got.Resolved.Format)
	}
	want := "a/**,b/**"
	if strings.Join(got.Resolved.Exclude, ",") != want {
		t.Errorf("Exclude = %v, want %v (appended)",
			got.Resolved.Exclude, want)
	}
}

func TestLoad_ReserveNotInheritedWhenLimitOverridden(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cake.toml"), `
version = 1

[default]
context-limit = "200k"
reserve = "20000"

[profiles.small]
context-limit = "32k"
`)
	got, err := Load(root, "small")
	if err != nil {
		t.Fatal(err)
	}
	if got.Resolved.Reserve != "" {
		t.Errorf("Reserve = %q, want empty (не наследуется)", got.Resolved.Reserve)
	}
}

func TestLoad_ReserveInheritedWhenLimitUnchanged(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cake.toml"), `
version = 1

[default]
context-limit = "200k"
reserve = "20000"

[profiles.same]
format = "markdown"
`)
	got, err := Load(root, "same")
	if err != nil {
		t.Fatal(err)
	}
	if got.Resolved.Reserve != "20000" {
		t.Errorf("Reserve = %q, want 20000", got.Resolved.Reserve)
	}
}

func TestLoad_UnknownProfile(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cake.toml"), `
version = 1

[default]
context-limit = "200k"
`)
	_, err := Load(root, "nope")
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("want error mentioning profile name, got %v", err)
	}
}

func TestLoad_UnknownKey(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cake.toml"), `
version = 1

[default]
context_limit = "200k"
`)
	if _, err := Load(root, ""); err == nil {
		t.Fatal("want error for unknown key")
	}
}

func TestLoad_VersionMismatch(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cake.toml"), `
version = 2

[default]
context-limit = "200k"
`)
	_, err := Load(root, "")
	if err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("want version error, got %v", err)
	}
}

func TestLoad_SearchUpward(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cake.toml"), `
version = 1

[default]
context-limit = "200k"
`)
	sub := filepath.Join(root, "svc", "a")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Load(sub, "")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("want config found в родителе")
	}
}

func TestLoad_StopsAtGitRoot(t *testing.T) {
	isolate(t)
	// cake.toml в родителе, .git в промежуточной папке.
	outer := t.TempDir()
	writeFile(t, filepath.Join(outer, "cake.toml"), `
version = 1

[default]
context-limit = "200k"
`)
	repo := filepath.Join(outer, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(repo, "svc")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Load(sub, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("не должны пересекать .git; got %+v", got)
	}
}

func TestLoad_GlobalFallback(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	writeFile(t, filepath.Join(tmp, "cake", "config.toml"), `
version = 1

[default]
context-limit = "128k"
`)
	got, err := Load(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Source != "global" {
		t.Fatalf("want global config, got %+v", got)
	}
	if got.Resolved.ContextLimit != "128k" {
		t.Errorf("ContextLimit = %q", got.Resolved.ContextLimit)
	}
}

func TestLoad_LocalOverridesGlobal(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	writeFile(t, filepath.Join(tmp, "cake", "config.toml"), `
version = 1

[default]
context-limit = "128k"
format = "xml"
`)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cake.toml"), `
version = 1

[default]
context-limit = "200k"
`)
	got, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Resolved.ContextLimit != "200k" {
		t.Errorf("ContextLimit = %q, want local override",
			got.Resolved.ContextLimit)
	}
	if got.Resolved.Format != "xml" {
		t.Errorf("Format = %q, want inherited from global",
			got.Resolved.Format)
	}
}
