#!/data/data/com.termux/files/usr/bin/bash
# =============================================================================
# TermCode — build directly in Termux (natively, without cross-compiling)
# Run this script on your phone if you want to build locally
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
echo -e "${CYAN}  TermCode — Build in Termux${NC}"
echo ""

# Проверяем Go
if ! command -v go &>/dev/null; then
    info "Installing Go..."
    pkg install -y golang || err "Failed to install Go"
fi
log "Go: $(go version)"

# Проверяем что рядом со скриптом есть исходники
if [[ ! -f "${SCRIPT_DIR}/go.mod" ]]; then
    err "go.mod not found in ${SCRIPT_DIR}. Make sure you run the script from the root of the TermCode project."
fi
if [[ ! -d "${SCRIPT_DIR}/internal" ]]; then
    err "Папка internal/ not found. Make sure all project files are copied to your phone."
fi

# Копируем исходники в build dir (чтобы не загрязнять рабочую папку)
info "Copy the source code to ${BUILD_DIR}..."
mkdir -p "$BUILD_DIR"
cp -r "${SCRIPT_DIR}/." "$BUILD_DIR/"
cd "$BUILD_DIR"

# Генерируем go.sum и загружаем внешние зависимости
info "Loading dependencies (go mod tidy)..."
# GOFLAGS=-mod=mod нужен чтобы tidy не ругался на workspace
GOFLAGS=-mod=mod go mod tidy || err "go mod tidy failed. Check the internet."

# Собираем
info "We are collecting termcode..."
VERSION="dev"
COMMIT="local"

CGO_ENABLED=0 go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o "${INSTALL_DIR}/termcode" \
    ./cmd/termcode

log "termcode installed: ${INSTALL_DIR}/termcode"
echo ""
echo -e "${CYAN}Launch:${NC}"
echo "  cd <your-project>"
echo "  termcode"
echo ""
echo -e "${CYAN}Config (providers/keys):${NC}"
echo "  termcode config init"
echo "  termcode config show"
echo "  termcode config set-provider ollama --model qwen2.5-coder:7b"
echo "  termcode config set-provider openai --key sk-... --model gpt-4o-mini"
