## TermCode Makefile
## Кросс-компиляция для всех платформ включая Android/Termux (aarch64)

BINARY   := termcode
MODULE   := github.com/AITechnologyDev/termcode
CMD      := ./cmd/termcode
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS  := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)"

## Дефолтная цель — сборка для текущей платформы
.PHONY: build
build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)
	@echo "✓ Собран: ./$(BINARY)"

## Сборка для Android/Termux (aarch64) — запускать на ноутбуке
.PHONY: termux
termux:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
		go build $(LDFLAGS) -o $(BINARY)-linux-arm64 $(CMD)
	@echo "✓ Собран: ./$(BINARY)-linux-arm64"
	@echo "  Скопируй на телефон:"
	@echo "  adb push $(BINARY)-linux-arm64 /sdcard/"
	@echo "  или через ssh/termux-url-opener"

## Все платформы
.PHONY: all
all: build-linux-amd64 build-linux-arm64 build-darwin build-windows

.PHONY: build-linux-amd64
build-linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
		go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 $(CMD)

.PHONY: build-linux-arm64
build-linux-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
		go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 $(CMD)

.PHONY: build-darwin
build-darwin:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 \
		go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 $(CMD)

.PHONY: build-windows
build-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
		go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe $(CMD)

## Установка зависимостей
.PHONY: deps
deps:
	go mod tidy
	go mod download

## Запуск тестов
.PHONY: test
test:
	go test ./... -v

## Линтер
.PHONY: lint
lint:
	go vet ./...
	@which staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "(staticcheck не установлен)"

## Очистка
.PHONY: clean
clean:
	rm -f $(BINARY) $(BINARY)-linux-arm64
	rm -rf dist/

## Установка в систему (для текущего пользователя)
.PHONY: install
install: build
	mkdir -p $(HOME)/.local/bin
	cp $(BINARY) $(HOME)/.local/bin/$(BINARY)
	@echo "✓ Установлен: $(HOME)/.local/bin/$(BINARY)"

## Установка в Termux (после сборки на ноутбуке и копирования на телефон)
## Запускать на телефоне в Termux:
##   bash install-termux.sh
.PHONY: termux-install-script
termux-install-script:
	@echo "Генерирую install-termux.sh..."
	@cat > install-termux.sh << 'EOF'
	#!/data/data/com.termux/files/usr/bin/bash
	set -e
	BINARY="termcode-linux-arm64"
	DEST="$$PREFIX/bin/termcode"
	if [[ ! -f "$$BINARY" ]]; then
	  echo "Файл $$BINARY не найден. Скопируй его в текущую директорию."
	  exit 1
	fi
	cp "$$BINARY" "$$DEST"
	chmod +x "$$DEST"
	echo "✓ termcode установлен: $$DEST"
	echo "  Запуск: termcode [директория]"
	EOF
	chmod +x install-termux.sh
	@echo "✓ install-termux.sh создан"

## Помощь
.PHONY: help
help:
	@echo "TermCode — цели сборки:"
	@echo ""
	@echo "  make build          Текущая платформа"
	@echo "  make termux         Linux ARM64 (для Termux/Android)"
	@echo "  make all            Все платформы → dist/"
	@echo "  make deps           Обновить зависимости"
	@echo "  make test           Тесты"
	@echo "  make lint           go vet + staticcheck"
	@echo "  make install        Установить в ~/.local/bin"
	@echo "  make clean          Очистить артефакты"
