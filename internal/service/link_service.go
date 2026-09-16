package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"linklik/internal/model"
	"linklik/internal/repository"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// LinkService link kısaltma ve sorgulama iş mantığı katmanıdır.
type LinkService struct {
	repo *repository.LinkRepository
}

// NewLinkService yeni bir LinkService örneği oluşturur.
func NewLinkService(repo *repository.LinkRepository) *LinkService {
	return &LinkService{repo: repo}
}

// ReservedAliases sistem rotalarıyla çakışmaması gereken rezerve kelimelerdir.
var ReservedAliases = map[string]bool{
	"api":         true,
	"static":      true,
	"setup":       true,
	"login":       true,
	"logout":      true,
	"dashboard":   true,
	"favicon.ico": true,
	"robots.txt":  true,
	"index.html":  true,
	"mcp":         true,
	"healthz":     true,
	"readyz":      true,
}

// aliasRegex özel alias'lar için izin verilen karakter şablonudur.
var aliasRegex = regexp.MustCompile(`^[a-zA-Z0-9-_]+$`)

// ShortenURL orijinal bir URL'i kısaltır ve veritabanına kaydeder.
func (s *LinkService) ShortenURL(originalURL string, customAlias string, maxClicks int64, expiresAtStr *string, password string, isActive *bool, createdByID int64) (*model.Link, error) {
	originalURL = strings.TrimSpace(originalURL)
	if originalURL == "" {
		return nil, errors.New("orijinal URL boş olamaz")
	}

	if !strings.HasPrefix(originalURL, "http://") && !strings.HasPrefix(originalURL, "https://") {
		originalURL = "https://" + originalURL
	}

	u, err := url.ParseRequestURI(originalURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("geçersiz URL formatı. Lütfen geçerli bir internet adresi yazın")
	}

	if len(originalURL) > 2048 {
		return nil, errors.New("URL çok uzun (maksimum 2048 karakter)")
	}

	var shortCode string
	if customAlias != "" {
		customAlias = strings.TrimSpace(customAlias)
		if len(customAlias) < 3 || len(customAlias) > 30 {
			return nil, errors.New("özel takma ad en az 3, en fazla 30 karakter olmalıdır")
		}
		if !aliasRegex.MatchString(customAlias) {
			return nil, errors.New("özel takma ad sadece harf, rakam, tire (-) ve alt çizgi (_) içerebilir")
		}

		lowerAlias := strings.ToLower(customAlias)
		if ReservedAliases[lowerAlias] {
			return nil, fmt.Errorf("'%s' özel takma adı sistem tarafından rezerve edilmiştir", customAlias)
		}

		existing, err := s.repo.GetByShortCode(customAlias)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, errors.New("bu özel takma ad zaten kullanımda")
		}

		shortCode = customAlias
	} else {
		var err error
		shortCode, err = s.generateUniqueShortCode()
		if err != nil {
			return nil, err
		}
	}

	// Son kullanma tarihi ayrıştırma
	var expiresAt *time.Time
	if expiresAtStr != nil && strings.TrimSpace(*expiresAtStr) != "" {
		parsed, err := parseDateString(strings.TrimSpace(*expiresAtStr))
		if err != nil {
			return nil, fmt.Errorf("geçersiz son kullanma tarihi: %v", err)
		}
		if parsed.Before(time.Now()) {
			return nil, errors.New("son kullanma tarihi geçmiş bir tarih olamaz")
		}
		expiresAt = &parsed
	}

	// Şifre hashleme
	var passwordHash string
	if strings.TrimSpace(password) != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(password)), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("şifre hashlenemedi: %v", err)
		}
		passwordHash = string(hash)
	}

	active := true
	if isActive != nil {
		active = *isActive
	}

	l := &model.Link{
		OriginalURL:  originalURL,
		ShortCode:    shortCode,
		CustomAlias:  customAlias,
		CreatedByID:  createdByID,
		ClickCount:   0,
		MaxClicks:    maxClicks,
		IsActive:     active,
		ExpiresAt:    expiresAt,
		PasswordHash: passwordHash,
		HasPassword:  passwordHash != "",
	}

	if err := s.repo.Create(l); err != nil {
		return nil, err
	}

	return l, nil
}

