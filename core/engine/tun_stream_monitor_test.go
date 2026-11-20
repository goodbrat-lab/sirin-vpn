package engine

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestStreamMonitorNoEscalateOnIdle(t *testing.T) {
	var escalated atomic.Bool
	m := newStreamMonitor(func() {
		escalated.Store(true)
	})
	defer m.Close()

	// Соединение открылось, но данных нет (idle)
	time.Sleep(1500 * time.Millisecond)

	if escalated.Load() {
		t.Fatalf("монитор не должен эскалировать при отсутствии данных")
	}
}

func TestStreamMonitorNoEscalateOnFastStream(t *testing.T) {
	var escalated atomic.Bool
	m := newStreamMonitor(func() {
		escalated.Store(true)
	})
	defer m.Close()

	// Шлём 1 МБайт в секунду (8 Мбит/с > 5 Мбит/с)
	for i := 0; i < 3; i++ {
		m.AddClientBytes(100)
		m.AddServerBytes(1_000_000)
		time.Sleep(500 * time.Millisecond)
	}

	if escalated.Load() {
		t.Fatalf("монитор не должен эскалировать при быстрой скорости (8 Мбит/с)")
	}
}

func TestStreamMonitorEscalatesOnThrottledStream(t *testing.T) {
	var escalated atomic.Bool
	m := newStreamMonitor(func() {
		escalated.Store(true)
	})
	defer m.Close()

	// Имитируем троттлинг ТСПУ: клиент шлёт запрос, сервер отдаёт всего 100 КБ/с (800 кбит/с < 5 Мбит/с)
	// непрерывно в течение 3.5 секунд.
	start := time.Now()
	for time.Since(start) < 3500*time.Millisecond {
		m.AddClientBytes(50)
		m.AddServerBytes(25_000)
		time.Sleep(250 * time.Millisecond)
		if escalated.Load() {
			break
		}
	}

	if !escalated.Load() {
		t.Fatalf("монитор обязан был эскалировать на задушенном потоке!")
	}
}
