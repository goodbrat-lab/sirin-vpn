package menu

import (
	"sync"
	"time"
)

// State описывает текущее интерактивное состояние пользователя.
type State struct {
	UserID    int64             // Telegram User ID
	Action    string            // "new_name", "new_days_custom", "clients_filter", "search_query", "rename_new", "extend_days", "shorten_days"
	ClientID  string            // UUID клиента для контекста
	Data      map[string]string // Произвольные данные шагов
	MessageID int               // ID сообщения для возможного редактирования
	UpdatedAt time.Time         // Время последнего обновления состояния
}

// FSM управляет конечным автоматом диалогов для пользователей.
type FSM struct {
	mu     sync.RWMutex
	states map[int64]*State
	ttl    time.Duration
	stopCh chan struct{}
}

// NewFSM создаёт новый менеджер состояний.
func NewFSM(ttl time.Duration) *FSM {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	fsm := &FSM{
		states: make(map[int64]*State),
		ttl:    ttl,
		stopCh: make(chan struct{}),
	}
	go fsm.cleanupLoop()
	return fsm
}

// Get возвращает актуальное состояние пользователя, если оно не истекло по TTL.
func (f *FSM) Get(userID int64) (*State, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	s, ok := f.states[userID]
	if !ok {
		return nil, false
	}

	if time.Since(s.UpdatedAt) > f.ttl {
		delete(f.states, userID)
		return nil, false
	}

	// Возвращаем копию для безопасности
	copyData := make(map[string]string, len(s.Data))
	for k, v := range s.Data {
		copyData[k] = v
	}

	clone := &State{
		UserID:    s.UserID,
		Action:    s.Action,
		ClientID:  s.ClientID,
		Data:      copyData,
		MessageID: s.MessageID,
		UpdatedAt: s.UpdatedAt,
	}
	return clone, true
}

// Set устанавливает новое состояние пользователя.
func (f *FSM) Set(userID int64, action, clientID string, msgID int, data map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if data == nil {
		data = make(map[string]string)
	}

	f.states[userID] = &State{
		UserID:    userID,
		Action:    action,
		ClientID:  clientID,
		Data:      data,
		MessageID: msgID,
		UpdatedAt: time.Now(),
	}
}

// Update обновляет поля существующего состояния.
func (f *FSM) Update(userID int64, fn func(s *State)) {
	f.mu.Lock()
	defer f.mu.Unlock()

	s, ok := f.states[userID]
	if !ok {
		return
	}
	fn(s)
	s.UpdatedAt = time.Now()
}

// Clear сбрасывает состояние пользователя.
func (f *FSM) Clear(userID int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.states, userID)
}

// Close останавливает фоновый воркер очистки.
func (f *FSM) Close() {
	select {
	case <-f.stopCh:
	default:
		close(f.stopCh)
	}
}

func (f *FSM) cleanupLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-f.stopCh:
			return
		case <-ticker.C:
			f.mu.Lock()
			now := time.Now()
			for uid, s := range f.states {
				if now.Sub(s.UpdatedAt) > f.ttl {
					delete(f.states, uid)
				}
			}
			f.mu.Unlock()
		}
	}
}