// GetOriginalURL kısa koda göre linki getirir, aktiflik, süre ve şifre kontrolü yapar, tıklama sayacını artırır.
func (s *LinkService) GetOriginalURL(shortCode string) (*model.Link, error) {
	link, err := s.repo.GetByShortCode(shortCode)
	if err != nil {
		return nil, err
	}
	if link == nil {
		return nil, nil
	}

	// 1. Aktiflik Kontrolü
	if !link.IsActive {
		return link, errors.New("LINK_INACTIVE")
	}

	// 2. Son Kullanma Tarihi Kontrolü
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		return link, errors.New("LINK_EXPIRED")
	}

	// 3. Şifre Kontrolü (Şifreli linkler şifre ekranına yönlendirilmeli)
	if link.PasswordHash != "" {
		return link, errors.New("PASSWORD_REQUIRED")
	}

	// 4. Tıklama Limiti Kontrolü ve Sayaç Artırımı
	if link.MaxClicks > 0 {
		incremented, err := s.repo.IncrementClickIfBelowLimit(link.ID)
		if err != nil {
			return nil, err
		}
		if !incremented {
			return link, errors.New("MAX_CLICKS_REACHED")
		}
	} else {
		_ = s.repo.IncrementClick(link.ID)
	}

	return link, nil
}

// VerifyPasswordAndGetURL şifreli bir linkin şifresini doğrular ve geçerliyse tıklamayı artırarak linki döner.
func (s *LinkService) VerifyPasswordAndGetURL(shortCode, password string) (*model.Link, error) {
	link, err := s.repo.GetByShortCode(shortCode)
	if err != nil {
		return nil, err
	}
	if link == nil {
		return nil, errors.New("link bulunamadı")
	}

	if !link.IsActive {
		return nil, errors.New("LINK_INACTIVE")
	}

	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		return nil, errors.New("LINK_EXPIRED")
	}

	if link.PasswordHash == "" {
		return link, nil
	}

	if err := bcrypt.CompareHashAndPassword([]byte(link.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("hatalı şifre")
	}

	// Sayaç artır
	if link.MaxClicks > 0 {
		incremented, err := s.repo.IncrementClickIfBelowLimit(link.ID)
		if err != nil {
			return nil, err
		}
		if !incremented {
			return nil, errors.New("MAX_CLICKS_REACHED")
		}
	} else {
		_ = s.repo.IncrementClick(link.ID)
	}

	return link, nil
}

// ToggleLinkActive linkin aktif/pasif durumunu değiştirir.
func (s *LinkService) ToggleLinkActive(shortCode string, requesterID int64, requesterRole string) (bool, error) {
	link, err := s.repo.GetByShortCode(shortCode)
	if err != nil {
		return false, err
	}
	if link == nil {
		return false, errors.New("link bulunamadı")
	}

	if requesterRole != model.RoleSuperadmin && link.CreatedByID != requesterID {
		return false, errors.New("bu linki düzenleme yetkiniz bulunmamaktadır")
	}

	return s.repo.ToggleActive(link.ID)
}

// ResetLinkStats linkin tıklama sayısını ve analitik geçmişini sıfırlar.
func (s *LinkService) ResetLinkStats(shortCode string, requesterID int64, requesterRole string) error {
	link, err := s.repo.GetByShortCode(shortCode)
	if err != nil {
		return err
	}
	if link == nil {
		return errors.New("link bulunamadı")
	}

	if requesterRole != model.RoleSuperadmin && link.CreatedByID != requesterID {
		return errors.New("bu linkin istatistiklerini sıfırlama yetkiniz bulunmamaktadır")
	}

	return s.repo.ResetStats(link.ID)
}

// GetLinkByShortCode kısa koda göre link kaydını döner.
func (s *LinkService) GetLinkByShortCode(shortCode string) (*model.Link, error) {
	return s.repo.GetByShortCode(shortCode)
}

// GetPaginatedLinks sayfalama ve arama destekli link listesi döner.
func (s *LinkService) GetPaginatedLinks(userID int64, isSuperadmin bool, page, limit int, search, status string) ([]*model.Link, int64, error) {
	return s.repo.GetPaginated(userID, isSuperadmin, page, limit, search, status)
}

