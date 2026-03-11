#!/bin/bash
# publish-github.sh — первый пуш TermCode на GitHub
# Запускать из папки termcode/

set -e

REPO="AITechnologyDev/termcode"
REMOTE="https://github.com/$REPO.git"

echo ""
echo "  TermCode — публикация на GitHub"
echo ""

# Проверяем git
if ! command -v git &>/dev/null; then
    echo "[✗] git не найден. Установи: pkg install git"
    exit 1
fi

# Проверяем что мы в правильной папке
if [ ! -f "go.mod" ]; then
    echo "[✗] go.mod не найден. Запускай скрипт из папки termcode/"
    exit 1
fi

# Инициализируем репозиторий если нужно
if [ ! -d ".git" ]; then
    echo "[→] Инициализируем git репозиторий..."
    git init
    git branch -M main
fi

# .gitignore
if [ ! -f ".gitignore" ]; then
    echo "[→] Создаём .gitignore..."
    cat > .gitignore << 'EOF'
# Бинарники
termcode
termcode-linux-*
termcode-arm64

# Конфиги с секретами (не коммитим)
# ~/.config/termcode/ — это не в репозитории

# Go
vendor/
*.test

# Редакторы
.idea/
.vscode/
*.swp
EOF
fi

# Добавляем все файлы
echo "[→] Добавляем файлы..."
git add .

# Первый коммит
if git log --oneline -1 &>/dev/null 2>&1; then
    echo "[→] Коммитим изменения..."
    git commit -m "feat: update TermCode

- Multi-format tool call parser (GLM/Qwen/OpenAI compat)
- Context window management with auto-trim
- Ollama model browser + pull UI
- Command palette (Ctrl+P)
- Token speed & context usage in statusbar
- Interactive AI questions with option picker
- <think> tag filtering for reasoning models" 2>/dev/null || echo "[i] Нечего коммитить"
else
    echo "[→] Первый коммит..."
    git commit -m "feat: initial release of TermCode v0.1.0

TermCode — AI coding assistant for terminal/Termux
- BubbleTea TUI with streaming responses
- Tool use: read/write/patch files, run commands
- Multi-provider: Ollama, OpenAI, Anthropic, OpenRouter
- Native Android/ARM64 support via Termux
- Session history, context management
- Command palette (Ctrl+P)"
fi

# Добавляем remote если нет
if ! git remote get-url origin &>/dev/null 2>&1; then
    echo "[→] Добавляем remote origin..."
    git remote add origin "$REMOTE"
fi

echo ""
echo "  Готово к пушу!"
echo ""
echo "  Для публикации выполни:"
echo ""
echo "    git push -u origin main"
echo ""
echo "  Если репозиторий ещё не создан на GitHub:"
echo "    1. Зайди на https://github.com/new"
echo "    2. Имя: termcode"
echo "    3. НЕ добавляй README (он уже есть)"
echo "    4. Нажми Create repository"
echo "    5. Вернись сюда и запусти: git push -u origin main"
echo ""
