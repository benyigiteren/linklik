package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	chi_middleware "github.com/go-chi/chi/v5/middleware"
)

// rateLimiterEntry IP başına token bucket (basit zaman penceresi limiti) saklar.
type rateLimiterEntry struct {
	mu        sync.Mutex
	count     int
	windowEnd time.Time
}

// RateLimit her IP için belirli bir zaman penceresinde maksimum istek sayısını sınırlandırır.
// Güvenlik: Brute-force (login/setup), spam (link oluşturma) ve DoS engelinin temel katmanı.
// limit: pencere başına izin verilen istek sayısı; window: zaman penceresi.
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	var (
		mu      sync.Mutex
		buckets = make(map[string]*rateLimiterEntry)
	)

	// Arka planda eski pencere girişlerini temizle
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			mu.Lock()
			for ip, e := range buckets {
				if e.windowEnd.Before(now) {
					delete(buckets, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if ip == "" {
				ip = "unknown"
			}

			mu.Lock()
			e, ok := buckets[ip]
			if !ok {
				e = &rateLimiterEntry{}
				buckets[ip] = e
			}
			e.mu.Lock()
			now := time.Now()
			if e.windowEnd.Before(now) {
				e.count = 0
				e.windowEnd = now.Add(window)
			}
			e.count++
			allowed := e.count <= limit
			e.mu.Unlock()
			mu.Unlock()

			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP güvenilir chi ClientIP middleware'inden IP'yi okur.
// chi v5 GetClientIP, r.RemoteAddr'ı mutasyona uğratmaz (eski RealIP'in aksine),
// dolayısıyla X-Forwarded-For sahteciliğine karşı güvenlidir (varsayılan konfig).
func clientIP(r *http.Request) string {
	if ip := chi_middleware.GetClientIP(r.Context()); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// MaxBodySize istek gövdesini belirtilen bayt sınırına kısıtlar. Bu, JSON tabanlı
// DoS saldırılarını (büyük gövde üzerinden bellek tüketimi) engeller.
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Yalnızca gövde içeren metotları sınırla
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}