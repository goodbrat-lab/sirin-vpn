package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sirin/server/internal/bot"
	"sirin/server/internal/relays"
)

// DefaultTCPProbe стандартный проверщик TCP-соединения с замером RTT.
func DefaultTCPProbe(ctx context.Context, host string, port int, timeout time.Duration) (time.Duration, error) {
	d := net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return 0, err
	}
	_ = conn.Close()
	return time.Since(start), nil
}

// CheckKey проверяет статус ключа и привязку к Telegram.
func CheckKey(client *bot.Client, user *bot.UserKey, telegramID int64, storage *bot.Storage) CheckResult {
	if client == nil {
		return CheckResult{
			Name:    "Статус ключа",
			Status:  StatusFail,
			Message: "Ключ не найден в базе данных сервера",
			Details: "Проверьте правильность введённого UUID ключа",
		}
	}

	if client.Revoked {
		return CheckResult{
			Name:    "Статус ключа",
			Status:  StatusFail,
			Message: "Ключ отозван администратором",
			Details: "Для восстановления доступа обратитесь в поддержку",
		}
	}

	if !client.IsActive() {
		return CheckResult{
			Name:    "Статус ключа",
			Status:  StatusFail,
			Message: "Срок действия ключа истёк",
			Details: "Продлите подписку в меню [💳 Купить подписку]",
		}
	}

	// Проверка срока истечения (< 3 дней)
	if client.ExpiresAt != nil && !client.ExpiresAt.IsZero() {
		daysLeft := int(time.Until(*client.ExpiresAt).Hours() / 24)
		if daysLeft < 0 {
			daysLeft = 0
		}
		if daysLeft <= 3 {
			res := CheckResult{
				Name:    "Срок действия ключа",
				Status:  StatusWarn,
				Message: fmt.Sprintf("Ключ активен, но истекает через %d дн.", daysLeft),
				Details: "Рекомендуется заблаговременно продлить подписку",
			}
			attachBindingFixIfNeeded(&res, client, user, telegramID, storage)
			return res
		}

		res := CheckResult{
			Name:    "Статус ключа",
			Status:  StatusOK,
			Message: fmt.Sprintf("Ключ активен (осталось %d дн.)", daysLeft),
		}
		attachBindingFixIfNeeded(&res, client, user, telegramID, storage)
		return res
	}

	res := CheckResult{
		Name:    "Статус ключа",
		Status:  StatusOK,
		Message: "Ключ активен (бессрочный)",
	}
	attachBindingFixIfNeeded(&res, client, user, telegramID, storage)
	return res
}

func attachBindingFixIfNeeded(res *CheckResult, client *bot.Client, user *bot.UserKey, telegramID int64, storage *bot.Storage) {
	if telegramID == 0 || storage == nil {
		return
	}
	if user == nil || user.TelegramID != telegramID {
		res.AutoFix = &AutoFix{
			ID:          "link_telegram",
			Title:       "🔗 Привязать ключ к аккаунту",
			Description: "Связать данный ключ с вашим Telegram для быстрого управления",
			Safe:        true,
			Apply: func(ctx context.Context) error {
				return storage.Set(bot.UserKey{
					TelegramID: telegramID,
					UUID:       client.UUID,
					Name:       client.Name,
					CreatedAt:  time.Now().Unix(),
				})
			},
		}
	}
}

// CheckAPI проверяет доступность управляющего API и возвращает egress IP сервера.
func CheckAPI(ctx context.Context, httpClient *http.Client, apiURL string, isAdmin bool) (CheckResult, string) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+"/health", nil)
	if err != nil {
		return CheckResult{
			Name:    "Сервер управления",
			Status:  StatusFail,
			Message: "Ошибка создания запроса к API",
			Details: err.Error(),
		}, ""
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return CheckResult{
			Name:    "Сервер управления",
			Status:  StatusFail,
			Message: "Сервер API временно недоступен",
			Details: "Не удалось установить соединение с сервером управления",
		}, ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CheckResult{
			Name:    "Сервер управления",
			Status:  StatusFail,
			Message: fmt.Sprintf("Сервер API вернул код ошибки %d", resp.StatusCode),
		}, ""
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return CheckResult{
			Name:    "Сервер управления",
			Status:  StatusFail,
			Message: "Некорректный ответ от API сервера",
		}, ""
	}

	status, _ := data["status"].(string)
	if status != "ok" {
		return CheckResult{
			Name:    "Сервер управления",
			Status:  StatusFail,
			Message: "API сообщил о сбое работы сервера",
		}, ""
	}

	egressIP, _ := data["egress_ip"].(string)
	var details string
	if isAdmin {
		clientsCount := data["clients"]
		details = fmt.Sprintf("Клиентов на сервере: %v, Egress IP: %s", clientsCount, egressIP)
	}

	return CheckResult{
		Name:    "Сервер управления",
		Status:  StatusOK,
		Message: "Сервер API работает в штатном режиме",
		Details: details,
	}, egressIP
}

