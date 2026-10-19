package bot

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var digitsRegex = regexp.MustCompile(`^[0-9]+$`)

var (
	// ErrEmptyName возвращается, если не указано имя клиента.
	ErrEmptyName = errors.New("empty name")
	// ErrClientNotFound возвращается, если клиент не найден.
	ErrClientNotFound = errors.New("клиент не найден")
	// ErrAmbiguousClient возвращается при нескольких совпадениях.
	ErrAmbiguousClient = errors.New("найдено несколько клиентов")
	// ErrInvalidRenameSyntax возвращается при некорректном синтаксисе переименования.
	ErrInvalidRenameSyntax = errors.New("неверный формат команды (используйте: /rename <старое> -> <новое>)")
)

const (
	// DefaultDays — срок действия ключа по умолчанию (30 дней).
	DefaultDays = 30
	// MaxDays — максимально допустимый срок действия ключа (3650 дней, ~10 лет).
	MaxDays = 3650
	// MaxNameLen — максимальная длина имени в символах (рунах).
	MaxNameLen = 64
)

// parseNewKeyArgs парсит имя и срок из строки аргументов команды /newkey.
func parseNewKeyArgs(text string) (name string, days int, err error) {
	name, days, _, err = parseNewKeyArgsDetailed(text)
	return name, days, err
}

// parseNewKeyArgsDetailed выполняет парсинг и дополнительно возвращает флаг clamped,
// если запрошенный пользователем срок превысил MaxDays.
func parseNewKeyArgsDetailed(text string) (name string, days int, clamped bool, err error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "/newkey") {
		text = strings.TrimPrefix(text, "/newkey")
		if strings.HasPrefix(text, "@") {
			if idx := strings.Index(text, " "); idx != -1 {
				text = text[idx:]
			} else {
				text = ""
			}
		}
	}

	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", 0, false, ErrEmptyName
	}

	days = DefaultDays
	clamped = false
	nameTokens := fields

	// Если токенов 2 и более, проверяем последний токен: если это целое положительное число — это дни
	if len(fields) >= 2 {
		last := fields[len(fields)-1]
		if digitsRegex.MatchString(last) {
			val, convErr := strconv.Atoi(last)
			if convErr == nil && val > 0 {
				if val > MaxDays {
					days = MaxDays
					clamped = true
				} else {
					days = val
				}
				nameTokens = fields[:len(fields)-1]
			}
		}
	}

	name = strings.Join(nameTokens, " ")
	name = strings.TrimSpace(name)

	// Ограничение длины имени до MaxNameLen рун
	if utf8.RuneCountInString(name) > MaxNameLen {
		runes := []rune(name)
		name = strings.TrimSpace(string(runes[:MaxNameLen]))
	}

	if name == "" {
		return "", 0, false, ErrEmptyName
	}

	return name, days, clamped, nil
}

// maskUUID возвращает маскированный UUID: первые 8 символов + "…".
func maskUUID(uuid string) string {
	if len(uuid) <= 8 {
		return uuid
	}
	return uuid[:8] + "…"
}

// formatDaysLeft форматирует оставшийся срок действия.
func formatDaysLeft(expiresAt *time.Time) string {
	if expiresAt == nil || expiresAt.IsZero() {
		return "бессрочно"
	}
	now := time.Now()
	diff := expiresAt.Sub(now)
	if diff < 0 {
		days := int((-diff).Hours() / 24)
		if days == 0 {
			return "просрочен сегодня"
		}
		return fmt.Sprintf("просрочен %dд", days)
	}
	days := int(diff.Hours() / 24)
	if diff.Hours() < 24 {
		return "истекает сегодня"
	}
	return fmt.Sprintf("%dд", days)
}

func isHexOrUUID(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == '-') {
			return false
		}
	}
	return true
}

// resolveClient ищет клиента по точному имени, префиксу UUID (>=6 символов) или подстроке.
func resolveClient(clients []*Client, arg string) (*Client, error, []*Client) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return nil, ErrClientNotFound, nil
	}

	argLower := strings.ToLower(arg)

	// 1. Поиск по префиксу UUID (если 6+ символов hex)
	if len(arg) >= 6 && isHexOrUUID(arg) {
		var uuidMatches []*Client
		for _, c := range clients {
			if strings.HasPrefix(strings.ToLower(c.UUID), argLower) {
				uuidMatches = append(uuidMatches, c)
			}
		}
		if len(uuidMatches) == 1 {
			return uuidMatches[0], nil, nil
		}
		if len(uuidMatches) > 1 {
			return nil, ErrAmbiguousClient, uuidMatches
		}
	}

	// 2. Точное совпадение по имени (регистронезависимо)
	var exactMatches []*Client
	for _, c := range clients {
		if strings.EqualFold(c.Name, arg) {
			exactMatches = append(exactMatches, c)
		}
	}
	if len(exactMatches) == 1 {
		return exactMatches[0], nil, nil
	}
	if len(exactMatches) > 1 {
		return nil, ErrAmbiguousClient, exactMatches
	}

	// 3. Частичное совпадение по имени (подстрока)
	var subMatches []*Client
	for _, c := range clients {
		if strings.Contains(strings.ToLower(c.Name), argLower) {
			subMatches = append(subMatches, c)
		}
	}
	if len(subMatches) == 1 {
		return subMatches[0], nil, nil
	}
	if len(subMatches) > 1 {
		return nil, ErrAmbiguousClient, subMatches
	}

	return nil, ErrClientNotFound, nil
}

// parseRenameArgs разбирает синтаксис команды /rename <старое> -> <новое>.
func parseRenameArgs(text string) (oldName, newName string, err error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "/rename") {
		text = strings.TrimPrefix(text, "/rename")
		if strings.HasPrefix(text, "@") {
			if idx := strings.Index(text, " "); idx != -1 {
				text = text[idx:]
			} else {
				text = ""
			}
		}
	}
	text = strings.TrimSpace(text)

	parts := strings.SplitN(text, "->", 2)
	if len(parts) != 2 {
		return "", "", ErrInvalidRenameSyntax
	}

	oldName = strings.TrimSpace(parts[0])
	newName = strings.TrimSpace(parts[1])

	if oldName == "" || newName == "" {
		return "", "", ErrInvalidRenameSyntax
	}

	if utf8.RuneCountInString(newName) > MaxNameLen {
		runes := []rune(newName)
		newName = strings.TrimSpace(string(runes[:MaxNameLen]))
	}

	return oldName, newName, nil
}
