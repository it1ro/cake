// Package gitignore реализует подмножество семантики .gitignore,
// достаточное для обхода файлового дерева:
//
//   - стек правил: корневой + вложенные .gitignore;
//   - последнее совпавшее правило выигрывает;
//   - !переопределяет предыдущее совпадение;
//   - trailing "/" матчит только директории;
//   - "**" матчит любое число сегментов;
//   - "/" в паттерне (кроме trailing) делает его якорным.
package gitignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Rule — одно правило из файла .gitignore.
type Rule struct {
	pattern  string
	negate   bool
	dirOnly  bool
	anchored bool
	base     string
}

// Matcher — стек правил, применяемых к путям относительно root.
type Matcher struct {
	root  string
	rules []Rule
}

// New создаёт пустой Matcher для корня root.
func New(root string) *Matcher {
	return &Matcher{root: root}
}

// Load читает .gitignore из директории dir и добавляет его правила
// в стек. dir — абсолютный путь. Отсутствие файла — не ошибка.
func (m *Matcher) Load(dir string) error {
	path := filepath.Join(dir, ".gitignore")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	base, err := filepath.Rel(m.root, dir)
	if err != nil {
		return err
	}
	if base == "." {
		base = ""
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		line := sc.Text()
		rule, ok := parseLine(line, base)
		if ok {
			m.rules = append(m.rules, rule)
		}
	}
	return sc.Err()
}

// Ignore сообщает, должен ли path быть исключён.
// path — относительный путь от root, в slash-форме.
func (m *Matcher) Ignore(path string, isDir bool) bool {
	ignored := false
	for _, r := range m.rules {
		if !r.matches(path, isDir) {
			continue
		}
		ignored = !r.negate
	}
	return ignored
}

func parseLine(line, base string) (Rule, bool) {
	line = strings.TrimRight(line, "\r")
	line = strings.TrimRight(line, " \t")

	if line == "" || strings.HasPrefix(line, "#") {
		return Rule{}, false
	}

	var r Rule

	if strings.HasPrefix(line, "!") {
		r.negate = true
		line = line[1:]
	}
	if strings.HasPrefix(line, "\\!") || strings.HasPrefix(line, "\\#") {
		line = line[1:]
	}

	r.base = base

	if strings.HasSuffix(line, "/") {
		r.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}

	if strings.Contains(line, "/") {
		r.anchored = true
		line = strings.TrimPrefix(line, "/")
	}

	r.pattern = line
	return r, true
}

func (r Rule) matches(path string, isDir bool) bool {
	if r.dirOnly && !isDir {
		return false
	}

	rel := path
	if r.base != "" {
		prefix := r.base + "/"
		if !strings.HasPrefix(path, prefix) {
			return false
		}
		rel = path[len(prefix):]
	}

	if r.anchored {
		return matchPattern(r.pattern, rel)
	}
	for i := 0; i <= len(rel); i++ {
		if i > 0 {
			if rel[i-1] != '/' {
				continue
			}
		}
		if matchPattern(r.pattern, rel[i:]) {
			return true
		}
	}
	return false
}

// matchPattern: * — кроме "/", ? — один символ кроме "/",
// ** — включая "/". Мемоизация добавлена, чтобы ** не был
// экспоненциальным на длинных путях.
func matchPattern(pattern, s string) bool {
	type key struct{ i, j int }
	memo := make(map[key]bool)

	var rec func(i, j int) bool
	rec = func(i, j int) bool {
		k := key{i, j}
		if v, ok := memo[k]; ok {
			return v
		}

		var result bool
		switch {
		case i == len(pattern):
			result = j == len(s)

		case strings.HasPrefix(pattern[i:], "**"):
			rest := i + 2
			if rest < len(pattern) && pattern[rest] == '/' {
				rest++
			}
			if rest == len(pattern) {
				result = true
			} else {
				for k := j; k <= len(s); k++ {
					if rec(rest, k) {
						result = true
						break
					}
				}
			}

		case pattern[i] == '*':
			// * не пересекает "/"
			next := i + 1
			for k := j; k <= len(s); k++ {
				if rec(next, k) {
					result = true
					break
				}
				if k < len(s) && s[k] == '/' {
					break
				}
			}

		case pattern[i] == '?':
			result = j < len(s) && s[j] != '/' && rec(i+1, j+1)

		case pattern[i] == '\\' && i+1 < len(pattern):
			result = j < len(s) && s[j] == pattern[i+1] && rec(i+2, j+1)

		default:
			result = j < len(s) && s[j] == pattern[i] && rec(i+1, j+1)
		}

		memo[k] = result
		return result
	}

	return rec(0, 0)
}
