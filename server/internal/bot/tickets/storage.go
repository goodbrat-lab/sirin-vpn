package tickets

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Storage потокобезопасное хранилище тикетов технической поддержки.
type Storage struct {
	mu   sync.RWMutex
	path string
	data TicketsFile
}

// LoadTickets открывает или создаёт хранилище тикетов с правами 0600.
func LoadTickets(path string) (*Storage, error) {
	s := &Storage{
		path: path,
		data: TicketsFile{Tickets: []Ticket{}},
	}
	if err := s.loadOrCreate(); err != nil {
		return nil, err
	}
	// Автоматическая очистка тикетов старше 90 дней
	s.Cleanup(90 * 24 * time.Hour)
	return s, nil
}

func (s *Storage) loadOrCreate() error {
	dir := filepath.Dir(s.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s.saveLocked()
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", s.path, err)
	}

	if len(data) == 0 {
		return s.saveLocked()
	}

	var file TicketsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("parse %s: %w", s.path, err)
	}

	if file.Tickets == nil {
		file.Tickets = []Ticket{}
	}
	s.data = file
	return nil
}

func (s *Storage) saveLocked() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	// Создание резервной копии перед перезаписью, если файл уже существует
	if _, err := os.Stat(s.path); err == nil {
		bak := fmt.Sprintf("%s.bak.%d", s.path, time.Now().Unix())
		_ = os.WriteFile(bak, data, 0600)
	}

	tmp := s.path + ".tmp." + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write tmp %s: %w", tmp, err)
	}

	// Строгие права 0600 на файл тикетов
	_ = os.Chmod(tmp, 0600)

	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	_ = os.Chmod(s.path, 0600)

	return nil
}

// Reload перечитывает тикеты с диска.
func (s *Storage) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadOrCreate()
}

// CreateTicket создаёт новый тикет с валидацией rate limit.
func (s *Storage) CreateTicket(
	telegramID int64,
	username string,
	dev DeviceContext,
	category, subject, userMsg string,
	diag []DiagnosticItem,
) (*Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.loadOrCreate()

	now := time.Now()

	// 1. Проверка лимита: не более 3 открытых тикетов на telegram_id
	openCount := 0
	for _, t := range s.data.Tickets {
		if t.TelegramID == telegramID && t.Status != StatusClosed {
			openCount++
		}
	}
	if openCount >= 3 {
		return nil, fmt.Errorf("у вас уже открыто %d тикета (максимум 3). Дождитесь ответа или закройте неактуальные", openCount)
	}

	// 2. Проверка лимита: не более 1 нового тикета в час
	oneHourAgo := now.Add(-1 * time.Hour)
	for _, t := range s.data.Tickets {
		if t.TelegramID == telegramID && t.CreatedAt.After(oneHourAgo) {
			diff := time.Until(t.CreatedAt.Add(1 * time.Hour)).Round(time.Minute)
			return nil, fmt.Errorf("создавать тикеты можно не чаще одного раза в час (попробуйте через %v)", diff)
		}
	}

	// 3. Вычисление монотонного ID: max(id) + 1
	nextID := 1
	for _, t := range s.data.Tickets {
		if t.ID >= nextID {
			nextID = t.ID + 1
		}
	}

	var msgs []TicketMessage
	if userMsg != "" {
		msgs = append(msgs, TicketMessage{
			From: string(SenderUser),
			Text: userMsg,
			TS:   now,
		})
	}

	ticket := Ticket{
		ID:               nextID,
		TelegramID:       telegramID,
		Username:         username,
		DeviceUUID:       dev.UUID,
		DeviceLabel:      dev.Label,
		DevicePlatform:   dev.Platform,
		DeviceAppVersion: dev.AppVersion,
		Category:         category,
		Subject:          subject,
		Messages:         msgs,
		Status:           StatusOpen,
		CreatedAt:        now,
		UpdatedAt:        now,
		Diagnostics:      diag,
	}

	s.data.Tickets = append(s.data.Tickets, ticket)
	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	// OpSec: логируем только метаданные без текста обращения
	log.Printf("[TICKETS] created ticket_id=%d user_id=%d category=%s status=%s", ticket.ID, ticket.TelegramID, ticket.Category, ticket.Status)
	return &ticket, nil
}

