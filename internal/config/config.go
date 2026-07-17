package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/url"
	"os"
	"strings"
)

// Config uygulamanın çalışma zamanı ayarlarını tutar.
type Config struct {
	Port             string   // HTTP sunucusunun dinleyeceği port (örn: "8080")
	DBPath           string   // SQLite veritabanı dosyasının yolu
	JWTSecret        []byte   // JWT imzalamada kullanılacak gizli anahtar
	BaseURL          string   // Kısaltılmış linklerin ön eki (örn: "http://localhost:8080")
	CookieSecure     bool     // Çerez Secure bayrağı (https dağıtımlarında true)
	AllowedOrigins   []string  // CORS için kabul edilen kökenler ("https://linklik.example.com")
	TLSCertPath      string   // TLS sertifikası yol (boşsa HTTP)
	TLSKeyPath       string   // TLS anahtar yolu (boşsa HTTP)
}

// GlobalConfig global konfigürasyon nesnesidir.
var GlobalConfig *Config

// LoadConfig çevresel değişkenlerden konfigürasyonu yükler veya varsayılan değerleri atar.
func LoadConfig() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "linklik.db"
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:" + port
	}

	// CORS için; virgülle ayrılmış allowed origins
	allowedRaw := os.Getenv("ALLOWED_ORIGINS")
	var allowed []string
	for _, o := range strings.Split(allowedRaw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			allowed = append(allowed, o)
		}
	}
	// Varsayılan: BaseURL kökenini ekle
	if baseURLOrigin, err := url.Parse(baseURL); err == nil && baseURLOrigin.Host != "" {
		origin := baseURLOrigin.Scheme + "://" + baseURLOrigin.Host
		if !containsString(allowed, origin) {
			allowed = append(allowed, origin)
		}
	}

	// JWT secret değerini çevresel değişkenden al veya rastgele üret
	jwtSecretStr := os.Getenv("JWT_SECRET")
	var jwtSecret []byte
	if jwtSecretStr == "" {
		// Üretim ortamında mutlaka ortam değişkeni ile sağlanmalı.
		// Geliştirme için rastgele 32 byte üretilir (her restart'ta oturumlar düşer, bu kabul edilebilir).
		log.Println("[Security] JWT_SECRET ortam değişkeni ayarlanmamış; geçici rastgele anahtar üretiliyor (geliştirme modu).")
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			// crypto/rand başarısız olursa güvenli anahtar üretilemiyor: sistem durmamalı
			log.Fatalf("[Security] geçici JWT anahtarı üretilemedi: %v", err)
		}
		jwtSecret = bytes
	} else {
		// Zayıf anahtarları reddet: < 32 byte (256 bit) HMAC-SHA256 güvenliği için yetersiz
		if len(jwtSecretStr) < 32 {
			log.Fatalf("[Security] JWT_SECRET çok kısa (min 32 byte gerekir). Daha güçlü bir anahtar kullanın.")
		}
		jwtSecret = []byte(jwtSecretStr)
	}

	// Çerez Secure bayrağı: önce ortam değişkeni, yoksa BaseURL https ise true
	cookieSecure := false
	if v := os.Getenv("COOKIE_SECURE"); v != "" {
		cookieSecure = (v == "1" || strings.EqualFold(v, "true"))
	} else if u, err := url.Parse(baseURL); err == nil && strings.EqualFold(u.Scheme, "https") {
		cookieSecure = true
	}

	tlsCert := os.Getenv("TLS_CERT_PATH")
	tlsKey := os.Getenv("TLS_KEY_PATH")

	GlobalConfig = &Config{
		Port:           port,
		DBPath:         dbPath,
		JWTSecret:      jwtSecret,
		BaseURL:        baseURL,
		CookieSecure:   cookieSecure,
		AllowedOrigins: allowed,
		TLSCertPath:    tlsCert,
		TLSKeyPath:     tlsKey,
	}
}

// GetJWTSecretKey JWT doğrulaması için gizli anahtarı döndürür.
func (c *Config) GetJWTSecretKey() []byte {
	return c.JWTSecret
}

// IsOriginAllowed verilen Origin'in CORS için kabul edildiğini döner.
func (c *Config) IsOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	for _, a := range c.AllowedOrigins {
		if a == origin {
			return true
		}
	}
	return false
}

// GenerateRandomKey rastgele bir API Key üretir.
// Güvenlik: crypto/rand başarısız olursa (çok nadir) fallback yerine fatal çık.
func GenerateRandomKey() string {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		// Sabit fallback dönmek tüm kullanıcıların aynı anahtarı almasına yol açar.
		log.Fatalf("[Security] rastgele anahtar üretilemedi: %v", err)
	}
	return hex.EncodeToString(bytes)
}

func containsString(slice []string, v string) bool {
	for _, s := range slice {
		if s == v {
			return true
		}
	}
	return false
}
