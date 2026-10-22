package main

import (
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
	Entries []ParamsEntry `json:"entries"`
}

type ParamsEntry struct {
	ID        string   `json:"id"`
	Transport string   `json:"transport"`
	Host      string   `json:"host"`
	Port      int      `json:"port"`
	DTLS      bool     `json:"dtls"`
	SNI       string   `json:"sni"`
	SNIs      []string `json:"snis"`
	PBK       string   `json:"pbk"`
	SID       string   `json:"sid"`
	Path      string   `json:"path,omitempty"`
	Mode      string   `json:"mode,omitempty"`
}

// buildLadder строит структуру engine.Ladder и список IP/хостов серверов.
// Сначала пробует загрузить свежие параметры из /v1/params.
// При сбое или отсутствии сети переходит на статичный ladder из client.toml.
func buildLadder(cfg *Config) (*engine.Ladder, string, []string, error) {
	var resp *ParamsResponse
	var err error

	if cfg.Password != "" && cfg.APIURL != "" {
		log.Printf("[ladder] Запрос конфигурации с API: %s...", cfg.APIURL)
		resp, err = fetchParams(cfg.APIURL, cfg.Password)
		if err != nil {
			log.Printf("[ladder] Внимание: не удалось получить /v1/params: %v (переключаемся на fallback)", err)
		}
	}

	if resp != nil && len(resp.Entries) > 0 {
		return buildFromParams(resp)
	}

	if len(cfg.StaticLadder) > 0 {
		log.Printf("[ladder] Использование статичного fallback-конфига (%d узлов)", len(cfg.StaticLadder))
		return buildFromStatic(cfg.StaticLadder, cfg.Gateway)
	}

	return nil, "", nil, fmt.Errorf("нет доступных узлов для подключения (API недоступен и секция [[ladder]] пуста)")
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

		return &paramsResp, nil
	}

	return nil, fmt.Errorf("ошибка запроса к API: %w", lastErr)
}

func buildFromParams(resp *ParamsResponse) (*engine.Ladder, string, []string, error) {
	ladder := engine.NewLadder()
	if resp.Ladder.SecondRoundMs > 0 {
		ladder.SetSecondRoundMs(int32(resp.Ladder.SecondRoundMs))
	} else {
		ladder.SetSecondRoundMs(3000)
	}

	var hosts []string
	gateway := "crp.guseynovia.ru"
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
		}
	}

	if !hasEntries {
		return nil, "", nil, fmt.Errorf("в ответе API нет поддерживаемых узлов")
	}

	return ladder, gateway, hosts, nil
}

func buildFromStatic(entries []LadderEntry, fallbackGw string) (*engine.Ladder, string, []string, error) {
	ladder := engine.NewLadder()
	ladder.SetSecondRoundMs(3000)

	var hosts []string
	gateway := fallbackGw
	if gateway == "" && len(entries) > 0 {
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
		}
	}

	if !hasEntries {
		return nil, "", nil, fmt.Errorf("в static ladder нет валидных узлов")
	}

	return ladder, gateway, hosts, nil
}
