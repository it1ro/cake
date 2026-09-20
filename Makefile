# cake — CLI для извлечения контекста из кодовой базы.
# Все цели предполагают GNU Make. На macOS: `brew install make`
# и вызывать `gmake`, либо использовать системный BSD make
# (часть целей может потребовать адаптации).

BINARY      := cake
CMD         := ./cmd/cake
BIN_DIR     := bin
DIST_DIR    := dist

# Версия: git describe, если есть тег; иначе — short hash; иначе — dev.
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# -s -w убирают DWARF и таблицу символов (≈ -30% размера).
# -X прокидывает версию в runtime.
LDFLAGS     := -s -w \
               -X 'main.version=$(VERSION)' \
               -X 'main.commit=$(COMMIT)' \
               -X 'main.date=$(DATE)'

GO          ?= go
GOFLAGS     ?=
GOFMT       ?= gofmt

.DEFAULT_GOAL := help

# ─── Помощь ───────────────────────────────────────────────────────────

.PHONY: help
help: ## Показать список целей
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
	     /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ─── Разработка ───────────────────────────────────────────────────────

.PHONY: build
build: ## Собрать бинарь для текущей платформы в ./bin/cake
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(CMD)

.PHONY: install
install: ## Установить бинарь в $GOBIN (или $GOPATH/bin)
	$(GO) install $(GOFLAGS) -ldflags "$(LDFLAGS)" $(CMD)

.PHONY: run
run: ## Запустить с аргументами: make run ARGS="dump ."
	$(GO) run $(CMD) $(ARGS)

# ─── Качество ─────────────────────────────────────────────────────────

.PHONY: fmt
fmt: ## Форматировать код (gofmt -s -w)
	$(GOFMT) -s -w .

.PHONY: fmt-check
fmt-check: ## Проверить форматирование (падает, если не отформатировано)
	@out=$$($(GOFMT) -s -l .); \
	if [ -n "$$out" ]; then \
		echo "Не отформатировано:"; echo "$$out"; exit 1; \
	fi

.PHONY: vet
vet: ## go vet по всем пакетам
	$(GO) vet ./...

.PHONY: lint
lint: ## Запустить golangci-lint (нужен установленный бинарь)
	@command -v golangci-lint >/dev/null || { echo "golangci-lint не найден: brew install golangci-lint"; exit 1; }
	golangci-lint run ./...

.PHONY: check
check: fmt-check vet ## Быстрые проверки без тестов

# ─── Тесты ────────────────────────────────────────────────────────────

.PHONY: test
test: ## Запустить все тесты
	$(GO) test ./...

.PHONY: test-race
test-race: ## Тесты с race-детектором
	$(GO) test -race ./...

.PHONY: test-v
test-v: ## Тесты в verbose-режиме
	$(GO) test -v ./...

.PHONY: smoke
smoke: build ## Smoke-тест: exit-коды, fail/drop, cake.toml, отчёт
	@./scripts/smoke.sh

.PHONY: bench-k8s
bench-k8s: build ## Бенчмарк на kubernetes/kubernetes (opt-in: сеть + минуты)
	@CAKE_SMOKE_BENCH=1 ./scripts/smoke.sh

.PHONY: bench-k8s-refresh
bench-k8s-refresh: build ## То же, но с переклонированием k8s
	@CAKE_SMOKE_BENCH=1 CAKE_SMOKE_BENCH_REFRESH=1 ./scripts/smoke.sh

.PHONY: cover
cover: ## Покрытие с HTML-отчётом (./coverage.html)
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -1
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Отчёт: coverage.html"

.PHONY: bench
bench: ## Бенчмарки
	$(GO) test -bench=. -benchmem ./...

.PHONY: ci
ci: check test-race build-matrix smoke ## Полный прогон перед тегом: check + test-race + build-matrix + smoke

# ─── Зависимости ──────────────────────────────────────────────────────

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

.PHONY: deps
deps: ## Скачать зависимости
	$(GO) mod download

.PHONY: tools
tools: ## Установить dev-инструменты (golangci-lint, goreleaser)
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install github.com/goreleaser/goreleaser/v2@latest

# ─── Релиз ────────────────────────────────────────────────────────────

.PHONY: release-snapshot
release-snapshot: ## Локальный прогон goreleaser без публикации
	@command -v goreleaser >/dev/null || { echo "goreleaser не найден: make tools"; exit 1; }
	goreleaser release --snapshot --clean

# ─── Очистка ──────────────────────────────────────────────────────────

.PHONY: clean
clean: ## Удалить артефакты сборки
	rm -rf $(BIN_DIR) $(DIST_DIR) coverage.out coverage.html

.PHONY: build-all
build-all: ## Кросс-сборка под linux/darwin/windows (amd64 + arm64)
	@mkdir -p $(BIN_DIR)
	GOOS=linux   GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY)-linux-amd64   $(CMD)
	GOOS=linux   GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY)-linux-arm64   $(CMD)
	GOOS=darwin  GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY)-darwin-amd64  $(CMD)
	GOOS=darwin  GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY)-darwin-arm64  $(CMD)
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY)-windows-amd64.exe $(CMD)

.PHONY: build-matrix
build-matrix: ## Быстрая проверка компиляции под все GOOS без записи бинарей
	@for os in linux darwin windows; do \
		echo "→ $$os"; \
		GOOS=$$os $(GO) build ./... || exit 1; \
	done
