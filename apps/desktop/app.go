package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"sirin/desktop/protector"
	"sirin/desktop/route"
	"sirin/desktop/tun"
	"sirin/engine"
	"sirin/engine/desktop"
)

// App — сервис управления VPN для Wails v3
type App struct {
	mu     sync.Mutex
	state  string // "disconnected", "connecting", "connected", "error"
	handle *desktop.Handle
	tun    *tun.Interface
	cancel context.CancelFunc
	logs   []string
}

func NewApp() *App {
	return &App{
		state: "disconnected",
		logs:  make([]string, 0, 100),
	}
}

// ParamsResponse — формат ответа /v1/params
type ParamsResponse struct {
	Contract   string        `json:"contract"`
	Status     string        `json:"status"`
	Generation int           `json:"generation"`
	TTL        int           `json:"ttl"`
	Now        int64         `json:"now"`
	Relay      ParamsRelay   `json:"relay"`
	Ladder     ParamsLadder  `json:"ladder"`
	Entries    []ParamsEntry `json:"entries"`
}

type ParamsRelay struct {
	Enabled bool   `json:"enabled"`
	Target  string `json:"target"`
}

type ParamsLadder struct {
	DirectProbeEveryAttempts int `json:"direct_probe_every_attempts"`
	DirectSkipAfterFails     int `json:"direct_skip_after_fails"`
	SecondRoundMs            int `json:"second_round_ms"`
	DirectProbeIntervalMs    int `json:"direct_probe_interval_ms"`
}

type ParamsEntry struct {
	ID        string   `json:"id"`
	Host      string   `json:"host"`
	Port      int      `json:"port"`
	DTLS      bool     `json:"dtls"`
	Transport string   `json:"transport"`
	SNI       string   `json:"sni"`
	SNIs      []string `json:"snis"`
	PBK       string   `json:"pbk"`
	SID       string   `json:"sid"`
}

