package main

import (
	"context"
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

// YandexDiskDownloadResponse описывает ответ открытого REST API Яндекс Диска.
type YandexDiskDownloadResponse struct {
	Href      string `json:"href"`
	Method    string `json:"method"`
	Templated bool   `json:"templated"`
}

// DeadDropFetcher загружает актуальный список узлов ladder через
// резервные каналы (Яндекс Диск или S3-совместимые хранилища),
// находящиеся в белых списках провайдеров РФ и не блокируемые ТСПУ.
type DeadDropFetcher struct {
	Client    *http.Client
	CachePath string
}

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

// ResolveYandexDiskDownloadURL запрашивает через открытое REST API Яндекс Диска
// прямую CDN-ссылку на скачивание файла по его публичному URL без авторизации.
func ResolveYandexDiskDownloadURL(ctx context.Context, client *http.Client, diskURL string) (string, error) {
	apiEndpoint := "https://cloud-api.yandex.net/v1/disk/public/resources/download?public_key=" + url.QueryEscape(diskURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiEndpoint, nil)
	if err != nil {
		return "", fmt.Errorf("создание запроса к API Яндекса: %w", err)
	}
	req.Header.Set("User-Agent", "SirinDesktop-DeadDrop/1.2")

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

// FetchPayload выполняет HTTP GET запрос с лимитом размера (до 2 МБ).
func FetchPayload(ctx context.Context, client *http.Client, targetURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("создание запроса к %s: %w", targetURL, err)
	}
	req.Header.Set("User-Agent", "SirinDesktop-DeadDrop/1.2")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("загрузка %s: %w", targetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d от %s", resp.StatusCode, targetURL)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("чтение тела ответа %s: %w", targetURL, err)
	}

	return data, nil
}

// ParseDeadDropPayload парсит и валидирует ответ параметров.
func ParseDeadDropPayload(data []byte) (*ParamsResponse, error) {
	var resp ParamsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("ошибка разбора JSON параметров: %w", err)
	}

	if len(resp.Entries) == 0 {
		return nil, errors.New("список узлов entries пуст")
	}

	return &resp, nil
}

// Fetch пытается загрузить параметры лестницы из переданных URL адресов.
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

		paramsResp, err := ParseDeadDropPayload(data)
		if err != nil {
			log.Printf("[deaddrop] Невалидный payload от %s: %v", rawURL, err)
			lastErr = err
			continue
		}

		if f.CachePath != "" {
			_ = f.SaveCache(paramsResp)
		}

		return paramsResp, rawURL, nil
	}

	return nil, "", fmt.Errorf("все источники Dead Drop недоступны: %w", lastErr)
}

// SaveCache сохраняет параметры в локальный кеш.
func (f *DeadDropFetcher) SaveCache(resp *ParamsResponse) error {
	if f.CachePath == "" {
		return nil
	}
	dir := filepath.Dir(f.CachePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(f.CachePath, data, 0600)
}

// LoadCache загружает параметры из локального кеш-файла.
func (f *DeadDropFetcher) LoadCache() (*ParamsResponse, error) {
	if f.CachePath == "" {
		return nil, errors.New("путь к кешу не задан")
	}
	data, err := os.ReadFile(f.CachePath)
	if err != nil {
		return nil, err
	}
	return ParseDeadDropPayload(data)
}
