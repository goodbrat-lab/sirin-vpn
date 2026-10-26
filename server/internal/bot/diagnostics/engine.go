package diagnostics

import (
	"context"
	"strings"
	"sync"
	"time"

	"sirin/server/internal/bot"
)

// Engine управляет выполнением диагностических проверок.
type Engine struct {
	deps Dependencies
}

// NewEngine создаёт новый экземпляр диагностического движка.
func NewEngine(deps Dependencies) *Engine {
	if deps.TCPProbe == nil {
		deps.TCPProbe = DefaultTCPProbe
	}
	return &Engine{deps: deps}
}

// Run запускает комплексную диагностику клиента.
func (e *Engine) Run(ctx context.Context, client *bot.Client, user *bot.UserKey, telegramID int64, input ClientInput) DiagnosticReport {
	// Общий таймаут диагностики 15 секунд
	diagCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var (
		keyRes       CheckResult
		apiRes       CheckResult
		egressIP     string
		paramsRes    CheckResult
		relayRes     CheckResult
		transportRes CheckResult
		wg           sync.WaitGroup
	)

	// 1. Проверка ключа (синхронная, без сети)
	keyRes = CheckKey(client, user, telegramID, e.deps.Storage)

	// 2. Параллельные сетевые проверки
	wg.Add(4)

	go func() {
		defer wg.Done()
		apiRes, egressIP = CheckAPI(diagCtx, e.deps.HTTPClient, e.deps.APIURL, e.deps.IsAdmin)
	}()

	go func() {
		defer wg.Done()
		paramsRes, _ = CheckParams(diagCtx, e.deps.HTTPClient, e.deps.APIURL, client)
	}()

	go func() {
		defer wg.Done()
		relayRes = CheckRelay(diagCtx, client, e.deps.Relays, e.deps.Clients, e.deps.TCPProbe)
	}()

	go func() {
		defer wg.Done()
		transportRes = CheckTransport(input)
	}()

	wg.Wait()

	// 3. Проверка контекста клиента (зависит от egressIP из CheckAPI)
	contextChecks := CheckClientContext(input, egressIP, e.deps.Releases)

	// 4. Сборка всех результатов
	var checks []CheckResult
	checks = append(checks, keyRes)
	checks = append(checks, apiRes)
	if client != nil {
		checks = append(checks, paramsRes)
	}
	checks = append(checks, relayRes)
	checks = append(checks, contextChecks...)
	checks = append(checks, transportRes)

	// 5. Расчёт общего статуса
	overall := StatusOK
	for _, c := range checks {
		if c.Status == StatusFail {
			overall = StatusFail
			break
		} else if c.Status == StatusWarn && overall != StatusFail {
			overall = StatusWarn
		}
	}

	// 6. Формирование рекомендаций
	summary := generateSummary(overall, checks, input)

	clientUUID := ""
	if client != nil {
		clientUUID = client.UUID
	} else if user != nil {
		clientUUID = user.UUID
	}

	return DiagnosticReport{
		Timestamp:  time.Now(),
		ClientUUID: clientUUID,
		TelegramID: telegramID,
		Input:      input,
		Checks:     checks,
		Overall:    overall,
		Summary:    summary,
	}
}

func generateSummary(overall CheckStatus, checks []CheckResult, input ClientInput) string {
	if overall == StatusFail {
		for _, c := range checks {
			if c.Status == StatusFail {
				if strings.Contains(c.Name, "ключ") {
					return "Проблема с ключом доступа. Продлите подписку или обратитесь к администратору."
				}
				if strings.Contains(c.Name, "Релей") {
					return "Релей недоступен. Примените авто-исправление для переключения на прямой сервер NL."
				}
				if strings.Contains(c.Name, "управлен") {
					return "Временные неполадки на сервере управления. Попробуйте повторить проверку через минуту."
				}
			}
		}
		return "Обнаружены критические неполадки. Обратитесь в поддержку для решения проблемы."
	}

	if overall == StatusWarn {
		var hints []string
		for _, c := range checks {
			if c.Status == StatusWarn {
				if c.AutoFix != nil && c.AutoFix.ID == "reset_relay_nl" {
					hints = append(hints, "рекомендуется переключиться на прямой сервер NL")
				}
				if c.AutoFix != nil && c.AutoFix.ID == "update_app" {
					hints = append(hints, "обновите приложение до последней версии")
				}
				if strings.Contains(c.Name, "Транспорт") || strings.Contains(c.Message, "UDP") {
					hints = append(hints, "смените транспорт в настройках на VLESS Reality или XHTTP")
				}
				if strings.Contains(c.Name, "Туннель") {
					hints = append(hints, "убедитесь, что переключатель VPN в приложении включен")
				}
			}
		}
		if len(hints) > 0 {
			return "Рекомендации: " + strings.Join(hints, "; ") + "."
		}
		return "Есть предупреждения, обратите внимание на подсказки выше."
	}

	if input.IssueType == "sites_blocked" || input.IssueType == "slow" {
		return "Сетевые компоненты в норме. Если отдельные сайты (YouTube/Instagram) зависают, попробуйте сменить транспорт (VLESS Reality / XHTTP) или перезапустить приложение."
	}

	return "Все сервисы и ключ работают в штатном режиме. Если у вас сохраняются сложности, создайте обращение в поддержку."
}
