// Package config читает cake.toml: локальный (вверх от Root до
// корня git-репозитория или ФС) и глобальный (XDG_CONFIG_HOME,
// затем os.UserConfigDir). Схема строгая: неизвестный ключ,
// неизвестный профиль, version != 1 — ошибка с указанием файла.
//
// Приоритет слияния (review §6.2):
//
//	CLI (Changed) > профиль > [default] (локальный) > глобальный
//	[default] > встроенные дефолты
//
// Скаляры: побеждает более приоритетный. Списки (include, exclude):
// дополняются. Единственное исключение — reserve: если профиль
// переопределяет context-limit и не задаёт reserve, резерв не
// наследуется (review §D11).
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// CurrentVersion — версия схемы, которую понимает бинарь.
// Несовпадение — ошибка: схема публичный контракт.
const CurrentVersion = 1

// File — содержимое одного cake.toml.
type File struct {
	Version  int                `toml:"version"`
	Default  Profile            `toml:"default"`
	Profiles map[string]Profile `toml:"profiles"`
}

// Profile — набор настроек. Строковые значения для context-limit
// и reserve ("200k", "10%", "128_000") — разбираются в cli, чтобы
// config не зависел от tokens/pipeline. Указатели у опциональных
// полей, чтобы отличать «не задано» от zero value.
type Profile struct {
	ContextLimit string   `toml:"context-limit"`
	Reserve      string   `toml:"reserve"`
	OnOverflow   string   `toml:"on-overflow"`
	Format       string   `toml:"format"`
	Include      []string `toml:"include"`
	Exclude      []string `toml:"exclude"`
	MaxSize      *int64   `toml:"max-size"`
	KeepDoc      *bool    `toml:"keep-doc"`
	UseGitignore *bool    `toml:"use-gitignore"`
	Budget       *int     `toml:"budget"`
}

// Loaded — что применилось.
type Loaded struct {
	Path     string  // путь применённого cake.toml
	Source   string  // "local" | "global"
	Profile  string  // имя профиля ("default" или пользовательский)
	Resolved Profile // итоговые настройки после слияния
}

// Load находит и разбирает cake.toml, применяет указанный профиль
// поверх [default]. Возвращает nil, nil, если ни локального, ни
// глобального файла нет.
func Load(root, profileName string) (*Loaded, error) {
	if profileName == "" {
		profileName = "default"
	}

	localPath := findLocal(root)
	globalPath := globalConfigPath()

	localFile, err := readOptional(localPath)
	if err != nil {
		return nil, err
	}
	globalFile, err := readOptional(globalPath)
	if err != nil {
		return nil, err
	}
	if localFile == nil && globalFile == nil {
		return nil, nil
	}

	// База: глобальный [default], поверх — локальный [default].
	var resolved Profile
	var sourcePath, sourceKind string
	if globalFile != nil {
		resolved = globalFile.Default
		sourcePath, sourceKind = globalPath, "global"
	}
	if localFile != nil {
		resolved = merge(resolved, localFile.Default, false)
		sourcePath, sourceKind = localPath, "local"
	}

	// Профиль: ищем в локальном, потом в глобальном.
	if profileName != "default" {
		var prof *Profile
		if localFile != nil {
			if p, ok := localFile.Profiles[profileName]; ok {
				prof = &p
				sourcePath, sourceKind = localPath, "local"
			}
		}
		if prof == nil && globalFile != nil {
			if p, ok := globalFile.Profiles[profileName]; ok {
				prof = &p
				sourcePath, sourceKind = globalPath, "global"
			}
		}
		if prof == nil {
			return nil, fmt.Errorf(
				"profile %q not found (searched %s and %s)",
				profileName, localPath, globalPath)
		}
		// Профиль переопределил context-limit, но не reserve —
		// reserve из [default] не наследуем (review §D11).
		dropReserve := prof.ContextLimit != "" && prof.Reserve == ""
		resolved = merge(resolved, *prof, dropReserve)
	}

	return &Loaded{
		Path:     sourcePath,
		Source:   sourceKind,
		Profile:  profileName,
		Resolved: resolved,
	}, nil
}

func readOptional(path string) (*File, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseStrict(path, data)
}

func parseStrict(path string, data []byte) (*File, error) {
	var f File
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if f.Version != CurrentVersion {
		return nil, fmt.Errorf("%s: unsupported version %d (want %d)",
			path, f.Version, CurrentVersion)
	}
	if err := validateProfile(f.Default, path, "[default]"); err != nil {
		return nil, err
	}
	for name, p := range f.Profiles {
		if err := validateProfile(p, path, "[profiles."+name+"]"); err != nil {
			return nil, err
		}
	}
	return &f, nil
}

func validateProfile(p Profile, path, where string) error {
	if p.OnOverflow != "" &&
		p.OnOverflow != "fail" && p.OnOverflow != "drop" {
		return fmt.Errorf("%s: %s.on-overflow: want fail|drop, got %q",
			path, where, p.OnOverflow)
	}
	if p.Format != "" {
		switch p.Format {
		case "xml", "markdown", "plain":
		default:
			return fmt.Errorf("%s: %s.format: want xml|markdown|plain, got %q",
				path, where, p.Format)
		}
	}
	return nil
}

// merge накладывает overlay поверх base. Скаляры: побеждает overlay,
// если задан. Списки: дополняются. Если dropReserve — база теряет
// reserve (см. review §D11).
func merge(base, overlay Profile, dropReserve bool) Profile {
	out := base
	if overlay.ContextLimit != "" {
		out.ContextLimit = overlay.ContextLimit
	}
	if overlay.Reserve != "" {
		out.Reserve = overlay.Reserve
	} else if dropReserve {
		out.Reserve = ""
	}
	if overlay.OnOverflow != "" {
		out.OnOverflow = overlay.OnOverflow
	}
	if overlay.Format != "" {
		out.Format = overlay.Format
	}
	if len(overlay.Include) > 0 {
		out.Include = append(out.Include, overlay.Include...)
	}
	if len(overlay.Exclude) > 0 {
		out.Exclude = append(out.Exclude, overlay.Exclude...)
	}
	if overlay.MaxSize != nil {
		out.MaxSize = overlay.MaxSize
	}
	if overlay.KeepDoc != nil {
		out.KeepDoc = overlay.KeepDoc
	}
	if overlay.UseGitignore != nil {
		out.UseGitignore = overlay.UseGitignore
	}
	if overlay.Budget != nil {
		out.Budget = overlay.Budget
	}
	return out
}

// findLocal идёт вверх от root, возвращает первый найденный
// cake.toml. Останавливается на корне git-репозитория (.git) или
// ФС. Пустая строка — не найдено.
func findLocal(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	dir := abs
	for {
		candidate := filepath.Join(dir, "cake.toml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// globalConfigPath: $XDG_CONFIG_HOME/cake/config.toml или
// os.UserConfigDir()/cake/config.toml.
func globalConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "cake", "config.toml")
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "cake", "config.toml")
	}
	return ""
}
