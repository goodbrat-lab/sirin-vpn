package engine

// tun_stream_monitor.go — адаптивный монитор пропускной способности потоковых
// соединений (YouTube / GoogleVideo CDN) внутри туннеля в режиме Zapret 2.
//
// Принцип работы
// ──────────────
// На каждое соединение, которое обслуживает видеопотоки (YouTube / googlevideo.com / CDN),
// создаётся отдельный *streamMonitor. Он подсчитывает байты, принятые от удалённого
// сервера в пользу клиента (входящий поток: это и есть видео), и отслеживает активность
// запросов со стороны клиента (HTTP GET/Range).
//
// Если клиент активно запрашивает данные, но на протяжении streamLowSeconds секунд подряд
// скорость непрерывного потока не превышает streamMinKbps — монитор вызывает escalateFn,
// которая инициирует бесшовное переключение на самый быстрый VPN-сервер.
//
// Защита от ложных срабатываний:
// 1. Не эскалирует при простое соединения (idle/пауза воспроизведения).
// 2. Не эскалирует, если в окне была хотя бы одна быстрая секунда (burst).
// 3. Защита от шторма переключений: sessionEscalateCooldown (30 секунд).

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	// streamMinKbps — минимальная пропускная способность для комфортного
	// воспроизведения видео высокого разрешения (1080p60 / 4K).
	// YouTube Adaptive Bitrate: 720p ≈ 2–3 Мбит/с, 1080p ≈ 5–8 Мбит/с, 4K ≈ 15–25 Мбит/с.
	// Порог 5 Мбит/с позволяет 720p проходить на лёгком маршруте, но при попытке смотреть
	// 1080p/4K немедленно эскалирует на самый быстрый VPN.
	streamMinKbps = 5_000 // кбит/с = 5 Мбит/с

	// streamLowSeconds — сколько секунд подряд непрерывный поток должен быть ниже порога.
	streamLowSeconds = 3

	// sessionEscalateCooldown — пауза между переключениями серверов, чтобы не
	// вызвать шторм переподключений при параллельных чанках видео.
	sessionEscalateCooldown = 30 * time.Second
)

// streamMonitor отслеживает скорость одного соединения.
type streamMonitor struct {
	mu             sync.Mutex
	minActiveBytes int64     // минимальный порог активности в секунду (чтобы игнорировать keepalive)
	currentBucket  int64     // байты входящего потока за текущую секунду
	lowCount       int       // число секунд подряд со скоростью >= minActiveBytes и < streamMinKbps
	lastClientSend time.Time // время последнего запроса клиента
	totalRecv      int64
	done           chan struct{}
	once           sync.Once
}

// newStreamMonitor создаёт монитор для потоковых доменов (с порогом активности 1 байт/с)
// и запускает фоновый анализ скорости.
func newStreamMonitor(escalateFn func()) *streamMonitor {
	return newStreamMonitorWithMin(1, escalateFn)
}

// newStreamMonitorWithMin создаёт монитор с заданным минимальным порогом активности (байт/с).
func newStreamMonitorWithMin(minActiveBytes int64, escalateFn func()) *streamMonitor {
	if minActiveBytes < 1 {
		minActiveBytes = 1
	}
	m := &streamMonitor{
		minActiveBytes: minActiveBytes,
		done:           make(chan struct{}),
		lastClientSend: time.Now(),
	}
	go m.run(escalateFn)
	return m
}

// AddServerBytes регистрирует n байт входящего потока (server → client).
func (m *streamMonitor) AddServerBytes(n int) {
	if n <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentBucket += int64(n)
	m.totalRecv += int64(n)
}

// AddClientBytes регистрирует n байт исходящих запросов (client → server).
func (m *streamMonitor) AddClientBytes(n int) {
	if n <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastClientSend = time.Now()
}

// Close сигнализирует монитору, что соединение закрыто.
func (m *streamMonitor) Close() {
	m.once.Do(func() { close(m.done) })
}

func (m *streamMonitor) run(escalateFn func()) {
	t := time.NewTicker(time.Second)
	defer t.Stop()

	const thresholdBytes = streamMinKbps * 1000 / 8

	for {
		select {
		case <-m.done:
			return
		case <-t.C:
			m.mu.Lock()
			bytesInSec := m.currentBucket
			m.currentBucket = 0
			clientActive := !m.lastClientSend.IsZero() && time.Since(m.lastClientSend) <= 5*time.Second

			if clientActive && bytesInSec >= m.minActiveBytes && bytesInSec < thresholdBytes {
				m.lowCount++
			} else if bytesInSec >= thresholdBytes {
				m.lowCount = 0
			} else {
				// 0 байт, фоновый keepalive или клиент не активен — сбрасываем счётчик
				m.lowCount = 0
			}

			shouldEscalate := m.lowCount >= streamLowSeconds
			m.mu.Unlock()

			if shouldEscalate {
				escalateFn()
				return
			}
		}
	}
}

var (
	escalateMu      sync.Mutex
	lastEscalateAt  time.Time
	escalateEnabled atomic.Bool
)

func init() {
	escalateEnabled.Store(true)
}

// SetEscalateEnabled включает/выключает механизм эскалации.
func SetEscalateEnabled(on bool) {
	escalateEnabled.Store(on)
}

// maybeEscalate вызывает fn не чаще sessionEscalateCooldown.
func maybeEscalate(fn func()) {
	if !escalateEnabled.Load() {
		return
	}
	escalateMu.Lock()
	defer escalateMu.Unlock()
	if time.Since(lastEscalateAt) < sessionEscalateCooldown {
		return
	}
	lastEscalateAt = time.Now()
	go fn()
}
