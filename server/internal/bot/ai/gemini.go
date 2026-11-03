package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultModel = "gemini-2.5-flash"
	fallbackModel = "gemini-1.5-flash"
	maxHistoryTurns = 10
)

const systemPrompt = `Ты — умный, дружелюбный и отзывчивый ИИ-ассистент сервиса Sirin VPN (Сирин) в Telegram.
Твоя цель — приятно и естественно общаться с пользователями, отвечать на любые вопросы, помогать с работой VPN, настройкой приложений, обходом ограничений и просто поддерживать живой разговор.

Информация о Sirin VPN:
• Sirin VPN создан для комфортного и стабильного доступа к заблокированным в РФ сайтам и сервисам (YouTube, Instagram, Discord, ChatGPT, Notion, Twitter/X и др.) на высокой скорости без ограничений.
• Технологии: фирменный протокол Noise NK (с маскировкой под трафик звонков VK/WebRTC), VLESS Reality и XHTTP.
• Поддерживаемые платформы:
  - Android (официальное приложение Sirin, с функцией «Починить подключение» в настройках);
  - Windows (приложение Sirin Desktop);
  - Роутеры OpenWrt (клиент Sirin для всей домашней сети).
• Администратор и техническая поддержка: @giabrat.

Правила общения:
1. Отвечай на русском языке, доброжелательно, живо и по делу.
2. Форматируй текст в красивый Markdown для Telegram (жирный шрифт *...*, списки • ..., моноширинный код для команд).
3. Избегай излишне длинных «простыней» текста — отвечай понятно, ёмко и структурированно.
4. Если пользователь задаёт общий вопрос (не про VPN) — поддержи интересную и вежливую беседу как эрудированный собеседник.
5. Если у пользователя техническая проблема, которую не решить советом (например, ошибка ключа, не работает сервер, вопрос оплаты) — вежливо подскажи, что можно написать администратору @giabrat или нажать кнопку связи с поддержкой.`

type Part struct {
	Text string `json:"text"`
}

type Content struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

type SystemInstruction struct {
	Parts []Part `json:"parts"`
}

type GenerateRequest struct {
	SystemInstruction SystemInstruction `json:"systemInstruction,omitempty"`
	Contents          []Content         `json:"contents"`
	GenerationConfig  map[string]any    `json:"generationConfig,omitempty"`
}

type GenerateResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

type Client struct {
	mu        sync.RWMutex
	apiKey    string
	http      *http.Client
	historyMu sync.Mutex
	history   map[int64][]Content
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  strings.TrimSpace(apiKey),
		http:    &http.Client{Timeout: 30 * time.Second},
		history: make(map[int64][]Content),
	}
}

func (c *Client) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.apiKey != ""
}

func (c *Client) SetAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.apiKey = strings.TrimSpace(key)
}

func (c *Client) GetAPIKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.apiKey
}

func (c *Client) Reset(userID int64) {
	c.historyMu.Lock()
	defer c.historyMu.Unlock()
	delete(c.history, userID)
}

func (c *Client) Ask(userID int64, userMessage string) (string, error) {
	apiKey := c.GetAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("Gemini API key is not configured")
	}

	c.historyMu.Lock()
	userHistory := c.history[userID]

	// Добавляем текущее сообщение пользователя в историю
	currentTurn := Content{
		Role:  "user",
		Parts: []Part{{Text: userMessage}},
	}
	historyToSend := append(userHistory, currentTurn)

	// Ограничиваем глубину контекста
	if len(historyToSend) > maxHistoryTurns {
		historyToSend = historyToSend[len(historyToSend)-maxHistoryTurns:]
	}
	c.historyMu.Unlock()

	// 1. Пробуем сначала основную модель, затем fallback
	models := []string{defaultModel, fallbackModel}
	var lastErr error
	var replyText string

	for _, model := range models {
		replyText, lastErr = c.doGenerate(apiKey, model, historyToSend)
		if lastErr == nil && replyText != "" {
			break
		}
		log.Printf("[Gemini] model %s failed: %v, trying next...", model, lastErr)
	}

	if lastErr != nil {
		return "", lastErr
	}

	// Сохраняем ответ модели в историю диалога пользователя
	c.historyMu.Lock()
	c.history[userID] = append(historyToSend, Content{
		Role:  "model",
		Parts: []Part{{Text: replyText}},
	})
	c.historyMu.Unlock()

	return replyText, nil
}

func (c *Client) doGenerate(apiKey, model string, contents []Content) (string, error) {
	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)

	reqPayload := GenerateRequest{
		SystemInstruction: SystemInstruction{
			Parts: []Part{{Text: systemPrompt}},
		},
		Contents: contents,
		GenerationConfig: map[string]any{
			"temperature":     0.7,
			"maxOutputTokens": 1024,
		},
	}

	jsonBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(jsonBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read error: %w", err)
	}

	var genResp GenerateResponse
	if err := json.Unmarshal(bodyBytes, &genResp); err != nil {
		return "", fmt.Errorf("unmarshal error: %w (body: %s)", err, string(bodyBytes))
	}

	if genResp.Error != nil {
		return "", fmt.Errorf("Gemini API error (%d %s): %s", genResp.Error.Code, genResp.Error.Status, genResp.Error.Message)
	}

	if len(genResp.Candidates) == 0 || len(genResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}

	reply := strings.TrimSpace(genResp.Candidates[0].Content.Parts[0].Text)
	return reply, nil
}
