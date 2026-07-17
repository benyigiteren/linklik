package service

import (
	"encoding/json"
	"fmt"
	"io"
	"linklik/internal/model"
	"linklik/internal/repository"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// AnalyticsService analitik verilerini işleyen servis katmanıdır.
type AnalyticsService struct {
	repo *repository.AnalyticsRepository
}

// NewAnalyticsService yeni bir AnalyticsService örneği oluşturur.
func NewAnalyticsService(repo *repository.AnalyticsRepository) *AnalyticsService {
	return &AnalyticsService{repo: repo}
}

// LogRedirectAsync asenkron olarak yönlendirme verilerini veritabanına kaydeder (Goroutine içinde çağrılır).
func (s *AnalyticsService) LogRedirectAsync(linkID int64, ipAddress, userAgent, referrer, headerCountry string) {
	// Goroutine içerisinde ana işlem akışını engellememek için hata kurtarma ekliyoruz
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Analytics Error] Panik kurtarıldı: %v", r)
			}
		}()

		// IP Adresini temizle (port kısmını temizle)
		cleanIP := cleanIPAddress(ipAddress)

		// Ülke Tespiti (Öncelikle header kontrolü, yoksa GeoIP lookup)
		country := "Bilinmiyor"
		if headerCountry != "" {
			country = sanitizeCountry(headerCountry)
		} else {
			country = getCountryByIP(cleanIP)
		}

		// User-Agent ayrıştırma (kontrol karakterleri ayıklanmış haliyle)
		userAgent = truncateString(stripControlChars(userAgent), 512)
		browser, os := parseUserAgent(userAgent)

		// Referrer temizleme (kısa ve okunabilir URL yap)
		cleanReferrer := cleanReferrerURL(referrer)

		analytics := &model.Analytics{
			LinkID:    linkID,
			IPAddress: cleanIP,
			Referrer:  cleanReferrer,
			UserAgent: userAgent,
			Browser:   browser,
			OS:        os,
			Country:   country,
		}

		if err := s.repo.Create(analytics); err != nil {
			log.Printf("[Analytics Error] Analitik kaydedilemedi (Link ID: %d): %v", linkID, err)
		}
	}()
}

// GetStats belirli bir linke ait analitik özetini döner.
func (s *AnalyticsService) GetStats(linkID int64) (*model.LinkStats, error) {
	return s.repo.GetStatsByLinkID(linkID)
}

// parseUserAgent basit ve hızlı bir User-Agent ayrıştırıcısıdır.
func parseUserAgent(ua string) (browser, os string) {
	if ua == "" {
		return "Bilinmiyor", "Bilinmiyor"
	}

	// İşletim sistemi tespiti
	if strings.Contains(ua, "Windows") {
		os = "Windows"
	} else if strings.Contains(ua, "Android") {
		os = "Android"
	} else if strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad") {
		os = "iOS"
	} else if strings.Contains(ua, "Macintosh") || strings.Contains(ua, "Intel Mac") {
		os = "macOS"
	} else if strings.Contains(ua, "Linux") {
		os = "Linux"
	} else {
		os = "Diğer"
	}

	// Tarayıcı tespiti
	if strings.Contains(ua, "Firefox/") {
		browser = "Firefox"
	} else if strings.Contains(ua, "OPR/") || strings.Contains(ua, "Opera/") {
		browser = "Opera"
	} else if strings.Contains(ua, "Edg/") {
		browser = "Edge"
	} else if strings.Contains(ua, "Chrome/") {
		browser = "Chrome"
	} else if strings.Contains(ua, "Safari/") {
		browser = "Safari"
	} else {
		browser = "Diğer"
	}

	return browser, os
}

// cleanIPAddress IP adresindeki port numarasını kaldırır.
func cleanIPAddress(ip string) string {
	ip = strings.TrimSpace(ip)
	if strings.Contains(ip, ":") {
		host, _, err := net.SplitHostPort(ip)
		if err == nil {
			return host
		}
		// IPv6 adresleri için port yoksa SplitHostPort hata verebilir, direkt ip'yi dönelim
	}
	return ip
}

// cleanReferrerURL yönlendiren URL'i sadeleştirerek döndürür.
func cleanReferrerURL(ref string) string {
	if ref == "" {
		return "Direkt İrtibat / Yer imi"
	}
	ref = strings.TrimSpace(stripControlChars(ref))
	return truncateString(ref, 100)
}

// countryHeaderRegex istemci tarafından gönderilen ülke başlığı için izin verilen
// karakter kümesidir (harf, rakam, boşluk, nokta, tire, alt çizgi; en fazla 56 karakter).
var countryHeaderRegex = regexp.MustCompile(`^[a-zA-Z0-9 ._-]{1,56}$`)

// sanitizeCountry CF-IPCountry / X-Country-Code gibi tamamen istemci kontrollü
// başlıkları doğrular. Güvenlik: başlık sahtelenebilir; biçim dışı veya kontrol
// karakterli değerlerin veritabanına (ve yönetici paneli grafiklerine) girmesini engeller.
func sanitizeCountry(header string) string {
	header = strings.TrimSpace(stripControlChars(header))
	if !countryHeaderRegex.MatchString(header) {
		return "Bilinmiyor"
	}
	return header
}

// stripControlChars kontrol karakterlerini (\n, \r, \t, \x00 vb.) kaldırır.
// Güvenlik: log satırı enjeksiyonunu ve terminal/DB bozucu baytları engeller.
func stripControlChars(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

// truncateString dizeyi rune sınırını bozmadan en fazla max bayta kısaltır.
func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.ValidString(s[:max]) {
		max--
	}
	return s[:max]
}

// getCountryByIP IP adresinden asenkron ülke tespiti yapar.
func getCountryByIP(ip string) string {
	// Geçerli bir IP değilse hiç sorgulama yapmadan döner (SSRF/path manipülasyon engeli)
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "Bilinmiyor"
	}
	// Yerel (RFC1918 / loopback / link-local vb.) adresler için dış sorgu yapma
	if parsed.IsLoopback() || parsed.IsPrivate() || parsed.IsLinkLocalUnicast() || parsed.IsUnspecified() {
		return "Türkiye"
	}

	// ip-api.com üzerinden sorgulama (1 saniye zaman aşımı ile)
	client := http.Client{
		Timeout: 1 * time.Second,
	}

	resp, err := client.Get(fmt.Sprintf("http://ip-api.com/json/%s?fields=country", parsed.String()))
	if err != nil {
		return "Bilinmiyor"
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "Bilinmiyor"
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "Bilinmiyor"
	}

	var result struct {
		Country string `json:"country"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "Bilinmiyor"
	}

	if result.Country == "" {
		return "Bilinmiyor"
	}

	return result.Country
}
