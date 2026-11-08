package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sirin/engine"
)

const defaultVlessUUID = "5f26ebc3-6eb0-45fb-a490-c8c92b86b644"

type ParamsResponse struct {
	Version int `json:"version"`
	Ladder  struct {
		SecondRoundMs int `json:"second_round_ms"`
	} `json:"ladder"`
	Relay   *ParamsRelay  `json:"relay,omitempty"`
	Entries []ParamsEntry `json:"entries"`
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

type ParamsEntry struct {
	ID         string   `json:"id"`
	Transport  string   `json:"transport"`
	Host       string   `json:"host"`
	Port       int      `json:"port"`
	DTLS       bool     `json:"dtls"`
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

// buildLadder строит структуру engine.Ladder и список IP/хостов серверов.
// Иерархия источников конфигурации:
// 1. При prefer_static_ladder = true — статический список [[ladder]] из client.toml.
// 2. Основной API (/v1/params). При успехе обновляет локальный кеш.
// 3. Резервный канал Dead Drop (Яндекс Диск / VK Cloud / S3). При успехе обновляет локальный кеш.
// 4. Локальный кеш предыдущей успешной сессии (/tmp/sirin_params_cache.json).
// 5. Статический fallback [[ladder]] из client.toml.
// 6. Whitelist Fallback (аварийный канал через WebRTC TURN VK/DION при блокировках).
func buildLadder(cfg *Config) (*engine.Ladder, string, []string, error) {
	wlCfg := cfg.GetWhitelistFallbackConfig()

	if cfg.PreferStaticLadder && len(cfg.StaticLadder) > 0 {
		log.Printf("[ladder] Принудительное использование статичного ladder-конфига (%d узлов)", len(cfg.StaticLadder))
		return buildFromStatic(cfg.StaticLadder, cfg.Gateway, wlCfg)
	}

	cachePath := cfg.GetParamsCachePath()
	fetcher := NewDeadDropFetcher(cachePath)

	var resp *ParamsResponse
	var err error

	useAPI := cfg.Password != "" && cfg.APIURL != "" &&
		cfg.APIURL != "none" && cfg.APIURL != "off" && cfg.APIURL != "disabled"

	// 1. Запрос к основному API
	if useAPI {
		log.Printf("[ladder] Запрос конфигурации с API: %s...", cfg.APIURL)
		resp, err = fetchParams(cfg.APIURL, cfg.Password)
		if err != nil {
			log.Printf("[ladder] Внимание: не удалось получить /v1/params: %v (пробуем Dead Drop...)", err)
		} else if resp != nil && len(resp.Entries) > 0 {
			// Сохраняем свежие параметры в локальный кеш
			_ = fetcher.SaveCache(resp)
			return buildFromParams(resp, cfg.Gateway, wlCfg)
		}
	}

	// 2. Запрос к резервному каналу Dead Drop (Яндекс Диск / S3)
	deadDropURLs := cfg.GetDeadDropURLs()
	if len(deadDropURLs) > 0 {
		log.Printf("[ladder] Попытка получения конфигурации через Dead Drop (%d источников)...", len(deadDropURLs))
		ddResp, ddURL, ddErr := fetcher.Fetch(context.Background(), deadDropURLs)
		if ddErr != nil {
			log.Printf("[ladder] Внимание: Dead Drop недоступен: %v", ddErr)
		} else if ddResp != nil && len(ddResp.Entries) > 0 {
			log.Printf("[ladder] Конфигурация успешно получена через Dead Drop: %s", ddURL)
			return buildFromParams(ddResp, cfg.Gateway, wlCfg)
		}
	}

	// 3. Попытка загрузки из локального кеша предыдущих сессий
	if cachedResp, cacheErr := fetcher.LoadCache(); cacheErr == nil && cachedResp != nil && len(cachedResp.Entries) > 0 {
		log.Printf("[ladder] Использование сохранённого локального кеша (%d узлов из %s)", len(cachedResp.Entries), cachePath)
		return buildFromParams(cachedResp, cfg.Gateway, wlCfg)
	}

	// 4. Финальный fallback на статичный ladder из client.toml
	if len(cfg.StaticLadder) > 0 {
		log.Printf("[ladder] Использование статичного fallback-конфига (%d узлов)", len(cfg.StaticLadder))
		return buildFromStatic(cfg.StaticLadder, cfg.Gateway, wlCfg)
	}

	// 5. Если списков узлов нет вообще, но настроен аварийный Whitelist Fallback
	if wlCfg.Enabled && (len(wlCfg.VKHashes) > 0 || len(wlCfg.DionSlugs) > 0) {
		log.Printf("[ladder] Внимание: узлы не найдены, запускаем аварийный Whitelist Fallback (WebRTC TURN)...")
		return buildFromWhitelistOnly(cfg.Gateway, wlCfg)
	}

	return nil, "", nil, fmt.Errorf("нет доступных узлов для подключения (API, Dead Drop, кеш недоступны и секция [[ladder]] пуста)")
}

func fetchParams(apiBase, key string) (*ParamsResponse, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	cleanBase := strings.TrimRight(apiBase, "/")
	urls := []string{
		fmt.Sprintf("%s/v1/params?key=%s", cleanBase, url.QueryEscape(key)),
		fmt.Sprintf("https://crp.guseynovia.ru/v1/params?key=%s", url.QueryEscape(key)),
	}

	var lastErr error
	for _, u := range urls {
		paramsResp, err := fetchOneParamsURL(client, u)
		if err != nil {
			lastErr = err
			continue
		}
		return paramsResp, nil
	}

	return nil, fmt.Errorf("ошибка запроса к API: %w", lastErr)
}

// fetchOneParamsURL выполняет один HTTP-запрос к API параметров.
// Выделена в отдельную функцию, чтобы defer resp.Body.Close() срабатывал
// немедленно после обработки, а не откладывался до return из fetchParams.
func fetchOneParamsURL(client *http.Client, u string) (*ParamsResponse, error) {
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d from %s", resp.StatusCode, u)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var paramsResp ParamsResponse
	if err := json.Unmarshal(body, &paramsResp); err != nil {
		return nil, err
	}

	return &paramsResp, nil
}

func buildFromParams(resp *ParamsResponse, fallbackGw string, wlCfg WhitelistFallbackConfig) (*engine.Ladder, string, []string, error) {
	if wlCfg.Enabled && strings.ToLower(wlCfg.Mode) == "always" {
		log.Printf("[ladder] Режим Whitelist Fallback: ALWAYS (подключение только через отечественные WebRTC TURN VK/DION)")
		return buildFromWhitelistOnly(fallbackGw, wlCfg)
	}

	ladder := engine.NewLadder()
	if resp.Ladder.SecondRoundMs > 0 {
		ladder.SetSecondRoundMs(int32(resp.Ladder.SecondRoundMs))
	} else {
		ladder.SetSecondRoundMs(3000)
	}

	var hosts []string
	gateway := fallbackGw
	if gateway == "" {
		gateway = "crp.guseynovia.ru"
	}
	if len(resp.Entries) > 0 && resp.Entries[0].Host != "" {
		gateway = resp.Entries[0].Host
	}

	hasEntries := false
	for _, entry := range resp.Entries {
		if entry.Host != "" {
			hosts = append(hosts, entry.Host)
		}
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
				defaultVlessUUID,
				entry.PBK,
				entry.SID,
				entry.SNI,
				target,
				sniCSV,
			)
			if err != nil {
				return nil, "", nil, fmt.Errorf("AddVLESS %s: %w", entry.ID, err)
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
			err := ladder.AddXHTTP(
				entry.ID,
				entry.Host,
				int32(entry.Port),
				defaultVlessUUID,
				entry.PBK,
				entry.SID,
				entry.SNI,
				target,
				sniCSV,
				path,
				entry.Mode,
			)
			if err != nil {
				return nil, "", nil, fmt.Errorf("AddXHTTP %s: %w", entry.ID, err)
			}
			hasEntries = true
		case "relay":
			turn := entry.TurnBackup
			if turn == "" {
				turn = wlCfg.TurnBackup
			}
			realm := entry.TurnRealm
			if realm == "" {
				realm = wlCfg.TurnRealm
			}
			port := int32(entry.Port)
			if port <= 0 {
				port = int32(wlCfg.GatewayPort)
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

	// Добавляем аварийный Whitelist Fallback узел в конец лестницы (Auto fallback)
	if wlCfg.Enabled && (len(wlCfg.VKHashes) > 0 || len(wlCfg.DionSlugs) > 0 || resp.Relay != nil) {
		turn := wlCfg.TurnBackup
		realm := wlCfg.TurnRealm
		port := int32(wlCfg.GatewayPort)
		dtls := wlCfg.UseDTLS
		target := wlCfg.RelayTarget
		if resp.Relay != nil {
			if resp.Relay.Port > 0 {
				port = int32(resp.Relay.Port)
			}
			if resp.Relay.TURNBackup != "" {
				turn = resp.Relay.TURNBackup
			}
			if resp.Relay.TURNRealm != "" {
				realm = resp.Relay.TURNRealm
			}
			if resp.Relay.Target != "" {
				target = resp.Relay.Target
			}
			dtls = resp.Relay.DTLS
		}

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
		if resp.Relay != nil {
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
		}
		if target != "" {
			ladder.SetRelayTarget(target)
		}
		hasEntries = true
	}

	if !hasEntries {
		return nil, "", nil, fmt.Errorf("в ответе API нет поддерживаемых узлов")
	}

	return ladder, gateway, hosts, nil
}

func buildFromStatic(entries []LadderEntry, fallbackGw string, wlCfg WhitelistFallbackConfig) (*engine.Ladder, string, []string, error) {
	if wlCfg.Enabled && strings.ToLower(wlCfg.Mode) == "always" {
		log.Printf("[ladder] Режим Whitelist Fallback: ALWAYS (подключение только через отечественные WebRTC TURN VK/DION)")
		return buildFromWhitelistOnly(fallbackGw, wlCfg)
	}

	ladder := engine.NewLadder()
	ladder.SetSecondRoundMs(3000)

	var hosts []string
	gateway := fallbackGw
	if gateway == "" {
		gateway = "crp.guseynovia.ru"
	}
	if len(entries) > 0 && entries[0].Host != "" {
		gateway = entries[0].Host
	}

	hasEntries := false
	for _, entry := range entries {
		if entry.Host != "" {
			hosts = append(hosts, entry.Host)
		}
		switch strings.ToLower(entry.Transport) {
		case "udp", "":
			ladder.AddDirect(entry.Name, entry.Host, int32(entry.Port), entry.DTLS)
			hasEntries = true
		case "vless":
			sniCSV := strings.Join(entry.SNIs, ",")
			if sniCSV == "" {
				sniCSV = entry.SNI
			}
			target := "127.0.0.1:56006"
			err := ladder.AddVLESS(
				entry.Name,
				entry.Host,
				int32(entry.Port),
				defaultVlessUUID,
				entry.PBK,
				entry.SID,
				entry.SNI,
				target,
				sniCSV,
			)
			if err != nil {
				return nil, "", nil, fmt.Errorf("AddVLESS %s: %w", entry.Name, err)
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
			err := ladder.AddXHTTP(
				entry.Name,
				entry.Host,
				int32(entry.Port),
				defaultVlessUUID,
				entry.PBK,
				entry.SID,
				entry.SNI,
				target,
				sniCSV,
				path,
				entry.Mode,
			)
			if err != nil {
				return nil, "", nil, fmt.Errorf("AddXHTTP %s: %w", entry.Name, err)
			}
			hasEntries = true
		case "relay":
			turn := entry.TurnBackup
			if turn == "" {
				turn = wlCfg.TurnBackup
			}
			realm := entry.TurnRealm
			if realm == "" {
				realm = wlCfg.TurnRealm
			}
			port := int32(entry.Port)
			if port <= 0 {
				port = int32(wlCfg.GatewayPort)
			}
			ladder.AddRelay(entry.Name, turn, realm, port, entry.DTLS)
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

	// Аварийный Whitelist Fallback в конец статичного конфига
	if wlCfg.Enabled && (len(wlCfg.VKHashes) > 0 || len(wlCfg.DionSlugs) > 0) {
		turn := wlCfg.TurnBackup
		realm := wlCfg.TurnRealm
		port := int32(wlCfg.GatewayPort)
		ladder.AddRelay("R_WHITELIST", turn, realm, port, wlCfg.UseDTLS)
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
		if wlCfg.RelayTarget != "" {
			ladder.SetRelayTarget(wlCfg.RelayTarget)
		}
		hasEntries = true
	}

	if !hasEntries {
		return nil, "", nil, fmt.Errorf("в static ladder нет валидных узлов")
	}

	return ladder, gateway, hosts, nil
}

func buildFromWhitelistOnly(fallbackGw string, wlCfg WhitelistFallbackConfig) (*engine.Ladder, string, []string, error) {
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
		return nil, "", nil, fmt.Errorf("whitelist fallback режим активен, но не указаны ссылки/хеши VK Calls или DION")
	}

	gw := fallbackGw
	if gw == "" {
		gw = "crp.guseynovia.ru"
	}

	return ladder, gw, []string{gw}, nil
}
