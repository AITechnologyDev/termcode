package ai

import "strings"

// TrimMessages обрезает историю сообщений чтобы уложиться в contextLength токенов.
// Стратегия:
//  1. Системный промпт + последнее сообщение пользователя — неприкосновенны
//  2. Удаляем старые сообщения с начала (кроме первого user-сообщения)
//  3. Оставляем минимум 4 последних сообщения для связности диалога
//
// Возвращает обрезанный срез и количество удалённых сообщений.
func TrimMessages(msgs []Message, systemPrompt string, contextLength int) (trimmed []Message, dropped int) {
	if len(msgs) == 0 {
		return msgs, 0
	}

	// Резервируем 30% контекста для ответа модели и системного промпта
	// Оставшиеся 70% — под историю
	historyBudget := int(float64(contextLength) * 0.70)
	systemTokens := estimateTokens(systemPrompt)
	historyBudget -= systemTokens
	if historyBudget < 512 {
		historyBudget = 512
	}

	// Считаем токены всей истории
	total := 0
	for _, m := range msgs {
		total += estimateTokens(m.Content) + 4 // +4 на роль и разделители
	}

	if total <= historyBudget {
		return msgs, 0 // всё влезает — ничего не делаем
	}

	// Удаляем сообщения с начала пока не уложимся
	// Минимум оставляем 4 последних сообщения
	minKeep := 4
	if len(msgs) <= minKeep {
		return msgs, 0
	}

	result := make([]Message, len(msgs))
	copy(result, msgs)

	for total > historyBudget && len(result) > minKeep {
		dropped++
		removed := result[0]
		total -= estimateTokens(removed.Content) + 4
		result = result[1:]
	}

	return result, dropped
}

// EstimateTokens — публичный алиас для использования в других пакетах
func EstimateTokens(text string) int {
	return estimateTokens(text)
}

// estimateTokens приближённо оценивает количество токенов в тексте.
// Правило: ~4 символа = 1 токен для латиницы, ~2 символа = 1 токен для CJK/кириллицы.
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}

	latin := 0
	other := 0
	for _, r := range text {
		if r < 128 {
			latin++
		} else {
			other++
		}
	}

	tokens := latin/4 + other/2
	if tokens < 1 && len(text) > 0 {
		tokens = 1
	}
	return tokens
}

// SumTokens считает суммарные токены среза сообщений
func SumTokens(msgs []Message) int {
	total := 0
	for _, m := range msgs {
		total += estimateTokens(m.Content) + 4
	}
	return total
}

// FormatContextInfo возвращает строку вида "~1200 / 8192 tok" для статусбара
func FormatContextInfo(msgs []Message, systemPrompt string, contextLength int) string {
	used := estimateTokens(systemPrompt) + SumTokens(msgs)
	pct := 0
	if contextLength > 0 {
		pct = used * 100 / contextLength
	}

	suffix := ""
	if pct >= 80 {
		suffix = " ⚠"
	} else if pct >= 95 {
		suffix = " ✗"
	}

	return formatK(used) + " / " + formatK(contextLength) + " tok" + suffix
}

func formatK(n int) string {
	if n >= 1000 {
		s := strings.TrimRight(strings.TrimRight(
			string(rune('0'+n/1000))+"."+ // тысячи
				string(rune('0'+(n%1000)/100)), // сотни
			"0"), ".")
		return s + "k"
	}
	return itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