// CheckParams запрашивает конфигурацию маршрутизации (/v1/params).
func CheckParams(ctx context.Context, httpClient *http.Client, apiURL string, client *bot.Client) (CheckResult, string) {
	if client == nil {
		return CheckResult{
			Name:    "Получение конфигурации",
			Status:  StatusFail,
			Message: "Ключ не указан, невозможно запросить конфигурацию",
		}, ""
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}

	endpoint := fmt.Sprintf("%s/v1/params?key=%s", apiURL, client.UUID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return CheckResult{
			Name:    "Получение конфигурации",
			Status:  StatusFail,
			Message: "Ошибка формирования запроса параметров",
			Details: err.Error(),
		}, ""
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return CheckResult{
			Name:    "Получение конфигурации",
			Status:  StatusFail,
			Message: "Сервер параметров временно недоступен",
		}, ""
	}
	defer resp.Body.Close()

	relayLoadHeader := resp.Header.Get("X-Relay-Load")

	if resp.StatusCode != http.StatusOK {
		return CheckResult{
			Name:    "Получение конфигурации",
			Status:  StatusFail,
			Message: fmt.Sprintf("Сервер конфигурации вернул код %d", resp.StatusCode),
		}, relayLoadHeader
	}

	var data struct {
		Status  string `json:"status"`
		Error   string `json:"error,omitempty"`
		Entries []any  `json:"entries,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return CheckResult{
			Name:    "Получение конфигурации",
			Status:  StatusFail,
			Message: "Ошибка разбора ответа параметров",
		}, relayLoadHeader
	}

	if data.Status == "refused" {
		return CheckResult{
			Name:    "Получение конфигурации",
			Status:  StatusFail,
			Message: "Конфигурация отклонена: " + data.Error,
		}, relayLoadHeader
	}

	return CheckResult{
		Name:    "Получение конфигурации",
		Status:  StatusOK,
		Message: "Параметры маршрутизации получены успешно",
	}, relayLoadHeader
}

// CheckRelay проверяет статус релея клиента и выполняет TCP-проверку задержки.
func CheckRelay(ctx context.Context, client *bot.Client, relaysStorage *relays.Storage, clientsStorage *bot.ClientsStorage, tcpProbe TCPProber) CheckResult {
	if client == nil || client.RelayID == 0 {
		return CheckResult{
			Name:    "Релей маршрутизации",
			Status:  StatusOK,
			Message: "Прямое подключение (Нидерланды NL)",
		}
	}

	if relaysStorage == nil {
		return CheckResult{
			Name:    "Релей маршрутизации",
			Status:  StatusUnknown,
			Message: fmt.Sprintf("Назначен релей #%d (информация о релеях не загружена)", client.RelayID),
		}
	}

	relay, found := relaysStorage.GetByID(client.RelayID)
	if !found || !relay.Enabled {
		return CheckResult{
			Name:    "Релей маршрутизации",
			Status:  StatusFail,
			Message: fmt.Sprintf("Релей #%d отключен или удалён", client.RelayID),
			Details: "Трафик не проходит через отключенный релей",
			AutoFix: makeResetRelayFix(client, clientsStorage),
		}
	}

	var allClients []*bot.Client
	if clientsStorage != nil {
		allClients = clientsStorage.List()
	}
	activeOnRelay := relays.CountActiveOnRelay(allClients, relay.ID)

	if relay.MaxClients > 0 && activeOnRelay >= relay.MaxClients {
		return CheckResult{
			Name:    "Релей маршрутизации",
			Status:  StatusWarn,
			Message: fmt.Sprintf("Релей #%d (%s) перегружен (%d/%d клиентов)", relay.ID, relay.Name, activeOnRelay, relay.MaxClients),
			Details: "Высокая нагрузка может снижать скорость соединения",
			AutoFix: makeResetRelayFix(client, clientsStorage),
		}
	}

	if tcpProbe != nil && relay.Host != "" && relay.PortVLESS > 0 {
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		rtt, err := tcpProbe(probeCtx, relay.Host, relay.PortVLESS, 2*time.Second)
		if err != nil {
			return CheckResult{
				Name:    "Релей маршрутизации",
				Status:  StatusWarn,
				Message: "RTT до релея не определён (probe timeout), но релей активен. Возможно, временный сбой.",
				Details: "Если связь нестабильна, можно переключиться на прямой выход Direct NL.",
				AutoFix: nil,
			}
		}

		if rtt > 150*time.Millisecond {
			return CheckResult{
				Name:    "Релей маршрутизации",
				Status:  StatusWarn,
				Message: fmt.Sprintf("Релей #%d (%s) отвечает с высокой задержкой (RTT ~%d мс, нагрузка %d/%d)", relay.ID, relay.Name, rtt.Milliseconds(), activeOnRelay, relay.MaxClients),
				Details: "Высокий пинг может вызывать задержки при загрузке видео.",
				AutoFix: nil,
			}
		}

		return CheckResult{
			Name:    "Релей маршрутизации",
			Status:  StatusOK,
			Message: fmt.Sprintf("Релей #%d (%s) доступен (RTT ~%d мс, нагрузка %d/%d)", relay.ID, relay.Name, rtt.Milliseconds(), activeOnRelay, relay.MaxClients),
		}
	}

	return CheckResult{
		Name:    "Релей маршрутизации",
		Status:  StatusOK,
		Message: fmt.Sprintf("Релей #%d (%s) активен (нагрузка %d/%d)", relay.ID, relay.Name, activeOnRelay, relay.MaxClients),
	}
}

func makeResetRelayFix(client *bot.Client, clientsStorage *bot.ClientsStorage) *AutoFix {
	if client == nil || clientsStorage == nil {
		return nil
	}
	return &AutoFix{
		ID:          "reset_relay_nl",
		Title:       "🔄 Переключить на прямой выход NL",
		Description: "Сбросить релей на прямой стабильный сервер в Нидерландах",
		Safe:        true,
		Apply: func(ctx context.Context) error {
			_, err := clientsStorage.SetRelayID(client.UUID, 0)
			return err
		},
	}
}

// CheckClientContext проверяет совпадение внешнего IP клиента с VPN egress IP и версию клиента.
func CheckClientContext(input ClientInput, egressIP string, releases ReleaseProvider) []CheckResult {
	var results []CheckResult

	// 1. Проверка IP
	if input.ClientIP != "" {
		clientIP := strings.TrimSpace(input.ClientIP)
		srvIP := strings.TrimSpace(egressIP)

		if srvIP != "" && clientIP == srvIP {
			results = append(results, CheckResult{
				Name:    "Туннель VPN (IP)",
				Status:  StatusOK,
				Message: "Трафик идёт через защищённый VPN-сервер",
				Details: fmt.Sprintf("Внешний IP: %s", MaskIP(clientIP)),
			})
		} else if srvIP != "" {
			results = append(results, CheckResult{
				Name:    "Туннель VPN (IP)",
				Status:  StatusWarn,
				Message: "Ваш IP не совпадает с IP сервера VPN",
				Details: fmt.Sprintf("Ваш IP: %s (Сервер: %s). Проверьте, включено ли подключение в приложении.", MaskIP(clientIP), MaskIP(srvIP)),
			})
		}
	}

	// 2. Проверка версии приложения
	if input.AppVersion != "" && input.AppVersion != "unknown" && releases != nil {
		platform := input.Platform
		if platform == "" {
			platform = "android"
		}
		latestVer, latestURL := releases.GetLatestVersion(platform)
		if latestVer != "" {
			if strings.TrimPrefix(input.AppVersion, "v") != strings.TrimPrefix(latestVer, "v") {
				results = append(results, CheckResult{
					Name:    "Версия приложения",
					Status:  StatusWarn,
					Message: fmt.Sprintf("Установлена версия %s, доступна новая %s", input.AppVersion, latestVer),
					Details: "В новой версии исправлены ошибки подключения и улучшена стабильность",
					AutoFix: &AutoFix{
						ID:          "update_app",
						Title:       fmt.Sprintf("📥 Скачать обновление %s", latestVer),
						Description: latestURL,
						Safe:        true,
						Apply:       nil, // Ссылка для пользователя, автоматической установки в ОС нет
					},
				})
			} else {
				results = append(results, CheckResult{
					Name:    "Версия приложения",
					Status:  StatusOK,
					Message: fmt.Sprintf("Установлена актуальная версия (%s)", input.AppVersion),
				})
			}
		}
	}

	return results
}

// CheckTransport проверяет выбранный пользователем транспорт.
func CheckTransport(input ClientInput) CheckResult {
	transport := strings.ToLower(strings.TrimSpace(input.Transport))

	switch transport {
	case "udp":
		return CheckResult{
			Name:    "Протокол транспорта",
			Status:  StatusWarn,
			Message: "Выбран протокол UDP",
			Details: "В сетях мобильных операторов РФ (МТС, Билайн и др.) протокол UDP часто блокируется ТСПУ. Рекомендуем в настройках переключить транспорт на VLESS Reality или XHTTP.",
		}
	case "vless", "xhttp":
		return CheckResult{
			Name:    "Протокол транспорта",
			Status:  StatusOK,
			Message: fmt.Sprintf("Выбран устойчивый транспорт (%s)", strings.ToUpper(transport)),
		}
	default:
		if input.IssueType == "slow" || input.IssueType == "sites_blocked" {
			return CheckResult{
				Name:    "Протокол транспорта",
				Status:  StatusWarn,
				Message: "Транспорт не указан",
				Details: "Если YouTube или сайты не открываются — попробуйте сменить транспорт в настройках приложения (VLESS Reality или XHTTP).",
			}
		}
		return CheckResult{
			Name:    "Протокол транспорта",
			Status:  StatusUnknown,
			Message: "Транспорт не указан",
		}
	}
}
