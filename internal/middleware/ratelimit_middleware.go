package middleware

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
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

// maxBuckets bellek tüketim saldırısına karşı IP bucket sınırıdır.
// Güvenlik: Saldırgan milyonlarca farklı IP'den istek gönderebilir;
// bu sınır aşıldığında eski bucket'lar silinir.
const maxBuckets = 100000

// RateLimit her IP için belirli bir zaman penceresinde maksimum istek sayısını sınırlandırır.
// Güvenlik: Brute-force (login/setup), spam (link oluşturma) ve DoS engelinin temel katmanı.
// limit: pencere başına izin verilen istek sayısı; window: zaman penceresi.
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	var (
		mu      sync.Mutex
		buckets = make(map[string]*rateLimiterEntry)
	)

	// Arka planda eski pencere girişlerini temizle (1 dakikada bir)
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
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
				// Bellek koruma: bucket sayısı sınırı aşıldığında en eski bucket'ları temizle
				if len(buckets) >= maxBuckets {
					now := time.Now()
					for k, v := range buckets {
						if v.windowEnd.Before(now) {
							delete(buckets, k)
						}
					}
					// Hâlâ doluysa en eski yarısını sil
					if len(buckets) >= maxBuckets {
						count := 0
						for k := range buckets {
							delete(buckets, k)
							count++
							if count >= maxBuckets/2 {
								break
							}
						}
					}
				}
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
			retryAfter := int(time.Until(e.windowEnd).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			e.mu.Unlock()
			mu.Unlock()

			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success":     false,
					"error":       "Çok fazla istek gönderildi. Lütfen biraz bekleyip tekrar deneyin.",
					"retry_after": retryAfter,
				})
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

// StaticCacheHeaders statik dosyalar (CSS, JS, görseller) için tarayıcı önbellek başlıklarını ayarlar.
// Performans: Tekrarlanan isteklerde ağ trafiğini azaltır.
func StaticCacheHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.ToLower(r.URL.Path)
		if strings.HasSuffix(path, ".css") || strings.HasSuffix(path, ".js") ||
			strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".jpg") ||
			strings.HasSuffix(path, ".jpeg") || strings.HasSuffix(path, ".svg") ||
			strings.HasSuffix(path, ".ico") || strings.HasSuffix(path, ".woff2") ||
			strings.HasSuffix(path, ".woff") {
			w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
		}
		next.ServeHTTP(w, r)
	})
}

// RequireXHR cookie tabanlı oturumlarda CSRF koruması sağlar.
// State-changing (POST/PUT/DELETE/PATCH) isteklerde X-Requested-With başlığını zorunlu kılar.
// Güvenlik: HTML formları ve basit cross-origin istekler bu başlığı gönderemez;
// yalnızca JavaScript XHR/fetch istekleri gönderebilir. API key ile gelen istekler muaftır.
func RequireXHR(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sadece state-changing istekleri kontrol et
		if r.Method == http.MethodPost || r.Method == http.MethodPut ||
			r.Method == http.MethodDelete || r.Method == http.MethodPatch {

			// API Key ile gelen istekler muaf (programatik istemciler)
			if r.Header.Get("X-API-KEY") != "" {
				next.ServeHTTP(w, r)
				return
			}

			// Cookie tabanlı oturumlarda X-Requested-With zorunlu
			if _, err := r.Cookie("token"); err == nil {
				if r.Header.Get("X-Requested-With") == "" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"success": false,
						"error":   "Güvenlik hatası: İstek doğrulanamadı (CSRF koruması).",
					})
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}