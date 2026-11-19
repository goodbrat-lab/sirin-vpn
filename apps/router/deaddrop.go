package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SignedDeadDropEnvelope — подписанный Ed25519 конверт для Dead Drop JSON.
// Подпись вычисляется от точных сырых байтов поля Payload.
type SignedDeadDropEnvelope struct {
	Payload   json.RawMessage `json:"payload"`
	Signature string          `json:"signature"`
	Timestamp int64           `json:"timestamp,omitempty"`
}

// SignDeadDropPayload подписывает канонизированный (compact) JSON-payload приватным ключом Ed25519
// и возвращает сериализованный SignedDeadDropEnvelope.
func SignDeadDropPayload(payload []byte, priv ed25519.PrivateKey) ([]byte, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("неверная длина приватного ключа Ed25519")
	}
	var canonical bytes.Buffer
	if err := json.Compact(&canonical, payload); err != nil {
		return nil, fmt.Errorf("невалидный JSON payload для подписи: %w", err)
	}
	sig := ed25519.Sign(priv, canonical.Bytes())
	env := SignedDeadDropEnvelope{
		Payload:   json.RawMessage(canonical.Bytes()),
		Signature: base64.StdEncoding.EncodeToString(sig),
		Timestamp: time.Now().Unix(),
	}
	return json.MarshalIndent(env, "", "  ")
}

// YandexDiskDownloadResponse описывает ответ публичного REST API Яндекс Диска
// на запрос получения прямой ссылки на скачивание публичного ресурса.
type YandexDiskDownloadResponse struct {
	Href      string `json:"href"`
	Method    string `json:"method"`
	Templated bool   `json:"templated"`
}

// DeadDropFetcher отвечает за резервную доставку конфигурации узлов
// через недоступные для блокировки ТСПУ отечественные хранилища (Яндекс Диск, VK Cloud S3, Yandex Cloud S3).
type DeadDropFetcher struct {
	Client    *http.Client
	CachePath string
	PublicKey ed25519.PublicKey // если задан — требуется валидная Ed25519-подпись конверта
}

// NewDeadDropFetcher создаёт экземпляр загрузчика с тайм-аутом и путём к кешу.
func NewDeadDropFetcher(cachePath string) *DeadDropFetcher {
	return &DeadDropFetcher{
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
		CachePath: cachePath,
	}
}

// IsYandexDiskLink проверяет, является ли ссылка публичной ссылкой Яндекс Диска.
func IsYandexDiskLink(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Host)
	return host == "disk.yandex.ru" ||
		host == "disk.yandex.com" ||
		host == "yadi.sk" ||
		strings.HasSuffix(host, ".disk.yandex.ru") ||
		strings.HasSuffix(host, ".yadi.sk")
}

// ResolveYandexDiskDownloadURL запрашивает через официальное открытое REST API Яндекс Диска
// прямую CDN-ссылку на скачивание файла по его публичному URL без необходимости авторизации.
func ResolveYandexDiskDownloadURL(ctx context.Context, client *http.Client, diskURL string) (string, error) {
	apiEndpoint := "https://cloud-api.yandex.net/v1/disk/public/resources/download?public_key=" + url.QueryEscape(diskURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiEndpoint, nil)
	if err != nil {
		return "", fmt.Errorf("создание запроса к API Яндекса: %w", err)
	}
	req.Header.Set("User-Agent", "SirinVPN-DeadDrop/1.6")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("запрос к cloud-api.yandex.net: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("API Яндекс Диска вернул HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var yaResp YandexDiskDownloadResponse
	if err := json.NewDecoder(resp.Body).Decode(&yaResp); err != nil {
		return "", fmt.Errorf("парсинг JSON ответа Яндекс Диска: %w", err)
	}

	if yaResp.Href == "" {
		return "", errors.New("API Яндекс Диска вернул пустой href для скачивания")
	}

	return yaResp.Href, nil
}

// FetchPayload выполняет HTTP GET запрос с лимитом размера (до 2 МБ) для защиты памяти.
func FetchPayload(ctx context.Context, client *http.Client, targetURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("создание запроса к %s: %w", targetURL, err)
	}
	req.Header.Set("User-Agent", "SirinVPN-DeadDrop/1.6")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("загрузка %s: %w", targetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d от %s", resp.StatusCode, targetURL)
	}

	// Ограничиваем размер файла 2 МБ (достаточно для любого списка узлов)
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("чтение тела ответа %s: %w", targetURL, err)
	}

	return data, nil
}