// Connect инициирует подключение
func (a *App) Connect(key string) error {
	// Cleanup на случай если прошлый запуск упал с активными маршрутами
	_ = route.Cleanup()

	a.mu.Lock()
	if a.state == "connecting" || a.state == "connected" {
		a.mu.Unlock()
		return fmt.Errorf("уже подключается или подключено")
	}
	a.state = "connecting"
	a.mu.Unlock()
	a.emitState("connecting", "")

	key = strings.TrimSpace(key)
	if key != "" {
		_ = a.SaveKey(key)
	} else {
		key = a.GetKey()
	}
	if key == "" {
		a.mu.Lock()
		a.state = "error"
		a.mu.Unlock()
		a.emitState("error", "Ключ доступа не указан")
		return fmt.Errorf("ключ доступа не указан")
	}

	a.emitLog("Запрос параметров подключения...", "info")

	params, err := fetchParams(key)
	if err != nil {
		a.mu.Lock()
		a.state = "error"
		a.mu.Unlock()
		a.emitState("error", fmt.Sprintf("Ошибка получения параметров: %v", err))
		a.emitLog(fmt.Sprintf("Ошибка /v1/params: %v", err), "error")
		return err
	}

	ladder, gateway, err := buildLadder(params, key)
	if err != nil {
		a.mu.Lock()
		a.state = "error"
		a.mu.Unlock()
		a.emitState("error", fmt.Sprintf("Ошибка сборки лестницы: %v", err))
		a.emitLog(fmt.Sprintf("Ошибка сборки лестницы: %v", err), "error")
		return err
	}

	a.emitLog("Создание сетевого адаптера Wintun...", "info")
	tunDev, err := tun.New("Sirin", 1280)
	if err != nil {
		a.mu.Lock()
		a.state = "error"
		a.mu.Unlock()
		a.emitState("error", fmt.Sprintf("Ошибка Wintun: %v", err))
		a.emitLog(fmt.Sprintf("Ошибка Wintun: %v", err), "error")
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.cancel = cancel
	a.tun = tunDev
	a.mu.Unlock()

	cfg := desktop.Config{
		Gateway:        gateway,
		CachedFallback: params.Entries[0].Host,
		Ladder:         ladder,
		Password:       key,
		DeviceID:       deviceID(),
		Protector:      protector.New(physicalIfaceIndex()),
		ClampMSS:       true,
		TunnelDNS:      desktop.DefaultTunnelDNS,
	}

	a.emitLog(fmt.Sprintf("Подключение к шлюзу %s...", gateway), "info")
	handle, events, err := desktop.Start(ctx, cfg, tunDev)
	if err != nil {
		a.mu.Lock()
		a.state = "error"
		c := a.cancel
		t := a.tun
		a.cancel = nil
		a.tun = nil
		a.mu.Unlock()

		if c != nil {
			c()
		}
		if t != nil {
			t.Close()
		}

		a.emitState("error", fmt.Sprintf("Ошибка подключения: %v", err))
		a.emitLog(fmt.Sprintf("Ошибка подключения: %v", err), "error")
		return err
	}

	a.mu.Lock()
	a.handle = handle
	a.state = "connected"
	a.mu.Unlock()

	// Получить ifIndex Wintun
	ifIndex, err := tunDev.InterfaceIndex()
	if err != nil {
		a.emitLog(fmt.Sprintf("WARN: InterfaceIndex: %v", err), "error")
	}

	// Получить IP/gateway из handle
	info := handle.Info()
	a.emitLog(fmt.Sprintf("Tunnel: ip=%s prefix=%d gateway=%s mtu=%d",
		info.IP, info.PrefixLen, info.Gateway, info.MTU), "info")

	// Настроить IP на Wintun (netsh)
	if err := configureTunAddress("Sirin", info.IP, info.PrefixLen, info.Gateway); err != nil {
		a.emitLog(fmt.Sprintf("WARN: configureTunAddress: %v", err), "error")
	}

	// Включить full tunnel (route add)
	if err := route.Setup(info.Gateway, ifIndex); err != nil {
		a.emitLog(fmt.Sprintf("WARN: route.Setup: %v", err), "error")
	} else {
		a.emitLog("Full tunnel enabled (route /1)", "info")
	}

	// Настроить DNS
	if err := route.SetupDNS("Sirin", []string{"1.1.1.1", "8.8.8.8"}); err != nil {
		a.emitLog(fmt.Sprintf("WARN: SetupDNS: %v", err), "error")
	}

	a.emitState("connected", "")
	go a.handleEvents(events)

	return nil
}

// Disconnect обрывает соединение
func (a *App) Disconnect() error {
	// 1. Забрать ресурсы под lock, БЫСТРО
	a.mu.Lock()
	h := a.handle
	c := a.cancel
	t := a.tun
	a.handle = nil
	a.cancel = nil
	a.tun = nil
	a.state = "disconnected"
	a.mu.Unlock() // ← ЯВНЫЙ Unlock, без defer

	// 2. Освободить ресурсы ВНЕ lock
	if c != nil {
		c()
	}
	if h != nil {
		h.Stop()
	}
	if t != nil {
		t.Close()
	}

	// 3. Cleanup маршрутов ВНЕ lock
	_ = route.Cleanup()
	_ = route.CleanupDNS("Sirin")

	// 4. События ВНЕ lock (emitLog/emitState берут mu сами)
	a.emitState("disconnected", "")
	a.emitLog("Отключено", "info")
	return nil
}

// GetStatus возвращает текущий статус туннеля
func (a *App) GetStatus() map[string]interface{} {
	a.mu.Lock()
	defer a.mu.Unlock()

	st := desktop.CurrentStatus()
	res := map[string]interface{}{
		"state":  a.state,
		"winner": st.Winner,
		"active": st.Active,
		"lastTx": st.LastTx,
		"lastRx": st.LastRx,
		"mtu":    st.MTU,
	}
	if a.handle != nil {
		info := a.handle.Info()
		res["ip"] = info.IP
		res["prefixLen"] = info.PrefixLen
		res["gateway"] = info.Gateway
	}
	return res
}

// GetKey возвращает сохранённый ключ из конфигурации
func (a *App) GetKey() string {
	cfg, err := LoadConfig()
	if err != nil {
		return ""
	}
	return cfg.Key
}

// SaveKey сохраняет ключ в локальную конфигурацию
func (a *App) SaveKey(key string) error {
	cfg, err := LoadConfig()
	if err != nil {
		cfg = &LocalConfig{}
	}
	cfg.Key = strings.TrimSpace(key)
	return SaveConfig(cfg)
}

// GetLogs возвращает накопленный журнал сообщений
func (a *App) GetLogs() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	res := make([]string, len(a.logs))
	copy(res, a.logs)
	return res
}

func (a *App) handleEvents(events <-chan desktop.Event) {
	for ev := range events {
		switch ev.Kind {
		case "ready":
			a.mu.Lock()
			a.state = "connected"
			a.mu.Unlock()
			a.emitState("connected", "")
			a.emitLog(fmt.Sprintf("Туннель готов: %s", ev.Reason), "info")
		case "log":
			a.emitLog(ev.Reason, "info")
		case "stopping":
			a.emitLog("Остановка туннеля: "+ev.Reason, "info")
		case "stopped":
			a.mu.Lock()
			isExpected := (a.state == "disconnected")
			a.state = "disconnected"
			a.mu.Unlock()
			if !isExpected {
				a.emitState("disconnected", ev.Reason)
			}
			a.emitLog("Туннель остановлен: "+ev.Reason, "info")
		}
	}
}

