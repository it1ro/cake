#!/usr/bin/env bash
#
# smoke.sh — end-to-end проверка CLI-контракта cake.
#
# Секции 1–9: exit-коды 0/1/3, поведение fail/drop, печать -v,
# профили cake.toml, отчёт --report-file. Быстрые (секунды),
# входят в `make smoke` и `make ci`.
#
# Секция 10: бенчмарк на kubernetes/kubernetes. Opt-in через
# CAKE_SMOKE_BENCH=1 — требует сеть и минуты на клонирование.
# Кэш в $XDG_CACHE_HOME/cake/bench, переклонировать —
# CAKE_SMOKE_BENCH_REFRESH=1.
#
# Три замера в секции 10:
#   1. pre-flight — Plan + Size-оценка, без чтения.
#      Ловит регрессию в walker и render.Overhead.
#   2. dump с drop — чтение всех файлов + рендер уложившегося
#      набора. Ловит регрессию в process + budget-фильтре.
#   3. raw dump — чтение всех файлов + рендер всего вывода
#      в /dev/null. Ловит регрессию в render на большом объёме.
#
# Запуск: make smoke / make bench-k8s / ./scripts/smoke.sh
#
# Не является заменой `go test` — проверяет то, что в unit-тестах
# неудобно: реальные exit-коды процесса, порядок вывода в stderr,
# содержимое XML-файла на диске.

set -euo pipefail

# ─── Настройка ────────────────────────────────────────────────────────

# BIN может прийти из make как "./bin/cake" (относительный путь
# от корня репо). Скрипт делает cd в песочницу, поэтому резолвим
# путь в абсолютный ДО этого — иначе exit 127 (command not found).
BIN="${BIN:-./bin/cake}"
BIN="$(cd "$(dirname "$BIN")" && pwd)/$(basename "$BIN")"

if [[ ! -x "$BIN" ]]; then
    echo "smoke: $BIN не найден или не исполняем. Сначала: make build" >&2
    exit 1
fi

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

PASS=0
FAIL=0

# ─── Хелперы ──────────────────────────────────────────────────────────

check_exit() {
    local name="$1" want="$2" got="$3"
    if [[ "$want" == "$got" ]]; then
        echo "  ✓ $name (exit $got)"
        PASS=$((PASS + 1))
    else
        echo "  ✗ $name: want exit $want, got $got"
        FAIL=$((FAIL + 1))
    fi
}

check_contains() {
    local name="$1" file="$2" needle="$3"
    if grep -qF -- "$needle" "$file"; then
        echo "  ✓ $name"
        PASS=$((PASS + 1))
    else
        echo "  ✗ $name: не найдено «$needle» в $file"
        echo "    --- содержимое ---"
        sed 's/^/    /' "$file" | head -20
        FAIL=$((FAIL + 1))
    fi
}

check_not_contains() {
    local name="$1" file="$2" needle="$3"
    if ! grep -qF -- "$needle" "$file"; then
        echo "  ✓ $name"
        PASS=$((PASS + 1))
    else
        echo "  ✗ $name: не ожидалось «$needle» в $file"
        FAIL=$((FAIL + 1))
    fi
}

# check_time NAME ELAPSED MAX: сравнивает секунды с порогом.
# Проверяется только при успешном exit команды — вызывающая
# сторона гейтит вызов по код возврата (иначе «0s < 10s» даст
# ложный PASS на упавшей за 0s команде).
check_time() {
    local name="$1" elapsed="$2" max="$3"
    if (( elapsed > max )); then
        echo "  ✗ $name: ${elapsed}s > ${max}s"
        FAIL=$((FAIL + 1))
    else
        echo "  ✓ $name: ${elapsed}s (≤ ${max}s)"
        PASS=$((PASS + 1))
    fi
}

# ─── Песочница ────────────────────────────────────────────────────────

echo "smoke: песочница $WORK"
echo "smoke: бинарь $BIN"
mkdir -p "$WORK/src/cmd" "$WORK/src/internal"

printf 'package main\nfunc main() {}\n' > "$WORK/src/cmd/main.go"
head -c 40000 /dev/zero | tr '\0' 'x' > "$WORK/src/internal/big.go"

cd "$WORK/src"

# ─── 1. Всё влезает, exit 0 ───────────────────────────────────────────

echo
echo "1. Всё влезает"

set +e
"$BIN" dump . --context-limit 200k -o "$WORK/full.xml" > /dev/null 2> "$WORK/stderr1.txt"
code=$?
set -e
check_exit "exit 0 без переполнения" 0 "$code"
check_contains "файл записан" "$WORK/full.xml" '<context'
check_not_contains "нет dropped" "$WORK/full.xml" 'dropped='

# ─── 2. Переполнение, fail (дефолт) → exit 3 ─────────────────────────

echo
echo "2. Переполнение, fail"

