package service

import (
	"errors"
	"fmt"
	"linklik/internal/config"
	"linklik/internal/model"
	"linklik/internal/repository"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// UserService kullanıcı yetkilendirme ve yönetim iş mantığı katmanıdır.
type UserService struct {
	repo *repository.UserRepository
}

// NewUserService yeni bir UserService örneği oluşturur.
func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

// usernameRegex kullanıcı adlarının yalnızca güvenli karakter sınıfından oluşmasını zorunlu kılar
// (harf, rakam, nokta, tire, alt çizgi; 3-30 karakter). Bu kural aynı zamanda log injection,
// depolanan XSS (admin panelinde username render edilir) ve yeni satır enjeksiyonunu engeller.
var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]{3,30}$`)

// validateUsernameAndPassword kullanıcı adı biçimini ve şifre uzunluk sınırlarını kontrol eder.
// Tüm kullanıcı oluşturma/güncelleme yollarında çağrılmalıdır.
func validateUsernameAndPassword(username, password string) error {
	username = strings.TrimSpace(username)
	if !usernameRegex.MatchString(username) {
		return errors.New("kullanıcı adı 3-30 karakter olmalı ve yalnızca harf, rakam, nokta, tire (-) veya alt çizgi (_) içerebilir")
	}
	// Boş şifre (yalnızca UpdateProfile'da şifre değiştirilmeyecekse "") geçerlidir; çağıran kontrol eder.
	if password != "" {
		if len(password) < 8 {
			return errors.New("şifre en az 8 karakter olmalıdır")
		}
		if len(password) > 72 {
			return errors.New("şifre 72 karakterden uzun olamaz (bcrypt sınırı)")
		}
	}
	return nil
}

// IsSetupRequired sistemde hiç kullanıcı olup olmadığını kontrol eder.
func (s *UserService) IsSetupRequired() (bool, error) {
	count, err := s.repo.GetCount()
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// SetupFirstUser ilk kullanıcıyı oluşturur ve "Superadmin" yapar.
// Güvenlik: atomik CreateIfNone kullanılarak iki eşzamanlı setup isteği
// yarışını (TOCTOU) önler; yalnızca bir Superadmin oluşur.
func (s *UserService) SetupFirstUser(username, password string) (*model.User, error) {
	required, err := s.IsSetupRequired()
	if err != nil {
		return nil, err
	}
	if !required {
		return nil, errors.New("ilk kurulum zaten tamamlanmış. Yeni kullanıcı eklemek için yönetici girişi yapmalısınız")
	}

	// Kullanıcı adı ve şifre doğrulaması (XSS / log injection / zayıf şifre engeli)
	if err := validateUsernameAndPassword(username, password); err != nil {
		return nil, err
	}

	// Şifreyi hashle
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("şifre hashlenemedi: %v", err)
	}

	// İlk kullanıcı için API anahtarı üret
	apiKey := "lk_" + config.GenerateRandomKey()

	u := &model.User{
		Username:     strings.TrimSpace(username),
		PasswordHash: string(hashedPassword),
		Role:         model.RoleSuperadmin,
		APIKey:       apiKey,
	}

	created, err := s.repo.CreateIfNone(u)
	if err != nil {
		// Kullanıcı adı çakışması benzersiz kısıttan dolayı: setup eşzamanlı yarış kaybıysa
		// kullanıcıyı uyaralım.
		return nil, err
	}
	if !created {
		return nil, errors.New("ilk kurulum zaten tamamlanmış. Yeni kullanıcı eklemek için yönetici girişi yapmalısınız")
	}

	return u, nil
}

// Authenticate kullanıcı adı ve şifreyi kontrol eder, başarılıysa bir JWT token döndürür.
// dummyPasswordHash var olmayan kullanıcılar için zamanlama eşitlemesi amacıyla kullanılır.
// Güvenlik: bcrypt karşılaştırması ~100ms sürer; kullanıcı yoksa anında dönmek,
// yanıt süresi farkından kullanıcı adı numaralandırmasına (user enumeration) izin verir.
// Değer geçerli bir bcrypt hash'idir; hiçbir şifre ile eşleşmez.
var dummyPasswordHash = []byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")

func (s *UserService) Authenticate(username, password string) (string, *model.User, error) {
	u, err := s.repo.GetByUsername(username)
	if err != nil {
		return "", nil, err
	}
	if u == nil {
		// Zamanlama kanalını kapat: gerçek kullanıcıyla aynı maliyette sahte karşılaştırma yap.
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
		return "", nil, errors.New("kullanıcı adı veya şifre hatalı")
	}

	// Şifreyi doğrula
	err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	if err != nil {
		return "", nil, errors.New("kullanıcı adı veya şifre hatalı")
	}

	// JWT Token oluştur
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  u.ID,
		"username": u.Username,
		"role":     u.Role,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString(config.GlobalConfig.GetJWTSecretKey())
	if err != nil {
		return "", nil, fmt.Errorf("token imzalanamadı: %v", err)
	}

	return tokenString, u, nil
}

// CreateUser yeni bir üye (member) oluşturur (Sadece Superadmin yetkilidir).
func (s *UserService) CreateUser(username, password string) (*model.User, error) {
	// Girdi doğrulaması (biçim + şifre politikası)
	if err := validateUsernameAndPassword(username, password); err != nil {
		return nil, err
	}
	username = strings.TrimSpace(username)

	// Kullanıcı adı benzersiz olmalı
	existing, err := s.repo.GetByUsername(username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("bu kullanıcı adı zaten kullanımda")
	}

	// Şifreyi hashle
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("şifre hashlenemedi: %v", err)
	}

	apiKey := "lk_" + config.GenerateRandomKey()

	u := &model.User{
		Username:     username,
		PasswordHash: string(hashedPassword),
		Role:         model.RoleMember,
		APIKey:       apiKey,
	}

	if err := s.repo.Create(u); err != nil {
		return nil, err
	}

	return u, nil
}

// GetUserByAPIKey API anahtarı üzerinden kullanıcı doğrulaması yapar.
func (s *UserService) GetUserByAPIKey(apiKey string) (*model.User, error) {
	return s.repo.GetByAPIKey(apiKey)
}

// GetAllUsers tüm kullanıcıları listeler.
func (s *UserService) GetAllUsers() ([]*model.User, error) {
	return s.repo.GetAll()
}

// RegenerateAPIKey kullanıcının API anahtarını yeniler.
func (s *UserService) RegenerateAPIKey(userID int64) (string, error) {
	newKey := "lk_" + config.GenerateRandomKey()
	err := s.repo.UpdateAPIKey(userID, newKey)
	if err != nil {
		return "", err
	}
	return newKey, nil
}

// GetUserByID ID bazlı kullanıcı sorgular.
func (s *UserService) GetUserByID(id int64) (*model.User, error) {
	return s.repo.GetByID(id)
}

// DeleteUser kullanıcıyı siler (Superadmin yetkisinde). Kendisini silemez.
func (s *UserService) DeleteUser(userID int64, requesterID int64) error {
	if userID == requesterID {
		return errors.New("kendi hesabınızı silemezsiniz")
	}
	
	user, err := s.repo.GetByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("kullanıcı bulunamadı")
	}
	
	if user.Role == model.RoleSuperadmin {
		return errors.New("başka bir yönetici (Superadmin) hesabını silemezsiniz")
	}
	
	return s.repo.Delete(userID)
}

// UpdateProfile kullanıcının kullanıcı adı ve/veya şifresini günceller ve yeni bir JWT token üretir.
func (s *UserService) UpdateProfile(userID int64, username, password string) (*model.User, string, error) {
	// Girdi doğrulaması (username her zaman verilir; password opsiyonel)
	if err := validateUsernameAndPassword(username, password); err != nil {
		return nil, "", err
	}
	username = strings.TrimSpace(username)

	// Önce kullanıcıyı al
	u, err := s.repo.GetByID(userID)
	if err != nil {
		return nil, "", err
	}
	if u == nil {
		return nil, "", errors.New("kullanıcı bulunamadı")
	}

	// Kullanıcı adı değiştiyse çakışma kontrolü yap
	if username != u.Username {
		existing, err := s.repo.GetByUsername(username)
		if err != nil {
			return nil, "", err
		}
		if existing != nil {
			return nil, "", errors.New("bu kullanıcı adı zaten kullanımda")
		}
		u.Username = username
	}

	// Şifre güncellenecekse hashle ve ata
	if password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return nil, "", fmt.Errorf("şifre hashlenemedi: %v", err)
		}
		u.PasswordHash = string(hashedPassword)
	}

	// Veritabanında güncelle
	if err := s.repo.Update(u); err != nil {
		return nil, "", err
	}

	// Yeni JWT token oluştur
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  u.ID,
		"username": u.Username,
		"role":     u.Role,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString(config.GlobalConfig.GetJWTSecretKey())
	if err != nil {
		return nil, "", fmt.Errorf("token imzalanamadı: %v", err)
	}

	return u, tokenString, nil
}

// ResetUserPassword bir üyenin şifresini yönetici (Superadmin) yetkisiyle sıfırlar.
func (s *UserService) ResetUserPassword(targetUserID int64, newPassword string, requesterID int64) error {
	if len(newPassword) < 8 {
		return errors.New("yeni şifre en az 8 karakter olmalıdır")
	}
	if len(newPassword) > 72 {
		return errors.New("şifre 72 karakterden uzun olamaz (bcrypt sınırı)")
	}

	targetUser, err := s.repo.GetByID(targetUserID)
	if err != nil {
		return err
	}
	if targetUser == nil {
		return errors.New("kullanıcı bulunamadı")
	}

	// Başka bir Superadmin'in şifresi sadece kendi tarafından profil ayarlarından güncellenebilir
	if targetUser.Role == model.RoleSuperadmin && targetUser.ID != requesterID {
		return errors.New("başka bir yöneticinin (Superadmin) şifresi sıfırlanamaz")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("şifre hashlenemedi: %v", err)
	}

	return s.repo.UpdatePassword(targetUserID, string(hashedPassword))
}

