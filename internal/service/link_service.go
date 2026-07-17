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
}

// aliasRegex özel alias'lar için izin verilen karakter şablonudur (sadece İngilizce harfler, rakamlar, tire ve alt çizgi).
var aliasRegex = regexp.MustCompile(`^[a-zA-Z0-9-_]+$`)

// ShortenURL orijinal bir URL'i kısaltır ve veritabanına kaydeder.
func (s *LinkService) ShortenURL(originalURL string, customAlias string, maxClicks int64, createdByID int64) (*model.Link, error) {
	originalURL = strings.TrimSpace(originalURL)
	if originalURL == "" {
		return nil, errors.New("orijinal URL boş olamaz")
	}

	// Eğer protokol yoksa varsayılan olarak https:// ekle
	if !strings.HasPrefix(originalURL, "http://") && !strings.HasPrefix(originalURL, "https://") {
		originalURL = "https://" + originalURL
	}

	// URL formatını doğrula
	u, err := url.ParseRequestURI(originalURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("geçersiz URL formatı. Lütfen geçerli bir internet adresi yazın")
	}

	// URL uzunluk sınırı (aşırı uzun URL'ler DB şişirmesi / XSS yüzeyi)
	if len(originalURL) > 2048 {
		return nil, errors.New("URL çok uzun (maksimum 2048 karakter)")
	}

	var shortCode string

	if customAlias != "" {
		customAlias = strings.TrimSpace(customAlias)
		// Özel takma ad format doğrulaması
		if len(customAlias) < 3 || len(customAlias) > 30 {
			return nil, errors.New("özel takma ad en az 3, en fazla 30 karakter olmalıdır")
		}
		if !aliasRegex.MatchString(customAlias) {
			return nil, errors.New("özel takma ad sadece harf, rakam, tire (-) ve alt çizgi (_) içerebilir")
		}

		// Rezerve kelime kontrolü
		lowerAlias := strings.ToLower(customAlias)
		if ReservedAliases[lowerAlias] {
			return nil, fmt.Errorf("'%s' özel takma adı sistem tarafından rezerve edilmiştir", customAlias)
		}

		// Çakışma kontrolü
		existing, err := s.repo.GetByShortCode(customAlias)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, errors.New("bu özel takma ad zaten kullanımda")
		}

		shortCode = customAlias
	} else {
		// Rastgele benzersiz bir kısa kod üret (Base62 algoritması mantığıyla 6 karakterli)
		var err error
		shortCode, err = s.generateUniqueShortCode()
		if err != nil {
			return nil, err
		}
	}

	l := &model.Link{
		OriginalURL: originalURL,
		ShortCode:   shortCode,
		CustomAlias: customAlias,
		CreatedByID: createdByID,
		ClickCount:  0,
		MaxClicks:   maxClicks,
	}

	if err := s.repo.Create(l); err != nil {
		return nil, err
	}

	return l, nil
}

// GetOriginalURL kısa koda göre orijinal URL'i döner ve asenkron tıklanma sayacını artırır (tıklama sınırını da kontrol eder).
func (s *LinkService) GetOriginalURL(shortCode string) (*model.Link, error) {
	link, err := s.repo.GetByShortCode(shortCode)
	if err != nil {
		return nil, err
	}
	if link == nil {
		return nil, nil
	}

	// Tıklama limiti kontrolü ve sayaç artırımı atomik yapılır (yarış durumu engeli).
	// SQLite tek yazıcı olduğundan koşullu UPDATE, eşzamanlı istekler arasında
	// limit aşımını (TOCTOU) garantili olarak önler.
	if link.MaxClicks > 0 {
		incremented, err := s.repo.IncrementClickIfBelowLimit(link.ID)
		if err != nil {
			return nil, err
		}
		if !incremented {
			return nil, errors.New("MAX_CLICKS_REACHED")
		}
	} else {
		// Limitsiz linklerde koşulsuz artırım yeterlidir
		_ = s.repo.IncrementClick(link.ID)
	}

	return link, nil
}

