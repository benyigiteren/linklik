package middleware

import (
	"context"
	"encoding/json"
	"linklik/internal/config"
	"linklik/internal/model"
	"linklik/internal/service"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserContextKey contextKey = "user"

// jwtKeyFunc yalnızca HS256 (HMAC) imzalı token'lara izin verir. Bu, algoritma
// karışıklığı (alg confusion) saldırılarına karşı savunma sağlar.
func jwtKeyFunc(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, http.ErrAbortHandler
	}
	if token.Header["alg"] != "HS256" {
		return nil, http.ErrAbortHandler
	}
	return config.GlobalConfig.GetJWTSecretKey(), nil
}

// APIKeyAuth HTTP API isteklerinde X-API-KEY başlığını doğrular.
// Güvenlik: Yalnızca başlık kabul edilir; ?api_key= URL parametresi
// loglar/tarayıcı geçmişi/referer üzerinden sızdırabileceği için kaldırılmıştır.
func APIKeyAuth(userService *service.UserService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-KEY")

			if apiKey == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(model.APIResponse{
					Success: false,
					Error:   "Eksik veya geçersiz API Anahtarı. Lütfen istek başlığına 'X-API-KEY' ekleyin.",
				})
				return
			}

			user, err := userService.GetUserByAPIKey(apiKey)
			if err != nil || user == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(model.APIResponse{
					Success: false,
					Error:   "Geçersiz API Anahtarı.",
				})
				return
			}

			// Kullanıcıyı bağlam (context) içerisine ekle
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SessionAuth JWT token'ına göre (Cookie veya Authorization Header) kimlik doğrulaması yapar.
func SessionAuth(userService *service.UserService, isAPI bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tokenStr string

			// 1. Authorization header'ı kontrol et (Bearer <token>)
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			}

			// 2. Cookie'den token kontrol et
			if tokenStr == "" {
				cookie, err := r.Cookie("token")
				if err == nil {
					tokenStr = cookie.Value
				}
			}

			if tokenStr == "" {
				handleUnauthorized(w, r, isAPI)
				return
			}

