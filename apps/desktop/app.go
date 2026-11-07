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
	"path/filepath"
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
	Enabled    bool     `json:"enabled"`
	Port       int      `json:"port"`
	DTLS       bool     `json:"dtls"`
	TURNBackup string   `json:"turn_backup"`
	TURNRealm  string   `json:"turn_realm"`
	Target     string   `json:"target"`
	VKHashes   []string `json:"vk_hashes"`
	DionSlugs  []string `json:"dion_slugs"`
}

type ParamsLadder struct {
	DirectProbeEveryAttempts int `json:"direct_probe_every_attempts"`
	DirectSkipAfterFails     int `json:"direct_skip_after_fails"`
	SecondRoundMs            int `json:"second_round_ms"`
	DirectProbeIntervalMs    int `json:"direct_probe_interval_ms"`
}

type ParamsEntry struct {
	ID         string   `json:"id"`
	Host       string   `json:"host"`
	Port       int      `json:"port"`
	DTLS       bool     `json:"dtls"`
	Transport  string   `json:"transport"`
	SNI        string   `json:"sni"`
	SNIs       []string `json:"snis"`
	PBK        string   `json:"pbk"`
	SID        string   `json:"sid"`
	Path       string   `json:"path,omitempty"`
	Mode       string   `json:"mode,omitempty"`
	TurnBackup string   `json:"turn_backup,omitempty"`
	TurnRealm  string   `json:"turn_realm,omitempty"`
	VKHashes   []string `json:"vk_hashes,omitempty"`
	DionSlugs  []string `json:"dion_slugs,omitempty"`
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

	localCfg, _ := LoadConfig()
	var deadDropURLs []string
	if localCfg != nil {
		deadDropURLs = localCfg.DeadDropURLs
	}

	params, source, err := fetchParams(key, deadDropURLs, a.emitLog)
	if err != nil {
		a.mu.Lock()
		a.state = "error"
		a.mu.Unlock()
		a.emitState("error", fmt.Sprintf("Ошибка получения параметров: %v", err))
		a.emitLog(fmt.Sprintf("Ошибка получения параметров (API, Dead Drop, кеш недоступны): %v", err), "error")
		return err
	}
	if source != "api" {
		a.emitLog(fmt.Sprintf("Параметры успешно получены из резервного источника [%s]", source), "info")
	}

	var wlCfg *WhitelistFallbackConfig
	if localCfg != nil {
		wlCfg = localCfg.WhitelistFallback
	}

	ladder, gateway, err := buildLadder(params, key, wlCfg)
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

	// Загружаем локальную конфигурацию маршрутизации
	localCfg, _ = LoadConfig()
	if localCfg == nil {
		localCfg = &LocalConfig{RuDirect: true, SplitMode: "bypass"}
	}

	// Настройка маршрутизации (Full tunnel / RuDirect / Split tunneling)
	routeCfg := route.Config{
		Gateway:       info.Gateway,
		IfIndex:       ifIndex,
		RuDirect:      localCfg.RuDirect,
		Mode:          localCfg.SplitMode,
		CustomCIDRs:   localCfg.SplitCIDRs,
		CustomDomains: localCfg.SplitDomains,
	}

	if err := route.Setup(routeCfg); err != nil {
		a.emitLog(fmt.Sprintf("WARN: route.Setup: %v", err), "error")
	} else {
		cnt := route.ActiveRouteCount()
		a.emitLog(fmt.Sprintf("Маршрутизация применена: режим=%s, RuDirect=%v, активных маршрутов=%d",
			routeCfg.Mode, routeCfg.RuDirect, cnt), "info")
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

// GetRoutingConfig возвращает настройки раздельного туннелирования
func (a *App) GetRoutingConfig() map[string]interface{} {
	cfg, err := LoadConfig()
	if err != nil {
		cfg = &LocalConfig{RuDirect: true, SplitMode: "bypass"}
	}
	res := map[string]interface{}{
		"ru_direct":      cfg.RuDirect,
		"split_mode":     cfg.SplitMode,
		"split_cidrs":    cfg.SplitCIDRs,
		"split_domains":  cfg.SplitDomains,
		"dead_drop_urls": cfg.DeadDropURLs,
	}
	if cfg.WhitelistFallback != nil {
		res["whitelist_enabled"] = cfg.WhitelistFallback.Enabled
		res["whitelist_mode"] = cfg.WhitelistFallback.Mode
		res["vk_hashes"] = cfg.WhitelistFallback.VKHashes
		res["dion_slugs"] = cfg.WhitelistFallback.DionSlugs
	}
	return res
}

// SaveRoutingConfig сохраняет настройки раздельного туннелирования
func (a *App) SaveRoutingConfig(ruDirect bool, mode string, cidrs []string, domains []string) error {
	cfg, err := LoadConfig()
	if err != nil {
		cfg = &LocalConfig{}
	}
	cfg.RuDirect = ruDirect
	if mode == "only" {
		cfg.SplitMode = "only"
	} else {
		cfg.SplitMode = "bypass"
	}
	cfg.SplitCIDRs = cidrs
	cfg.SplitDomains = domains
	return SaveConfig(cfg)
}

// SaveAdvancedConfig сохраняет настройки Dead Drop и Whitelist Fallback
func (a *App) SaveAdvancedConfig(deadDropURLs []string, wlEnabled bool, wlMode string, vkHashes []string, dionSlugs []string) error {
	cfg, err := LoadConfig()
	if err != nil {
		cfg = &LocalConfig{}
	}
	cfg.DeadDropURLs = deadDropURLs
	if cfg.WhitelistFallback == nil {
		cfg.WhitelistFallback = &WhitelistFallbackConfig{
			GatewayPort: 56005,
			TurnBackup:  "193.203.43.1:19302",
			TurnRealm:   "vkcalls",
		}
	}
	cfg.WhitelistFallback.Enabled = wlEnabled
	if wlMode == "always" {
		cfg.WhitelistFallback.Mode = "always"
	} else {
		cfg.WhitelistFallback.Mode = "auto"
	}
	cfg.WhitelistFallback.VKHashes = vkHashes
	cfg.WhitelistFallback.DionSlugs = dionSlugs
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

func fetchParams(key string, deadDropURLs []string, logFn func(string, string)) (*ParamsResponse, string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	cachePath := filepath.Join(filepath.Dir(configPath), "params_cache.json")
	fetcher := NewDeadDropFetcher(cachePath)

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

		if paramsResp.Status != "answered" && len(paramsResp.Entries) == 0 {
			lastErr = fmt.Errorf("сервер вернул статус: %s", paramsResp.Status)
			continue
		}

		_ = fetcher.SaveCache(&paramsResp)
		return &paramsResp, "api", nil
	}

	// 2. Резервный канал Dead Drop (Яндекс Диск / VK Cloud / S3)
	if len(deadDropURLs) > 0 {
		if logFn != nil {
			logFn(fmt.Sprintf("Основной API недоступен (%v), опрос Dead Drop (%d адресов)...", lastErr, len(deadDropURLs)), "warning")
		}
		ddResp, ddSource, ddErr := fetcher.Fetch(context.Background(), deadDropURLs)
		if ddErr == nil && ddResp != nil && len(ddResp.Entries) > 0 {
			_ = fetcher.SaveCache(ddResp)
			return ddResp, "deaddrop:" + ddSource, nil
		}
		if logFn != nil {
			logFn(fmt.Sprintf("Dead Drop не ответил: %v", ddErr), "warning")
		}
	}

	// 3. Локальный кеш предыдущей успешной сессии
	if cachedResp, cacheErr := fetcher.LoadCache(); cacheErr == nil && cachedResp != nil && len(cachedResp.Entries) > 0 {
		if logFn != nil {
			logFn("Использование параметров из сохранённого локального кеша", "warning")
		}
		return cachedResp, "cache", nil
	}

	return nil, "", fmt.Errorf("не удалось получить параметры (API: %v)", lastErr)
}

func cleanVKHash(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		const marker = "/call/join/"
		at := strings.Index(low, marker)
		if at < 0 {
			return ""
		}
		s = s[at+len(marker):]
	}
	for _, stop := range []string{"?", "#", "/"} {
		if at := strings.Index(s, stop); at >= 0 {
			s = s[:at]
		}
	}
	return strings.TrimSpace(s)
}

func cleanDionSlug(raw string) string {
	s := strings.TrimSpace(raw)
	for _, p := range []string{"https://dion.vc/event/", "http://dion.vc/event/", "dion.vc/event/"} {
		s = strings.TrimPrefix(s, p)
	}
	if i := strings.IndexAny(s, "?#/"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func buildLadder(resp *ParamsResponse, key string, wlCfg *WhitelistFallbackConfig) (*engine.Ladder, string, error) {
	if wlCfg != nil && wlCfg.Enabled && strings.ToLower(wlCfg.Mode) == "always" {
		ladder := engine.NewLadder()
		ladder.SetSecondRoundMs(3000)
		turn := wlCfg.TurnBackup
		if turn == "" {
			turn = "193.203.43.1:19302"
		}
		realm := wlCfg.TurnRealm
		if realm == "" {
			realm = "vkcalls"
		}
		port := int32(wlCfg.GatewayPort)
		if port <= 0 {
			port = 56005
		}
		ladder.AddRelay("R_WHITELIST", turn, realm, port, wlCfg.UseDTLS)
		hasCreds := false
		for _, h := range wlCfg.VKHashes {
			if ch := cleanVKHash(h); ch != "" {
				ladder.AddRelayHash(ch)
				hasCreds = true
			}
		}
		for _, s := range wlCfg.DionSlugs {
			if cs := cleanDionSlug(s); cs != "" {
				ladder.AddRelayDion(cs)
				hasCreds = true
			}
		}
		if wlCfg.RelayTarget != "" {
			ladder.SetRelayTarget(wlCfg.RelayTarget)
		}
		if !hasCreds {
			return nil, "", fmt.Errorf("whitelist fallback режим active='always', но не указаны ссылки/хеши VK Calls или DION")
		}
		gw := "crp.guseynovia.ru"
		if len(resp.Entries) > 0 && resp.Entries[0].Host != "" {
			gw = resp.Entries[0].Host
		}
		return ladder, gw, nil
	}

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
		case "xhttp":
			sniCSV := strings.Join(entry.SNIs, ",")
			if sniCSV == "" {
				sniCSV = entry.SNI
			}
			target := "127.0.0.1:56006"
			path := entry.Path
			if path == "" {
				path = "/s1r1nxhttp2026k"
			}
			mode := entry.Mode
			if mode == "" {
				mode = "stream-one"
			}
			err := ladder.AddXHTTP(
				entry.ID,
				entry.Host,
				int32(entry.Port),
				vlessUUID,
				entry.PBK,
				entry.SID,
				entry.SNI,
				target,
				sniCSV,
				path,
				mode,
			)
			if err != nil {
				return nil, "", fmt.Errorf("AddXHTTP %s: %w", entry.ID, err)
			}
			hasEntries = true
		case "relay":
			turn := entry.TurnBackup
			realm := entry.TurnRealm
			port := int32(entry.Port)
			if wlCfg != nil {
				if turn == "" {
					turn = wlCfg.TurnBackup
				}
				if realm == "" {
					realm = wlCfg.TurnRealm
				}
				if port <= 0 {
					port = int32(wlCfg.GatewayPort)
				}
			}
			if port <= 0 {
				port = 56005
			}
			ladder.AddRelay(entry.ID, turn, realm, port, entry.DTLS)
			for _, h := range entry.VKHashes {
				if ch := cleanVKHash(h); ch != "" {
					ladder.AddRelayHash(ch)
				}
			}
			for _, s := range entry.DionSlugs {
				if cs := cleanDionSlug(s); cs != "" {
					ladder.AddRelayDion(cs)
				}
			}
			hasEntries = true
		}
	}

	// Если настроен Whitelist Fallback, добавляем аварийный релей в конец
	if wlCfg != nil && wlCfg.Enabled && (len(wlCfg.VKHashes) > 0 || len(wlCfg.DionSlugs) > 0 || resp.Relay.Enabled) {
		turn := wlCfg.TurnBackup
		if turn == "" {
			turn = resp.Relay.TURNBackup
		}
		if turn == "" {
			turn = "193.203.43.1:19302"
		}
		realm := wlCfg.TurnRealm
		if realm == "" {
			realm = resp.Relay.TURNRealm
		}
		if realm == "" {
			realm = "vkcalls"
		}
		port := int32(wlCfg.GatewayPort)
		if port <= 0 {
			port = int32(resp.Relay.Port)
		}
		if port <= 0 {
			port = 56005
		}
		dtls := wlCfg.UseDTLS || resp.Relay.DTLS

		ladder.AddRelay("R_WHITELIST", turn, realm, port, dtls)
		for _, h := range wlCfg.VKHashes {
			if ch := cleanVKHash(h); ch != "" {
				ladder.AddRelayHash(ch)
			}
		}
		for _, s := range wlCfg.DionSlugs {
			if cs := cleanDionSlug(s); cs != "" {
				ladder.AddRelayDion(cs)
			}
		}
		for _, h := range resp.Relay.VKHashes {
			if ch := cleanVKHash(h); ch != "" {
				ladder.AddRelayHash(ch)
			}
		}
		for _, s := range resp.Relay.DionSlugs {
			if cs := cleanDionSlug(s); cs != "" {
				ladder.AddRelayDion(cs)
			}
		}
		if resp.Relay.Target != "" {
			ladder.SetRelayTarget(resp.Relay.Target)
		}
		if wlCfg.RelayTarget != "" {
			ladder.SetRelayTarget(wlCfg.RelayTarget)
		}
		hasEntries = true
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
