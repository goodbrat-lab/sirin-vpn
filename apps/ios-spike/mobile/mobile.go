package mobile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"sirin/engine"
	"sirin/engine/desktop"
)

func init() {
	// Жесткий лимит памяти для соответствия 50 MiB ограничению iOS Network Extension
	debug.SetMemoryLimit(38 << 20) // 38 MiB
	debug.SetGCPercent(25)         // Агрессивный сборщик мусора
}

// PacketFlow — интерфейс для передачи пакетов между Network Extension и Go-движком
type PacketFlow interface {
	ReadBatch() []byte
	WriteBatch(p []byte) bool
	Close()
}

// Logger — интерфейс логирования в Swift
type Logger interface {
	Log(line string)
}

type flowAdapter struct {
	flow PacketFlow
}

func (a *flowAdapter) ReadBatch() []byte {
	if a.flow == nil {
		return nil
	}
	return a.flow.ReadBatch()
}

func (a *flowAdapter) WriteBatch(p []byte) bool {
	if a.flow == nil {
		return false
	}
	return a.flow.WriteBatch(p)
}

func (a *flowAdapter) Close() {
	if a.flow != nil {
		a.flow.Close()
	}
}

var (
	mu     sync.Mutex
	cancel context.CancelFunc
	handle *desktop.Handle
)

// ParamsResponse — формат ответа /v1/params
type ParamsResponse struct {
	Status string `json:"status"`
	Ladder struct {
		SecondRoundMs int `json:"second_round_ms"`
	} `json:"ladder"`
	Entries []struct {
		ID        string   `json:"id"`
		Host      string   `json:"host"`
		Port      int      `json:"port"`
		Transport string   `json:"transport"`
		DTLS      bool     `json:"dtls"`
		SNI       string   `json:"sni"`
		SNIs      []string `json:"snis"`
		PBK       string   `json:"pbk"`
		SID       string   `json:"sid"`
	} `json:"entries"`
}

// SirinStart поднимает туннель с переданным UUID и адаптером пакетов
func SirinStart(uuid string, flow PacketFlow, logger Logger) error {
	mu.Lock()
	defer mu.Unlock()

	if cancel != nil {
		return errors.New("сессия уже запущена")
	}

	log := func(msg string) {
		if logger != nil {
			logger.Log(msg)
		}
	}

	log("Sirin iOS Spike: Запрос параметров сервера...")
	params, err := fetchParams(uuid)
	if err != nil {
		log(fmt.Sprintf("Ошибка получения параметров: %v", err))
		return err
	}

	ladder, gateway, err := buildLadder(params)
	if err != nil {
		log(fmt.Sprintf("Ошибка сборки лестницы: %v", err))
		return err
	}

	ctx, c := context.WithCancel(context.Background())
	cancel = c

	cfg := desktop.Config{
		Gateway:        gateway,
		CachedFallback: gateway,
		Ladder:         ladder,
		Password:       uuid,
		DeviceID:       "ios-spike-" + fmt.Sprintf("%x", time.Now().UnixNano()&0xFFFFFF),
		ClampMSS:       true,
		TunnelDNS:      desktop.DefaultTunnelDNS,
		LogFunc:        log,
	}

	log("Sirin iOS Spike: Запуск сессии через desktop.Start...")
	h, events, err := desktop.Start(ctx, cfg, &flowAdapter{flow: flow})
	if err != nil {
		cancel()
		cancel = nil
		log(fmt.Sprintf("Ошибка старта движка: %v", err))
		return err
	}

	handle = h

	go func() {
		for ev := range events {
			log(fmt.Sprintf("[%s] %s", ev.Kind, ev.Reason))
		}
	}()

	log("Sirin iOS Spike: Туннель поднят")
	return nil
}

// SirinStop останавливает туннель и принудительно освобождает память
func SirinStop() {
	mu.Lock()
	defer mu.Unlock()

	if cancel != nil {
		cancel()
		cancel = nil
	}
	if handle != nil {
		handle.Stop()
		handle = nil
	}
	// Принудительный возврат страниц памяти ОС
	debug.FreeOSMemory()
}

// SirinMemoryUsage возвращает текущий объем аллоцированной памяти в байтах
func SirinMemoryUsage() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Alloc)
}

// SirinStatus возвращает текстовый статус сессии
func SirinStatus() string {
	st := desktop.CurrentStatus()
	return fmt.Sprintf("active=%v winner=%s resolved=%s tx=%d rx=%d",
		st.Active, st.Winner, st.ResolvedVia, st.LastTx, st.LastRx)
}

func fetchParams(key string) (*ParamsResponse, error) {
	client := &http.Client{Timeout: 10 * time.Second}
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
			lastErr = fmt.Errorf("HTTP status %d", resp.StatusCode)
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
			lastErr = fmt.Errorf("status: %s", paramsResp.Status)
			continue
		}

		return &paramsResp, nil
	}

	return nil, fmt.Errorf("не удалось получить параметры: %w", lastErr)
}

func buildLadder(resp *ParamsResponse) (*engine.Ladder, string, error) {
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

	const vlessUUID = "5f26ebc3-6eb0-45fb-a490-c8c92b86b644"

	for _, entry := range resp.Entries {
		switch strings.ToLower(entry.Transport) {
		case "udp", "":
			ladder.AddDirect(entry.ID, entry.Host, int32(entry.Port), entry.DTLS)
		case "vless":
			sniCSV := strings.Join(entry.SNIs, ",")
			if sniCSV == "" {
				sniCSV = entry.SNI
			}
			target := "127.0.0.1:56006"
			_ = ladder.AddVLESS(
				entry.ID,
				entry.Host,
				int32(entry.Port),
				vlessUUID,
				entry.PBK,
				entry.SID,
				entry.SNI,
				target,
				sniCSV,
			)
		}
	}

	return ladder, gateway, nil
}
