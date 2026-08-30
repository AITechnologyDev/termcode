package tui

// i18nStrings — все строки интерфейса, локализуемые через m.tr()
type i18nStrings struct {
	Placeholder      string
	StatusReady      string
	StatusGenerating string
	StatusLastTok    string
	HintSend         string
	HintNewline      string
	HintCommands     string
	HintModels       string
	HintSave         string
	HintLang         string
	Thinking         string
	LoadingModels    string
	ModelSelectTitle string
	ModelSelectHint  string
	ModelSelectCount string
	PullTitle        string
	PullInterrupt    string
	PullDone         string
	QAHint           string
	QASelected       string
	ContextDropped   string
	WelcomeMsg       string
	PaletteSearch    string
	PaletteHint      string
	SessionHint      string
	UserLabel        string
	// Palette items
	PalCmdPalette       string
	PalCmdPaletteDesc   string
	PalModels           string
	PalModelsDesc       string
	PalPull             string
	PalPullDesc         string
	PalNew              string
	PalNewDesc          string
	PalLang             string
	PalLangDesc         string
	PalSessions         string
	PalSessionsDesc     string
	PalSave             string
	PalSaveDesc         string
	PalLS               string
	PalLSDesc           string
	PalGit              string
	PalGitDesc          string
	PalBuild            string
	PalBuildDesc        string
	PalTest             string
	PalTestDesc         string
	PalCtx              string
	PalCtxDesc          string
	PalClear            string
	PalClearDesc        string
	PalProvider         string
	PalProviderDesc     string
	ProviderTitle       string
	ProviderHint        string
	PalProfile          string
	PalProfileDesc      string
	PalInstructions     string
	PalInstructionsDesc string
	ProfileTitle        string
	ProfileSaveHint     string
	InstructTitle       string
	InstructSaveHint    string
	// Session list
	SessionsTitle   string
	SessionsEmpty   string
	SessionsCount   string
	SessionsMsgs    string
	SessionsLoading string
	// Palette
	PaletteTitle string
	PaletteEmpty string
	// formatAge
	AgeJustNow string
	AgeMin     string
	AgeHour    string
	AgeDay     string
	// Retry messages
	RetryConnecting string
	RetryAttempt    string
	// Provider select
	ProviderSectionLocal string
	ProviderSectionFree  string
	ProviderSectionCloud string
	ProviderKeyMissing   string
	ProviderKeyPresent   string
	ProviderKeyLocal     string
	ProviderPickModel    string
	ProviderKeyURL       string
	// Model select
	ModelTagCloud string
	ModelTagLocal string
	ModelTagFree  string
	// Overlay
	OverlayBack string
	// Provider fields (i18n)
	FldModel string
	FldKey   string
	FldURL   string
}