set +e
"$BIN" dump . --context-limit 100 -o "$WORK/fail.xml" > /dev/null 2> "$WORK/stderr2.txt"
code=$?
set -e
check_exit "exit 3 при fail" 3 "$code"
if [[ -e "$WORK/fail.xml" ]]; then
    echo "  ✗ файл не должен был создаться при fail"
    FAIL=$((FAIL + 1))
else
    echo "  ✓ файл не создан при fail"
    PASS=$((PASS + 1))
fi
check_contains "отчёт в stderr" "$WORK/stderr2.txt" 'не влезаем'

# ─── 3. Переполнение, drop → exit 0 ──────────────────────────────────

echo
echo "3. Переполнение, drop"

set +e
"$BIN" dump . --context-limit 100 --on-overflow=drop \
    -o "$WORK/drop.xml" > /dev/null 2> "$WORK/stderr3.txt"
code=$?
set -e
check_exit "exit 0 при drop" 0 "$code"
check_contains "файл записан" "$WORK/drop.xml" 'dropped='
check_contains "предупреждение в stderr" "$WORK/stderr3.txt" 'обрезан'
check_contains "omitted в XML" "$WORK/drop.xml" '<omitted'

# ─── 4. --report-file в fail ─────────────────────────────────────────

echo
echo "4. --report-file"

set +e
"$BIN" dump . --context-limit 100 --report-file "$WORK/report.json" \
    > /dev/null 2>&1
code=$?
set -e
check_exit "exit 3 при fail с report-file" 3 "$code"
if [[ -f "$WORK/report.json" ]]; then
    echo "  ✓ JSON-отчёт записан"
    PASS=$((PASS + 1))
    check_contains "schema: 1" "$WORK/report.json" '"schema": 1'
    check_contains "estimate" "$WORK/report.json" '"estimate"'
else
    echo "  ✗ JSON-отчёт не создан"
    FAIL=$((FAIL + 1))
fi

# ─── 5. --report-file в недоступную директорию → exit 3 ──────────────

echo
echo "5. --report-file в несуществующую директорию"

set +e
"$BIN" dump . --context-limit 100 --report-file /nope/report.json \
    > /dev/null 2> "$WORK/stderr5.txt"
code=$?
set -e
check_exit "exit 3 сохраняется" 3 "$code"
check_contains "I/O-ошибка в stderr" "$WORK/stderr5.txt" 'report-file'

# ─── 6. -v печатает путь конфига ─────────────────────────────────────

echo
echo "6. -v с cake.toml"

cat > cake.toml <<'TOML'
version = 1

[default]
context-limit = "200k"

[profiles.cheap]
context-limit = "32k"
TOML

set +e
"$BIN" dump . -v > /dev/null 2> "$WORK/stderr6.txt"
code=$?
set -e
check_exit "-v не влияет на exit" 0 "$code"
check_contains "путь конфига" "$WORK/stderr6.txt" 'config: '
check_contains "профиль default" "$WORK/stderr6.txt" '(profile default)'

# ─── 7. Профиль ───────────────────────────────────────────────────────

echo
echo "7. Профиль cheap"

set +e
"$BIN" dump . --profile cheap -v --context-limit 200k \
    > /dev/null 2> "$WORK/stderr7.txt"
code=$?
set -e
check_exit "exit 0" 0 "$code"
check_contains "профиль cheap" "$WORK/stderr7.txt" '(profile cheap)'

# ─── 8. --profile без cake.toml ───────────────────────────────────────

echo
echo "8. --profile без cake.toml"

rm -f cake.toml
set +e
"$BIN" dump . --profile cheap > /dev/null 2> "$WORK/stderr8.txt"
code=$?
set -e
check_exit "exit 1 при отсутствии конфига" 1 "$code"
check_contains "имя профиля в ошибке" "$WORK/stderr8.txt" 'cheap'

# ─── 9. --context-limit 0 отключает лимит ────────────────────────────

echo
echo "9. --context-limit 0"

cat > cake.toml <<'TOML'
version = 1

[default]
context-limit = "100"
TOML

set +e
"$BIN" dump . --context-limit 0 -o "$WORK/zero.xml" > /dev/null 2>&1
code=$?
set -e
check_exit "exit 0 (лимит отключён)" 0 "$code"
check_not_contains "нет dropped" "$WORK/zero.xml" 'dropped='

# ─── 10. Бенчмарк на kubernetes/kubernetes (opt-in) ──────────────────
#
# Не входит в дефолтный прогон: требует сеть и минуты на
# клонирование. Включается CAKE_SMOKE_BENCH=1. Кэш —
# $XDG_CACHE_HOME/cake/bench, переклонировать —
# CAKE_SMOKE_BENCH_REFRESH=1.

