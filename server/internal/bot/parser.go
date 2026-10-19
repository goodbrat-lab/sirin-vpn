package bot

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var digitsRegex = regexp.MustCompile(`^[0-9]+$`)

// ErrEmptyName возвращается, если не указано имя клиента.
var ErrEmptyName = errors.New("empty name")

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
