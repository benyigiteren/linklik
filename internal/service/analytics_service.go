package service

import (
	"context"
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
	"sync"
	"time"
	"unicode/utf8"
)

// redirectJob arka planda işlenecek tıklama analitik işidir.
type redirectJob struct {
	linkID        int64
	ipAddress     string
	userAgent     string
	referrer      string
	headerCountry string
}

// AnalyticsService analitik verilerini asenkron worker havuzu ile işleyen servis katmanıdır.
type AnalyticsService struct {
	repo         *repository.AnalyticsRepository
	jobQueue     chan *redirectJob
	wg           sync.WaitGroup
	countryCache sync.Map // IP -> Ülke önbelleği (dış API kotalarını tüketmemek ve gecikmeyi önlemek için)
	httpClient   *http.Client
	closed       bool
	closeMu      sync.Mutex
}

// NewAnalyticsService yeni bir AnalyticsService örneği oluşturur ve arka plan worker havuzunu başlatır.
func NewAnalyticsService(repo *repository.AnalyticsRepository) *AnalyticsService {
	s := &AnalyticsService{
		repo:     repo,
		jobQueue: make(chan *redirectJob, 10000), // 10.000 arabellekli kuyruk
		httpClient: &http.Client{
			Timeout: 1500 * time.Millisecond,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}

	// 4 adet arka plan worker başlat
	const workerCount = 4
	for i := 0; i < workerCount; i++ {
		s.wg.Add(1)
		go s.worker()
	}

	return s
}

// worker kuyruktan analitik işlerini çekip veritabanına kaydeder.
func (s *AnalyticsService) worker() {
	defer s.wg.Done()
	for job := range s.jobQueue {
		s.processJob(job)
	}
}

func (s *AnalyticsService) processJob(job *redirectJob) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Analytics Error] Worker panik kurtarıldı: %v", r)
		}
	}()

	cleanIP := cleanIPAddress(job.ipAddress)

	country := "Bilinmiyor"
	if job.headerCountry != "" {
		country = sanitizeCountry(job.headerCountry)
	} else {
		country = s.resolveCountry(cleanIP)
	}

	ua := truncateString(stripControlChars(job.userAgent), 512)
	browser, os := parseUserAgent(ua)
	cleanRef := cleanReferrerURL(job.referrer)

	analytics := &model.Analytics{
		LinkID:    job.linkID,
		IPAddress: cleanIP,
		Referrer:  cleanRef,
		UserAgent: ua,
		Browser:   browser,
		OS:        os,
		Country:   country,
	}

	if err := s.repo.Create(analytics); err != nil {
		log.Printf("[Analytics Error] Analitik kaydedilemedi (Link ID: %d): %v", job.linkID, err)
	}
}

// LogRedirectAsync tıklama verisini arabellekli worker kuyruğuna ekler (non-blocking).
func (s *AnalyticsService) LogRedirectAsync(linkID int64, ipAddress, userAgent, referrer, headerCountry string) {
	s.closeMu.Lock()
	if s.closed {
		s.closeMu.Unlock()
		return
	}
	s.closeMu.Unlock()

	job := &redirectJob{
		linkID:        linkID,
		ipAddress:     ipAddress,
		userAgent:     userAgent,
		referrer:      referrer,
		headerCountry: headerCountry,
	}

	select {
	case s.jobQueue <- job:
	default:
		// Kuyruk tamamen doluysa işlemi drop et ve sunucunun kilitlenmesini engelle
		log.Printf("[Analytics Warning] Analitik kuyruğu dolu, iş atlandı (Link ID: %d)", linkID)
	}
}

// Shutdown worker havuzunu güvenle sonlandırır ve kuyruktaki işleri tüketir.
func (s *AnalyticsService) Shutdown(ctx context.Context) {
	s.closeMu.Lock()
	if s.closed {
		s.closeMu.Unlock()
		return
	}
	s.closed = true
	close(s.jobQueue)
	s.closeMu.Unlock()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("[Analytics] Tüm analitik işleri başarıyla veritabanına yazıldı.")
	case <-ctx.Done():
		log.Println("[Analytics Warning] Zaman aşımı: Kalan analitik işleri beklenmeden çıkıldı.")
	}
}

// ResetStats linkin istatistiklerini sıfırlar.
func (s *AnalyticsService) ResetStats(linkID int64) error {
	return s.repo.DeleteByLinkID(linkID)
}

// GetStats belirli bir linke ait analitik özetini döner.
func (s *AnalyticsService) GetStats(linkID int64) (*model.LinkStats, error) {
	return s.repo.GetStatsByLinkID(linkID)
}

// resolveCountry IP adresinden ülke bilgisini önce önbellekten, yoksa dış API'den sorgular.
func (s *AnalyticsService) resolveCountry(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "Bilinmiyor"
	}
	if parsed.IsLoopback() || parsed.IsPrivate() || parsed.IsLinkLocalUnicast() || parsed.IsUnspecified() {
		return "Yerel Ağ / Özel"
	}

	// Önbellek kontrolü
	if cached, ok := s.countryCache.Load(ip); ok {
		return cached.(string)
	}

	// Dış API sorgulama (ip-api.com)
	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=country", parsed.String())
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "Bilinmiyor"
	}
	req.Header.Set("User-Agent", "Linklik-Shortener/1.0")

	resp, err := s.httpClient.Do(req)
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
	if err := json.Unmarshal(body, &result); err != nil || result.Country == "" {
		return "Bilinmiyor"
	}

	s.countryCache.Store(ip, result.Country)
	return result.Country
}

// parseUserAgent basit ve hızlı bir User-Agent ayrıştırıcısıdır.
func parseUserAgent(ua string) (browser, os string) {
	if ua == "" {
		return "Bilinmiyor", "Bilinmiyor"
	}

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

var countryHeaderRegex = regexp.MustCompile(`^[a-zA-Z0-9 ._-]{1,56}$`)

func sanitizeCountry(header string) string {
	header = strings.TrimSpace(stripControlChars(header))
	if !countryHeaderRegex.MatchString(header) {
		return "Bilinmiyor"
	}
	return header
}

func stripControlChars(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.ValidString(s[:max]) {
		max--
	}
	return s[:max]
}