var i18nEN = i18nStrings{
	Placeholder:      "Ask anything... (Enter to send, Shift+Enter for newline)",
	StatusReady:      "Ready — %d messages",
	StatusGenerating: "Generating",
	StatusLastTok:    "last: %.1f tok/s · %d tok",
	HintSend:         "send",
	HintNewline:      "newline",
	HintCommands:     "commands",
	HintModels:       "models",
	HintSave:         "save",
	HintLang:         "Ctrl+P→lang",
	Thinking:         "Thinking...",
	LoadingModels:    "Loading...",
	ModelSelectTitle: " Select Model ",
	ModelSelectHint:  "  ↑↓ navigate  Enter select  p pull  q skip\n\n",
	ModelSelectCount: "  Model %d/%d",
	PullTitle:        " Downloading Model ",
	PullInterrupt:    " — cancel",
	PullDone:         "Done!",
	QAHint:           "  Space — select  ↑↓ — navigate  Enter — confirm  Esc — cancel",
	QASelected:       "  ✓ Selected: %d",
	ContextDropped:   "context: dropped %d old messages",
	WelcomeMsg:       "  Welcome to TermCode. Ask a question or request a file change.",
	PaletteSearch:    "Type to search...",
	PaletteHint:      "  ↑↓ navigate  Enter select  Esc close",
	SessionHint:      "  ↑↓ navigate  Enter load  Backspace delete  Esc back\n\n",
	UserLabel:        "You",
	// Palette
	PalCmdPalette:        "Command Palette",
	PalCmdPaletteDesc:    "Open this palette",
	PalModels:            "Switch Model",
	PalModelsDesc:        "Show Ollama model list",
	PalPull:              "Pull Model",
	PalPullDesc:          "Enter model name to download",
	PalNew:               "New Session",
	PalNewDesc:           "Start a new chat (current will be saved)",
	PalLang:              "Switch Language / Сменить язык",
	PalLangDesc:          "EN ↔ RU — interface and AI response language",
	PalSessions:          "Load Session",
	PalSessionsDesc:      "Open list of saved chats",
	PalSave:              "Save Session",
	PalSaveDesc:          "Save chat history to disk",
	PalLS:                "Project Files",
	PalLSDesc:            "Show file tree of working directory",
	PalGit:               "Git Status",
	PalGitDesc:           "Show git status of project",
	PalBuild:             "Go Build",
	PalBuildDesc:         "Run go build ./...",
	PalTest:              "Go Test",
	PalTestDesc:          "Run go test ./...",
	PalCtx:               "Context Usage",
	PalCtxDesc:           "How many tokens the current history uses",
	PalClear:             "Clear Screen",
	PalClearDesc:         "Clear viewport (history is kept)",
	PalProvider:          "Switch Provider",
	PalProviderDesc:      "Switch between Ollama / OpenAI / Anthropic / OpenRouter",
	ProviderTitle:        " Select Provider ",
	ProviderHint:         "  ↑↓ navigate  Enter switch  m edit model  k edit key  u edit URL  Esc back\n\n",
	PalProfile:           "Edit Profile",
	PalProfileDesc:       "Set your name, role, and background for AI context",
	PalInstructions:      "Edit AI Instructions",
	PalInstructionsDesc:  "How AI should respond to you",
	ProfileTitle:         " Your Profile ",
	ProfileSaveHint:      "  Ctrl+S save  Esc cancel",
	InstructTitle:        " AI Instructions ",
	InstructSaveHint:     "  Ctrl+S save  Esc cancel",
	SessionsTitle:        " Sessions ",
	SessionsEmpty:        "  No saved sessions.\n\n",
	SessionsCount:        "\n  %d sessions saved",
	SessionsMsgs:         "%d msgs",
	SessionsLoading:      "  %s Loading sessions...\n",
	PaletteTitle:         " Command Palette ",
	PaletteEmpty:         "  Nothing found",
	AgeJustNow:           "just now",
	AgeMin:               "%dm ago",
	AgeHour:              "%dh ago",
	AgeDay:               "%dd ago",
	RetryConnecting:      "Connection lost. Retrying...",
	RetryAttempt:         "Retry %d/%d...",
	ProviderSectionLocal: "Local",
	ProviderSectionFree:  "Free tier",
	ProviderSectionCloud: "Cloud",
	ProviderKeyMissing:   "no key",
	ProviderKeyPresent:   "key set",
	ProviderKeyLocal:     "no key needed",
	ProviderPickModel:    "  Enter — pick model for this provider",
	ProviderKeyURL:       "  Get a key:",
	ModelTagCloud:        "cloud",
	ModelTagLocal:        "local",
	ModelTagFree:         "free",
	OverlayBack:          "Esc — back",
	FldModel:             "Model",
	FldKey:               "API Key",
	FldURL:               "Base URL",
}