func (a *App) emitState(state string, errStr string) {
	payload := map[string]interface{}{
		"state": state,
	}
	if errStr != "" {
		payload["error"] = errStr
	}
	wailsApp := application.Get()
	if wailsApp != nil {
		wailsApp.Event.Emit("vpn:state", payload)
	}
}

func (a *App) emitLog(msg string, level string) {
	a.mu.Lock()
	line := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)
	a.logs = append(a.logs, line)
	if len(a.logs) > 500 {
		a.logs = a.logs[len(a.logs)-500:]
	}
	a.mu.Unlock()

	payload := map[string]interface{}{
		"message": msg,
		"level":   level,
	}
	wailsApp := application.Get()
	if wailsApp != nil {
		wailsApp.Event.Emit("vpn:log", payload)
	}
}

func fetchParams(key string) (*ParamsResponse, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	urls := []string{
		fmt.Sprintf("https://crp.guseynovia.ru/v1/params?key=%s", url.QueryEscape(key)),
		fmt.Sprintf("https://api.guseynovia.ru/v1/params?key=%s", url.QueryEscape(key)),
	}

	var lastErr error
	for _, u := range urls {
		resp, err := client.Get(u)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status %d from %s", resp.StatusCode, u)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}

		var paramsResp ParamsResponse
		if err := json.Unmarshal(body, &paramsResp); err != nil {
			lastErr = err
			continue
		}

		if paramsResp.Status != "answered" {
			lastErr = fmt.Errorf("сервер вернул статус: %s", paramsResp.Status)
			continue
		}

		return &paramsResp, nil
	}

	return nil, fmt.Errorf("не удалось получить параметры: %w", lastErr)
}

func buildLadder(resp *ParamsResponse, key string) (*engine.Ladder, string, error) {
	ladder := engine.NewLadder()
	if resp.Ladder.SecondRoundMs > 0 {
		ladder.SetSecondRoundMs(int32(resp.Ladder.SecondRoundMs))
	} else {
		ladder.SetSecondRoundMs(3000)
	}

	gateway := "crp.guseynovia.ru"
	if len(resp.Entries) > 0 && resp.Entries[0].Host != "" {
		gateway = resp.Entries[0].Host
	}

	// VLESS UUID — отдельный секрет Xray, получаем из API или 
	// захардкожен (как в Android-клиенте).
	// TODO: API должен отдавать vless_uuid в /v1/params.
	// Пока — захардкожен (совпадает с сервером):
	const vlessUUID = "5f26ebc3-6eb0-45fb-a490-c8c92b86b644"

	hasEntries := false
	for _, entry := range resp.Entries {
		switch strings.ToLower(entry.Transport) {
		case "udp", "":
			ladder.AddDirect(entry.ID, entry.Host, int32(entry.Port), entry.DTLS)
			hasEntries = true
		case "vless":
			sniCSV := strings.Join(entry.SNIs, ",")
			if sniCSV == "" {
				sniCSV = entry.SNI
			}
			target := "127.0.0.1:56006"
			err := ladder.AddVLESS(
				entry.ID,
				entry.Host,
				int32(entry.Port),
				vlessUUID, // ← не key!
				entry.PBK,
				entry.SID,
				entry.SNI,
				target,
				sniCSV,
			)
			if err != nil {
				return nil, "", fmt.Errorf("AddVLESS %s: %w", entry.ID, err)
			}
			hasEntries = true
		}
	}

	if !hasEntries {
		return nil, "", fmt.Errorf("в ответе /v1/params нет поддерживаемых узлов")
	}

	return ladder, gateway, nil
}

func physicalIfaceIndex() uint32 {
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, ifi := range ifaces {
			if ifi.Flags&net.FlagLoopback == 0 && ifi.Flags&net.FlagUp != 0 {
				name := strings.ToLower(ifi.Name)
				if !strings.Contains(name, "sirin") && !strings.Contains(name, "wintun") {
					return uint32(ifi.Index)
				}
			}
		}
	}
	return 1
}

func deviceID() string {
	host, _ := os.Hostname()
	return "desktop-" + host
}

func configureTunAddress(ifaceName, ip string, prefixLen int, gateway string) error {
	mask := prefixToMask(prefixLen)
	args := []string{"interface", "ip", "set", "address", "name=" + ifaceName,
		"static", ip, mask, gateway, "store=active"}
	if out, err := exec.Command("netsh", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("netsh set address: %w (%s)", err, out)
	}
	return nil
}

func prefixToMask(prefix int) string {
	mask := (0xFFFFFFFF << (32 - prefix)) & 0xFFFFFFFF
	return fmt.Sprintf("%d.%d.%d.%d", byte(mask>>24), byte(mask>>16), byte(mask>>8), byte(mask))
}