// Token doğrula
		token, err := jwt.Parse(tokenStr, jwtKeyFunc)

		if err != nil || !token.Valid {
			handleUnauthorized(w, r, isAPI)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			handleUnauthorized(w, r, isAPI)
			return
		}

		userIDFloat, ok := claims["user_id"].(float64)
		if !ok {
			handleUnauthorized(w, r, isAPI)
			return
		}

		// Kullanıcıyı veritabanından çek
		user, err := userService.GetUserByID(int64(userIDFloat))
		if err != nil || user == nil {
			handleUnauthorized(w, r, isAPI)
			return
		}

		// Kullanıcıyı context'e kaydet
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole kullanıcının belirli bir role sahip olmasını zorunlu kılar.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value(UserContextKey).(*model.User)
			if !ok || user == nil || user.Role != role {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(model.APIResponse{
					Success: false,
					Error:   "Bu işlem için yetkiniz bulunmamaktadır. (Yalnızca Superadmin)",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CORS yalnızca yapılandırılmış izin verilen kökenlere (allowed origins) izin verir.
// Güvenlik: Yaban * yerine, yalnızca tanınan Origin'ler geri yansıtılır; Vary: Origin ayarlanır.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := config.GlobalConfig.IsOriginAllowed(origin)
		w.Header().Add("Vary", "Origin")
		if allowed && origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-KEY")
			// Allow-Credentials: false (cookie oturumları CORS ile paylaşılmaz; dashboard same-origin)
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders tarayıcı düzeyinde savunma için temel güvenlik başlıklarını ekler.
// CSP, XSS sızıntısını büyük ölçüde kısıtlar; X-Frame-Options clickjacking engeller.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		// Content-Security-Policy: same-origin + gerekli CDN'ler; inline script/devamı için 'unsafe-inline'
		// (chart.js CDN'den + mevcut inline bloklar). Frame-ancestors ile clickjacking de engellendi.
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; "+
				"style-src 'self' 'unsafe-inline' https://unpkg.com; "+
				"img-src 'self' data:; "+
				"font-src 'self' data: https://unpkg.com https://cdn.jsdelivr.net; "+
				"connect-src 'self'; "+
				"frame-ancestors 'none'; "+
				"base-uri 'none'; "+
				"form-action 'self'")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		h.Set("X-XSS-Protection", "0") // modern tarayıcılar CSP'ye öncelik vermeli
		// HSTS yalnızca HTTPS üzerinden gelen yanıtlarda
		if r.TLS != nil {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// GetUserFromContext context içerisindeki kullanıcı nesnesini döner.
func GetUserFromContext(ctx context.Context) *model.User {
	u, ok := ctx.Value(UserContextKey).(*model.User)
	if !ok {
		return nil
	}
	return u
}

// handleUnauthorized yetkisiz isteklerde uygun yanıtı (JSON hata mesajı veya login sayfasına yönlendirme) döner.
func handleUnauthorized(w http.ResponseWriter, r *http.Request, isAPI bool) {
	if isAPI {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(model.APIResponse{
			Success: false,
			Error:   "Oturum süresi dolmuş veya yetkisiz istek. Lütfen giriş yapın.",
		})
		return
	}

	// Web istekleri için login sayfasına yönlendir
	http.Redirect(w, r, "/login", http.StatusFound)
}

// SessionAuthOptional token'ı kontrol edip context'e ekler, ancak geçersiz veya eksikse hata vermez, devam eder.
func SessionAuthOptional(userService *service.UserService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tokenStr string

			// 1. Authorization header
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			}

			// 2. Cookie
			if tokenStr == "" {
				cookie, err := r.Cookie("token")
				if err == nil {
					tokenStr = cookie.Value
				}
			}

			if tokenStr != "" {
				token, err := jwt.Parse(tokenStr, jwtKeyFunc)

				if err == nil && token.Valid {
					if claims, ok := token.Claims.(jwt.MapClaims); ok {
						if userIDFloat, ok := claims["user_id"].(float64); ok {
							user, err := userService.GetUserByID(int64(userIDFloat))
							if err == nil && user != nil {
								ctx := context.WithValue(r.Context(), UserContextKey, user)
								next.ServeHTTP(w, r.WithContext(ctx))
								return
							}
						}
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
// AuthEither API anahtarı (yalnızca header) VEYA oturum çerezi/JWT kullanarak kimlik doğrulama yapar.
// Güvenlik: URL ?api_key= sızıntısı (log/referer/geçmiş) yüzünden kaldırılmıştır.
func AuthEither(userService *service.UserService, isAPI bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Önce API Anahtarını kontrol et (yalnızca header)
			apiKey := r.Header.Get("X-API-KEY")

			if apiKey != "" {
				user, err := userService.GetUserByAPIKey(apiKey)
				if err == nil && user != nil {
					ctx := context.WithValue(r.Context(), UserContextKey, user)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				// Eğer geçersiz bir API anahtarı gönderildiyse devam etmeyip hata dönelim
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(model.APIResponse{
					Success: false,
					Error:   "Geçersiz API Anahtarı.",
				})
				return
			}

			// API anahtarı yoksa JWT oturumunu kontrol et
			var tokenStr string
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			}
			if tokenStr == "" {
				cookie, err := r.Cookie("token")
				if err == nil {
					tokenStr = cookie.Value
				}
			}

			if tokenStr == "" {
				handleUnauthorized(w, r, isAPI)
				return
			}

			token, err := jwt.Parse(tokenStr, jwtKeyFunc)

			if err != nil || !token.Valid {
				handleUnauthorized(w, r, isAPI)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				handleUnauthorized(w, r, isAPI)
				return
			}

			userIDFloat, ok := claims["user_id"].(float64)
			if !ok {
				handleUnauthorized(w, r, isAPI)
				return
			}

			user, err := userService.GetUserByID(int64(userIDFloat))
			if err != nil || user == nil {
				handleUnauthorized(w, r, isAPI)
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