if [[ "${CAKE_SMOKE_BENCH:-0}" == "1" ]]; then
    echo
    echo "10. Бенчмарк на kubernetes/kubernetes"

    CACHE="${XDG_CACHE_HOME:-$HOME/.cache}/cake/bench"
    K8S="$CACHE/kubernetes"
    mkdir -p "$CACHE"

    K8S_OK=1
    if [[ ! -d "$K8S/.git" || "${CAKE_SMOKE_BENCH_REFRESH:-0}" == "1" ]]; then
        echo "  … клонирую kubernetes/kubernetes (--depth=1)"
        rm -rf "$K8S"
        set +e
        git clone --depth=1 --quiet \
            https://github.com/kubernetes/kubernetes.git "$K8S" 2>/dev/null
        clone_rc=$?
        set -e
        if [[ "$clone_rc" -ne 0 ]]; then
            echo "  ✗ клонирование не удалось (rc=$clone_rc, сеть?)"
            FAIL=$((FAIL + 1))
            K8S_OK=0
        fi
    else
        echo "  … использую кэш $K8S"
    fi

    if [[ "$K8S_OK" -eq 1 ]]; then
        pushd "$K8S" > /dev/null

        # ─── 10.1. Pre-flight: Plan + Size, без чтения ────────────────
        #
        # Единственный замер, который не читает 194 MB. Если он
        # вылезает за 10s — деградировал walker или Overhead.
        # --no-progress не обязателен (stderr не TTY → nil), но
        # оставлен для явности.
        start=$SECONDS
        set +e
        "$BIN" dump . --context-limit 200k --no-progress \
            > /dev/null 2> "$WORK/bench-pre.txt"
        code=$?
        set -e
        pre_elapsed=$((SECONDS - start))

        check_exit "pre-flight: k8s не влезает в 200k → exit 3" 3 "$code"
        check_contains "pre-flight: отчёт в stderr" "$WORK/bench-pre.txt" 'не влезаем'
        echo "  pre-flight: ${pre_elapsed}s"
        if [[ "$code" -eq 3 ]]; then
            check_time "pre-flight ≤ 10s" "$pre_elapsed" 10
        else
            echo "  … тайминг не проверяется: команда не отработала"
        fi

        # ─── 10.2. Dump с drop: чтение всех + рендер малого вывода ────
        #
        # На k8s --on-overflow=drop отдаёт ~65 файлов из
        # CHANGELOG/ (побайтовая сортировка ставит их первыми,
        # и они съедают весь бюджет). Это не баг фильтра, но
        # демонстрация того, почему в v0.3 запланировано
        # приоритетное отсечение. Здесь важно, что чтение всех
        # 25k файлов занимает секунды.
        start=$SECONDS
        set +e
        "$BIN" dump . --context-limit 200k --on-overflow=drop --no-progress \
            -o "$WORK/bench-k8s.xml" > /dev/null 2>&1
        code=$?
        set -e
        drop_elapsed=$((SECONDS - start))

        check_exit "dump с drop: exit 0" 0 "$code"
        if [[ -f "$WORK/bench-k8s.xml" ]]; then
            size=$(du -h "$WORK/bench-k8s.xml" | cut -f1)
            files=$(grep -c '<file ' "$WORK/bench-k8s.xml" || true)
            files=${files:-0}
            echo "  dump с drop: ${drop_elapsed}s, ${files} файлов, ${size}"
            check_contains "dump с drop: <omitted> в XML" \
                "$WORK/bench-k8s.xml" '<omitted'
        else
            echo "  ✗ dump с drop: файл не создан"
            FAIL=$((FAIL + 1))
        fi
        if [[ "$code" -eq 0 ]]; then
            check_time "dump с drop ≤ 15s" "$drop_elapsed" 15
        else
            echo "  … тайминг не проверяется: команда не отработала"
        fi

        # ─── 10.3. Raw dump: чтение всех + рендер всего в /dev/null ───
        #
        # Без --context-limit и без --budget: ничего не отбрасывается,
        # render сериализует все 194 MB в /dev/null. Меряет стоимость
        # render на большом выводе — то, чего не видно в 10.2, где
        # output крошечный. /dev/null не «бесплатный»: байты всё
        # равно проходят через bufio и XML-escape.
        #
        # Порог 20s — 7× от README-овых 2.6s (там NVMe, здесь
        # может быть shared-IO CI). Ловит регрессию в разы, не
        # флапает на +50%.
        start=$SECONDS
        set +e
        "$BIN" dump . --no-progress \
            -o /dev/null > /dev/null 2>&1
        code=$?
        set -e
        raw_elapsed=$((SECONDS - start))

        echo "  raw dump: ${raw_elapsed}s (все файлы, без бюджета)"
        check_exit "raw dump: exit 0" 0 "$code"
        if [[ "$code" -eq 0 ]]; then
            check_time "raw dump ≤ 20s" "$raw_elapsed" 20
        else
            echo "  … тайминг не проверяется: команда не отработала"
        fi

        popd > /dev/null
    fi
fi

# ─── Итог ─────────────────────────────────────────────────────────────

echo
echo "─────────────────────────────────────"
echo "smoke: $PASS passed, $FAIL failed"
echo "─────────────────────────────────────"

if [[ "$FAIL" -gt 0 ]]; then
    exit 1
fi