// GetLinkByShortCode kısa koda göre link kaydını döner (tıklama sayacını artırmaz ve limit kontrolü yapmaz).
func (s *LinkService) GetLinkByShortCode(shortCode string) (*model.Link, error) {
	return s.repo.GetByShortCode(shortCode)
}

// UpdateLink var olan bir kısaltılmış linki günceller (yetki ve format kontrolü ile).
func (s *LinkService) UpdateLink(oldShortCode string, originalURL string, customAlias string, maxClicks int64, requesterID int64, requesterRole string) (*model.Link, error) {
	link, err := s.repo.GetByShortCode(oldShortCode)
	if err != nil {
		return nil, err
	}
	if link == nil {
		return nil, errors.New("link bulunamadı")
	}

	// Yetki kontrolü
	if requesterRole != model.RoleSuperadmin && link.CreatedByID != requesterID {
		return nil, errors.New("bu linki düzenlemek için yetkiniz bulunmamaktadır")
	}

	// Orijinal URL temizle ve doğrula
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

	// URL uzunluk sınırı
	if len(originalURL) > 2048 {
		return nil, errors.New("URL çok uzun (maksimum 2048 karakter)")
	}

	customAlias = strings.TrimSpace(customAlias)
	var finalShortCode string

	// Eğer yeni alias eskisinden farklıysa çakışma ve validasyon kontrolü yap
	if customAlias != "" && customAlias != link.ShortCode && customAlias != link.CustomAlias {
		// Validasyonlar
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

		// Çakışma kontrolü
		existing, err := s.repo.GetByShortCode(customAlias)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, errors.New("bu özel takma ad zaten kullanımda")
		}
		finalShortCode = customAlias
	} else if customAlias == "" && link.CustomAlias != "" {
		// Eğer kullanıcı önceden özel alias girmiş ama şimdi boş bırakarak rastgele kod istiyorsa yeni kod üretelim
		code, err := s.generateUniqueShortCode()
		if err != nil {
			return nil, err
		}
		finalShortCode = code
	} else {
		// Değişiklik yoksa eskisini koru
		finalShortCode = link.ShortCode
		if customAlias == "" {
			customAlias = link.CustomAlias
		}
	}

	// Değerleri güncelle
	link.OriginalURL = originalURL
	link.ShortCode = finalShortCode
	link.CustomAlias = customAlias
	link.MaxClicks = maxClicks

	if err := s.repo.Update(link); err != nil {
		return nil, err
	}

	return link, nil
}

// GetLinksByUserID kullanıcının kendi kısalttığı linkleri getirir.
func (s *LinkService) GetLinksByUserID(userID int64) ([]*model.Link, error) {
	return s.repo.GetByUserID(userID)
}

// GetAllLinks tüm sistem linklerini listeler (Superadmin için).
func (s *LinkService) GetAllLinks() ([]*model.Link, error) {
	return s.repo.GetAll()
}

// DeleteLinkByShortCode kısa koda göre linki siler ve yetki kontrolü yapar.
func (s *LinkService) DeleteLinkByShortCode(shortCode string, requesterID int64, requesterRole string) error {
	link, err := s.repo.GetByShortCode(shortCode)
	if err != nil {
		return err
	}
	if link == nil {
		return errors.New("link bulunamadı")
	}

	// Yetki Kontrolü: Yalnızca kendi linkini silen üye veya Superadmin silebilir.
	if requesterRole != model.RoleSuperadmin && link.CreatedByID != requesterID {
		return errors.New("bu linki silmek için yetkiniz bulunmamaktadır")
	}

	return s.repo.Delete(link.ID)
}

// generateUniqueShortCode çakışmayan, benzersiz 6 haneli Base62 kodu üretir.
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

// generateRandomBase62 belirtilen uzunlukta rastgele Base62 karakter dizisi üretir.
func generateRandomBase62(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, length)
	_, _ = rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = chars[b%62]
	}
	return string(bytes)
}