// UpdateLink var olan bir kısaltılmış linki günceller.
func (s *LinkService) UpdateLink(oldShortCode string, originalURL string, customAlias string, maxClicks int64, expiresAtStr *string, password string, isActive *bool, requesterID int64, requesterRole string) (*model.Link, error) {
	link, err := s.repo.GetByShortCode(oldShortCode)
	if err != nil {
		return nil, err
	}
	if link == nil {
		return nil, errors.New("link bulunamadı")
	}

	if requesterRole != model.RoleSuperadmin && link.CreatedByID != requesterID {
		return nil, errors.New("bu linki düzenlemek için yetkiniz bulunmamaktadır")
	}

	originalURL = strings.TrimSpace(originalURL)
	if originalURL == "" {
		return nil, errors.New("orijinal URL boş olamaz")
	}

	if !strings.HasPrefix(originalURL, "http://") && !strings.HasPrefix(originalURL, "https://") {
		originalURL = "https://" + originalURL
	}

	u, err := url.ParseRequestURI(originalURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("geçersiz URL formatı")
	}

	if len(originalURL) > 2048 {
		return nil, errors.New("URL çok uzun (maksimum 2048 karakter)")
	}

	customAlias = strings.TrimSpace(customAlias)
	var finalShortCode string

	if customAlias != "" && customAlias != link.ShortCode && customAlias != link.CustomAlias {
		if len(customAlias) < 3 || len(customAlias) > 30 {
			return nil, errors.New("özel takma ad en az 3, en fazla 30 karakter olmalıdır")
		}
		if !aliasRegex.MatchString(customAlias) {
			return nil, errors.New("özel takma ad sadece harf, rakam, tire (-) ve alt çizgi (_) içerebilir")
		}

		lowerAlias := strings.ToLower(customAlias)
		if ReservedAliases[lowerAlias] {
			return nil, fmt.Errorf("'%s' özel takma adı sistem tarafından rezerve edilmiştir", customAlias)
		}

		existing, err := s.repo.GetByShortCode(customAlias)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, errors.New("bu özel takma ad zaten kullanımda")
		}
		finalShortCode = customAlias
	} else if customAlias == "" && link.CustomAlias != "" {
		code, err := s.generateUniqueShortCode()
		if err != nil {
			return nil, err
		}
		finalShortCode = code
	} else {
		finalShortCode = link.ShortCode
		if customAlias == "" {
			customAlias = link.CustomAlias
		}
	}

	// Son kullanma tarihi güncelleme
	if expiresAtStr != nil {
		str := strings.TrimSpace(*expiresAtStr)
		if str == "" || strings.EqualFold(str, "clear") {
			link.ExpiresAt = nil
		} else {
			parsed, err := parseDateString(str)
			if err != nil {
				return nil, fmt.Errorf("geçersiz son kullanma tarihi: %v", err)
			}
			link.ExpiresAt = &parsed
		}
	}

	// Şifre güncelleme
	if password != "" {
		if strings.EqualFold(password, "remove") || strings.EqualFold(password, "clear") {
			link.PasswordHash = ""
		} else {
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return nil, fmt.Errorf("şifre hashlenemedi: %v", err)
			}
			link.PasswordHash = string(hash)
		}
	}

	if isActive != nil {
		link.IsActive = *isActive
	}

	link.OriginalURL = originalURL
	link.ShortCode = finalShortCode
	link.CustomAlias = customAlias
	link.MaxClicks = maxClicks
	link.HasPassword = link.PasswordHash != ""

	if err := s.repo.Update(link); err != nil {
		return nil, err
	}

	return link, nil
}

// GetLinksByUserID kullanıcının kendi kısalttığı linkleri getirir.
func (s *LinkService) GetLinksByUserID(userID int64) ([]*model.Link, error) {
	return s.repo.GetByUserID(userID)
}

// GetAllLinks tüm sistem linklerini listeler.
func (s *LinkService) GetAllLinks() ([]*model.Link, error) {
	return s.repo.GetAll()
}

// DeleteLinkByShortCode kısa koda göre linki siler.
func (s *LinkService) DeleteLinkByShortCode(shortCode string, requesterID int64, requesterRole string) error {
	link, err := s.repo.GetByShortCode(shortCode)
	if err != nil {
		return err
	}
	if link == nil {
		return errors.New("link bulunamadı")
	}

	if requesterRole != model.RoleSuperadmin && link.CreatedByID != requesterID {
		return errors.New("bu linki silmek için yetkiniz bulunmamaktadır")
	}

	return s.repo.Delete(link.ID)
}

func (s *LinkService) generateUniqueShortCode() (string, error) {
	const maxTries = 10
	for i := 0; i < maxTries; i++ {
		code := generateRandomBase62(6)
		existing, err := s.repo.GetByShortCode(code)
		if err != nil {
			return "", err
		}
		if existing == nil && !ReservedAliases[strings.ToLower(code)] {
			return code, nil
		}
	}
	return "", errors.New("benzersiz kısa kod üretilemedi, lütfen tekrar deneyin")
}

func generateRandomBase62(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, length)
	_, _ = rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = chars[b%62]
	}
	return string(bytes)
}

func parseDateString(str string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.ParseInLocation(f, str, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("desteklenmeyen tarih formatı (beklenen örn: 2026-12-31T23:59)")
}