var i18nRU = i18nStrings{
	Placeholder:      "Введи запрос... (Enter — отправить, Shift+Enter — перенос)",
	StatusReady:      "Готов — %d сообщений",
	StatusGenerating: "Генерирую",
	StatusLastTok:    "последний: %.1f tok/s · %d tok",
	HintSend:         "отправить",
	HintNewline:      "перенос",
	HintCommands:     "команды",
	HintModels:       "модели",
	HintSave:         "сохранить",
	HintLang:         "Ctrl+P→язык",
	Thinking:         "Думаю...",
	LoadingModels:    "Загрузка...",
	ModelSelectTitle: " Выбор модели ",
	ModelSelectHint:  "  ↑↓ навигация  Enter выбрать  p скачать  q пропустить\n\n",
	ModelSelectCount: "  Модель %d/%d",
	PullTitle:        " Загрузка модели ",
	PullInterrupt:    " — прервать",
	PullDone:         "Готово!",
	QAHint:           "  Space — выбрать  ↑↓ — навигация  Enter — отправить  Esc — отмена",
	QASelected:       "  ✓ Выбрано: %d",
	ContextDropped:   "контекст: удалено %d старых сообщений",
	WelcomeMsg:       "  Добро пожаловать в TermCode. Задай вопрос или попроси изменить файл.",
	PaletteSearch:    "Введи для поиска...",
	PaletteHint:      "  ↑↓ навигация  Enter выбрать  Esc закрыть",
	SessionHint:      "  ↑↓ навигация  Enter загрузить  Backspace удалить  Esc назад\n\n",
	UserLabel:        "Ты",
	// Palette
	PalCmdPalette:        "Палитра команд",
	PalCmdPaletteDesc:    "Открыть эту палитру",
	PalModels:            "Сменить модель",
	PalModelsDesc:        "Показать список моделей Ollama",
	PalPull:              "Скачать модель",
	PalPullDesc:          "Ввести имя модели для загрузки",
	PalNew:               "Новая сессия",
	PalNewDesc:           "Начать новый диалог (текущий сохранится)",
	PalLang:              "Сменить язык / Switch Language",
	PalLangDesc:          "EN ↔ RU — язык интерфейса и ответов AI",
	PalSessions:          "Загрузить сессию",
	PalSessionsDesc:      "Открыть список сохранённых диалогов",
	PalSave:              "Сохранить сессию",
	PalSaveDesc:          "Сохранить историю диалога на диск",
	PalLS:                "Список файлов",
	PalLSDesc:            "Показать дерево файлов проекта",
	PalGit:               "Git статус",
	PalGitDesc:           "Показать git status проекта",
	PalBuild:             "Go build",
	PalBuildDesc:         "Запустить go build ./...",
	PalTest:              "Go test",
	PalTestDesc:          "Запустить go test ./...",
	PalCtx:               "Использование контекста",
	PalCtxDesc:           "Сколько токенов занимает текущая история",
	PalClear:             "Очистить экран",
	PalClearDesc:         "Очистить viewport (история сохраняется)",
	PalProvider:          "Сменить провайдера",
	PalProviderDesc:      "Переключить Ollama / OpenAI / Anthropic / OpenRouter",
	ProviderTitle:        " Выбор провайдера ",
	ProviderHint:         "  ↑↓ навигация  Enter переключить  m модель  k ключ  u URL  Esc назад\n\n",
	PalProfile:           "Редактировать профиль",
	PalProfileDesc:       "Имя, роль и контекст о вас для AI",
	PalInstructions:      "Инструкции для AI",
	PalInstructionsDesc:  "Как AI должен отвечать вам",
	ProfileTitle:         " Ваш профиль ",
	ProfileSaveHint:      "  Ctrl+S сохранить  Esc отмена",
	InstructTitle:        " Инструкции для AI ",
	InstructSaveHint:     "  Ctrl+S сохранить  Esc отмена",
	SessionsTitle:        " Сессии ",
	SessionsEmpty:        "  Нет сохранённых сессий.\n\n",
	SessionsCount:        "\n  %d сессий сохранено",
	SessionsMsgs:         "%d сообщ.",
	SessionsLoading:      "  %s Загружаем сессии...\n",
	PaletteTitle:         " Палитра команд ",
	PaletteEmpty:         "  Ничего не найдено",
	AgeJustNow:           "только что",
	AgeMin:               "%dм назад",
	AgeHour:              "%dч назад",
	AgeDay:               "%dд назад",
	RetryConnecting:      "Соединение потеряно. Переподключаюсь...",
	RetryAttempt:         "Попытка %d/%d...",
	ProviderSectionLocal: "Локальный",
	ProviderSectionFree:  "Бесплатный",
	ProviderSectionCloud: "Облачный",
	ProviderKeyMissing:   "нет ключа",
	ProviderKeyPresent:   "ключ задан",
	ProviderKeyLocal:     "ключ не нужен",
	ProviderPickModel:    "  Enter — выбрать модель этого провайдера",
	ProviderKeyURL:       "  Получить ключ:",
	ModelTagCloud:        "облако",
	ModelTagLocal:        "локально",
	ModelTagFree:         "бесплатно",
	OverlayBack:          "Esc — назад",
	FldModel:             "Модель",
	FldKey:               "API ключ",
	FldURL:               "Base URL",
}

// tr — возвращает строки на текущем языке конфига.
func (m *Model) tr() i18nStrings {
	if m.cfg != nil && m.cfg.Language == "ru" {
		return i18nRU
	}
	return i18nEN
}
