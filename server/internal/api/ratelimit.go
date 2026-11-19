package api

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ipBucket struct {
	tokens   float64
	lastSeen time.Time
}

// ipRateLimiter — легковесный per-IP token-bucket ограничитель без внешних зависимостей.
type ipRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*ipBucket
	rate    float64 // токенов в секунду
	burst   float64 // максимальный всплеск
	lastGC  time.Time
}

func newIPRateLimiter(rps float64, burst int) *ipRateLimiter {
	return &ipRateLimiter{
		buckets: make(map[string]*ipBucket),
		rate:    rps,
		burst:   float64(burst),
		lastGC:  time.Now(),
	}
}

func (l *ipRateLimiter) allow(ip string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	// Периодическая очистка устаревших записей (раз в 5 минут)
	if now.Sub(l.lastGC) > 5*time.Minute {
		for k, b := range l.buckets {
			if now.Sub(b.lastSeen) > 10*time.Minute {
				delete(l.buckets, k)
			}
		}
		l.lastGC = now
	}

	b, ok := l.buckets[ip]
	if !ok {
		l.buckets[ip] = &ipBucket{
			tokens:   l.burst - 1,
			lastSeen: now,
		}
		return true
	}

	elapsed := now.Sub(b.lastSeen).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func clientIP(r *http.Request) string {
	// Если стоим за локальным nginx (127.0.0.1), уважаем X-Real-IP / X-Forwarded-For
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if host == "127.0.0.1" || host == "::1" {
		if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
			return xri
		}
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			parts := strings.Split(xff, ",")
			if first := strings.TrimSpace(parts[0]); first != "" {
				return first
			}
		}
	}
	return host
}

func rateLimitMiddleware(limiter *ipRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow(clientIP(r)) {
			w.Header().Set("Retry-After", "1")
			writeJSON(w, http.StatusTooManyRequests, ErrorResponse{
				Contract: "v1",
				Status:   "error",
				Error:    "too many requests",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}