// AddMessage добавляет сообщение в тикет с проверкой rate limit (не более 5 сообщений за 10 мин от пользователя).
func (s *Storage) AddMessage(ticketID int, from MessageSender, text string) (*Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.loadOrCreate()

	idx := -1
	for i, t := range s.data.Tickets {
		if t.ID == ticketID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, fmt.Errorf("тикет #%d не найден", ticketID)
	}

	now := time.Now()
	t := &s.data.Tickets[idx]

	if from == SenderUser {
		// Проверка: тикет не должен быть закрыт
		if t.Status == StatusClosed {
			return nil, fmt.Errorf("тикет #%d закрыт. Для нового вопроса создайте новое обращение", ticketID)
		}

		// Проверка rate limit: не более 5 сообщений за 10 мин
		tenMinsAgo := now.Add(-10 * time.Minute)
		userMsgCount := 0
		for _, m := range t.Messages {
			if m.From == string(SenderUser) && m.TS.After(tenMinsAgo) {
				userMsgCount++
			}
		}
		if userMsgCount >= 5 {
			return nil, fmt.Errorf("превышен лимит сообщений в тикет (максимум 5 сообщений за 10 минут). Пожалуйста, подождите")
		}
	} else if from == SenderAdmin {
		// Если админ отвечает на открытый тикет, статус переходит в in_progress
		if t.Status == StatusOpen {
			t.Status = StatusInProgress
		}
	}

	t.Messages = append(t.Messages, TicketMessage{
		From: string(from),
		Text: text,
		TS:   now,
	})
	t.UpdatedAt = now

	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	// OpSec: логируем только метаданные
	log.Printf("[TICKETS] add_message ticket_id=%d from=%s count=%d", ticketID, from, len(t.Messages))
	clone := *t
	return &clone, nil
}

// SetStatus изменяет статус тикета.
func (s *Storage) SetStatus(ticketID int, status TicketStatus) (*Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.loadOrCreate()

	for i, t := range s.data.Tickets {
		if t.ID == ticketID {
			s.data.Tickets[i].Status = status
			s.data.Tickets[i].UpdatedAt = time.Now()
			if err := s.saveLocked(); err != nil {
				return nil, err
			}
			log.Printf("[TICKETS] status_change ticket_id=%d new_status=%s", ticketID, status)
			clone := s.data.Tickets[i]
			return &clone, nil
		}
	}
	return nil, fmt.Errorf("тикет #%d не найден", ticketID)
}

// GetByID возвращает тикет по ID.
func (s *Storage) GetByID(id int) (*Ticket, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, t := range s.data.Tickets {
		if t.ID == id {
			clone := t
			return &clone, true
		}
	}
	return nil, false
}

// GetByUser возвращает список всех тикетов пользователя, отсортированных по дате обновления (новые первыми).
func (s *Storage) GetByUser(telegramID int64) []Ticket {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var res []Ticket
	for _, t := range s.data.Tickets {
		if t.TelegramID == telegramID {
			res = append(res, t)
		}
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].UpdatedAt.After(res[j].UpdatedAt)
	})
	return res
}

// GetOpen возвращает все незакрытые тикеты (для администратора).
func (s *Storage) GetOpen() []Ticket {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var res []Ticket
	for _, t := range s.data.Tickets {
		if t.Status != StatusClosed {
			res = append(res, t)
		}
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].UpdatedAt.After(res[j].UpdatedAt)
	})
	return res
}

// GetAll возвращает все тикеты системы (для администратора).
func (s *Storage) GetAll() []Ticket {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]Ticket, len(s.data.Tickets))
	copy(res, s.data.Tickets)

	sort.Slice(res, func(i, j int) bool {
		return res[i].UpdatedAt.After(res[j].UpdatedAt)
	})
	return res
}

// CountOpenByUser возвращает количество открытых тикетов у пользователя.
func (s *Storage) CountOpenByUser(telegramID int64) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, t := range s.data.Tickets {
		if t.TelegramID == telegramID && t.Status != StatusClosed {
			count++
		}
	}
	return count
}

// Cleanup удаляет закрытые тикеты, обновлённые более retention назад (по умолчанию 90 дней).
func (s *Storage) Cleanup(retention time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-retention)

	var retained []Ticket
	removed := 0

	for _, t := range s.data.Tickets {
		if t.Status == StatusClosed && t.UpdatedAt.Before(cutoff) {
			removed++
			continue
		}
		retained = append(retained, t)
	}

	if removed > 0 {
		s.data.Tickets = retained
		_ = s.saveLocked()
		log.Printf("[TICKETS] cleanup removed %d old closed tickets (> %v)", removed, retention)
	}
	return removed
}

// DeleteTicket удаляет тикет (только для администратора).
func (s *Storage) DeleteTicket(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	var remaining []Ticket
	for _, t := range s.data.Tickets {
		if t.ID == id {
			found = true
			continue
		}
		remaining = append(remaining, t)
	}
	if !found {
		return fmt.Errorf("тикет #%d не найден", id)
	}

	s.data.Tickets = remaining
	log.Printf("[TICKETS] delete ticket_id=%d", id)
	return s.saveLocked()
}
