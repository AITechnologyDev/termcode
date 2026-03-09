#!/data/data/com.termux/files/usr/bin/bash
# =============================================================================
# TermCode — сборка прямо в Termux (нативно, без кросс-компиляции)
# Запускай этот скрипт на телефоне если хочешь собрать локально
# =============================================================================
set -euo pipefail

GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log()  { echo -e "${GREEN}[✓]${NC} $*"; }
info() { echo -e "${CYAN}[→]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
err()  { echo -e "${RED}[✗]${NC} $*"; exit 1; }

BUILD_DIR="${HOME}/termcode-build"
INSTALL_DIR="${PREFIX}/bin"
# Директория где лежит этот скрипт (рядом должны быть go.mod, cmd/, internal/)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo ""
echo -e "${CYAN}  TermCode — сборка в Termux${NC}"
echo ""

# Проверяем Go
if ! command -v go &>/dev/null; then
    info "Устанавливаем Go..."
    pkg install -y golang || err "Не удалось установить Go"
fi
log "Go: $(go version)"

# Проверяем что рядом со скриптом есть исходники
if [[ ! -f "${SCRIPT_DIR}/go.mod" ]]; then
    err "go.mod не найден в ${SCRIPT_DIR}. Убедись что запускаешь скрипт из корня проекта TermCode."
fi
if [[ ! -d "${SCRIPT_DIR}/internal" ]]; then
    err "Папка internal/ не найдена. Убедись что все файлы проекта скопированы на телефон."
fi

# Копируем исходники в build dir (чтобы не загрязнять рабочую папку)
info "Копируем исходники в ${BUILD_DIR}..."
mkdir -p "$BUILD_DIR"
cp -r "${SCRIPT_DIR}/." "$BUILD_DIR/"
cd "$BUILD_DIR"

# Загружаем только внешние зависимости (не трогаем внутренние пакеты)
info "Загружаем зависимости..."
go mod download || err "go mod download не удался. Проверь интернет."

# Собираем
info "Собираем termcode..."
VERSION="dev"
COMMIT="local"

CGO_ENABLED=0 go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o "${INSTALL_DIR}/termcode" \
    ./cmd/termcode

log "termcode установлен: ${INSTALL_DIR}/termcode"
echo ""
echo -e "${CYAN}Запуск:${NC}"
echo "  cd <твой-проект>"
echo "  termcode"
echo ""
echo -e "${CYAN}Конфиг (провайдеры / ключи):${NC}"
echo "  termcode config init"
echo "  termcode config show"
echo "  termcode config set-provider ollama --model qwen2.5-coder:7b"
echo "  termcode config set-provider openai --key sk-... --model gpt-4o-mini"