// ParseDeadDropPayload валидирует и парсит JSON с параметрами лестницы.
// Для проверки криптографической подписи используйте ParseSignedDeadDropPayload.
func ParseDeadDropPayload(data []byte) (*ParamsResponse, error) {
	return ParseSignedDeadDropPayload(data, nil)
}

// ParseSignedDeadDropPayload проверяет Ed25519-подпись конверта (если задан pub)
// и парсит JSON с параметрами лестницы.
func ParseSignedDeadDropPayload(data []byte, pub ed25519.PublicKey) (*ParamsResponse, error) {
	var env SignedDeadDropEnvelope
	if err := json.Unmarshal(data, &env); err == nil && len(env.Payload) > 0 && env.Signature != "" {
		if len(pub) == ed25519.PublicKeySize {
			sig, decErr := base64.StdEncoding.DecodeString(strings.TrimSpace(env.Signature))
			if decErr != nil || len(sig) != ed25519.SignatureSize {
				return nil, errors.New("некорректный формат Ed25519-подписи Dead Drop")
			}
			var canonical bytes.Buffer
			if err := json.Compact(&canonical, env.Payload); err != nil {
				return nil, fmt.Errorf("повреждённый JSON payload Dead Drop: %w", err)
			}
			if !ed25519.Verify(pub, canonical.Bytes(), sig) {
				return nil, errors.New("подпись Ed25519 Dead Drop недействительна (возможна подмена конфига)")
			}
		}
		data = env.Payload
	} else if len(pub) == ed25519.PublicKeySize {
		return nil, errors.New("отсутствует обязательная Ed25519-подпись Dead Drop")
	}

	var resp ParamsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("ошибка разбора JSON параметров: %w", err)
	}

	if len(resp.Entries) == 0 {
		return nil, errors.New("список узлов entries пуст")
	}

	return &resp, nil
}

// Fetch пытается загрузить параметры лестницы из переданных URL.
// Если ссылка указывает на Яндекс Диск, она автоматически резолвится через cloud-api.yandex.net.
// Если это прямой S3 или HTTPS URL, скачивается напрямую.
func (f *DeadDropFetcher) Fetch(ctx context.Context, urls []string) (*ParamsResponse, string, error) {
	var lastErr error
	for _, rawURL := range urls {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" {
			continue
		}

		targetDownloadURL := rawURL
		if IsYandexDiskLink(rawURL) {
			log.Printf("[deaddrop] Резолвинг публичной ссылки Яндекс Диска: %s", rawURL)
			directURL, err := ResolveYandexDiskDownloadURL(ctx, f.Client, rawURL)
			if err != nil {
				log.Printf("[deaddrop] Ошибка резолвинга Яндекс Диска (%s): %v", rawURL, err)
				lastErr = err
				continue
			}
			targetDownloadURL = directURL
		}

		log.Printf("[deaddrop] Загрузка актуального списка узлов: %s...", rawURL)
		data, err := FetchPayload(ctx, f.Client, targetDownloadURL)
		if err != nil {
			log.Printf("[deaddrop] Ошибка скачивания с %s: %v", rawURL, err)
			lastErr = err
			continue
		}

		paramsResp, err := ParseSignedDeadDropPayload(data, f.PublicKey)
		if err != nil {
			log.Printf("[deaddrop] Невалидный payload от %s: %v", rawURL, err)
			lastErr = err
			continue
		}

		log.Printf("[deaddrop] Успешно получено %d узлов через Dead Drop (%s)", len(paramsResp.Entries), rawURL)

		// Сохраняем в локальный кеш при успехе
		if f.CachePath != "" {
			if err := f.SaveCache(paramsResp); err != nil {
				log.Printf("[deaddrop] Предупреждение: ошибка записи кеша (%s): %v", f.CachePath, err)
			}
		}

		return paramsResp, rawURL, nil
	}

	return nil, "", fmt.Errorf("все источники Dead Drop недоступны: %w", lastErr)
}

// SaveCache сохраняет параметры в локальный кеш-файл.
func (f *DeadDropFetcher) SaveCache(resp *ParamsResponse) error {
	if f.CachePath == "" {
		return nil
	}
	dir := filepath.Dir(f.CachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(f.CachePath, data, 0644)
}

// LoadCache загружает параметры из локального кеш-файла (если доступен).
func (f *DeadDropFetcher) LoadCache() (*ParamsResponse, error) {
	if f.CachePath == "" {
		return nil, errors.New("путь к кешу не задан")
	}
	data, err := os.ReadFile(f.CachePath)
	if err != nil {
		return nil, err
	}
	paramsResp, err := ParseDeadDropPayload(data)
	if err != nil {
		return nil, fmt.Errorf("кеш-файл поврежден: %w", err)
	}
	return paramsResp, nil
}
