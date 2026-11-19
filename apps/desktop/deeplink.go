package main

import (
	"net/url"
	"strings"
)

// ParseSirinDeepLink извлекает 36-символьный UUID из deep-link ссылки вида:
//   - sirin://key?uuid=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
//   - sirin://import?key=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
//   - https://crp.guseynovia.ru/key?uuid=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
// Возвращает пустую строку, если аргумент не является валидной ссылкой с UUID.
func ParseSirinDeepLink(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "sirin" && scheme != "https" && scheme != "http" {
		return ""
	}
	q := u.Query()
	for _, k := range []string{"uuid", "key"} {
		v := strings.TrimSpace(q.Get(k))
		if isValidUUIDFormat(v) {
			return v
		}
	}
	// Поддержка короткой формы sirin://key/<uuid>
	trimmedPath := strings.Trim(u.Path, "/")
	if isValidUUIDFormat(trimmedPath) {
		return trimmedPath
	}
	return ""
}

func isValidUUIDFormat(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}
