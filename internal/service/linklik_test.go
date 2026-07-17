package service_test

import (
	"linklik/internal/config"
	"linklik/internal/db"
	"linklik/internal/model"
	"linklik/internal/repository"
	"linklik/internal/service"
	"testing"
)

// setupTestDB testler için bellek-içi (in-memory) SQLite veritabanını başlatır.
func setupTestDB(t *testing.T) (*service.UserService, *service.LinkService) {
	config.GlobalConfig = &config.Config{
		Port:      "8080",
		DBPath:    ":memory:",
		JWTSecret: []byte("test_secret_key_1234567890"),
		BaseURL:   "http://localhost:8080",
	}

	err := db.InitDB(config.GlobalConfig.DBPath)
	if err != nil {
		t.Fatalf("Test veritabanı başlatılamadı: %v", err)
	}

	userRepo := repository.NewUserRepository()
	linkRepo := repository.NewLinkRepository()

	return service.NewUserService(userRepo), service.NewLinkService(linkRepo)
}

// TestSetupAndUserCreation ilk kurulum, superadmin yetkilendirmesi, normal kullanıcı oluşturma ve giriş doğrulamalarını test eder.
func TestSetupAndUserCreation(t *testing.T) {
	userService, _ := setupTestDB(t)
	defer db.CloseDB()

	// 1. Başlangıçta kurulum gerekiyor olmalıdır.
	req, err := userService.IsSetupRequired()
	if err != nil {
		t.Fatalf("Kurulum kontrol hatası: %v", err)
	}
	if !req {
		t.Fatal("Boş veritabanında kurulum gerekli olmalıdır")
	}

	// 2. İlk kullanıcıyı oluştur (Superadmin olmalı).
	u1, err := userService.SetupFirstUser("admin", "adminpass")
	if err != nil {
		t.Fatalf("SetupFirstUser hatası: %v", err)
	}
	if u1.Role != model.RoleSuperadmin {
		t.Fatalf("İlk kullanıcı Superadmin olmalıdır, alınan: %s", u1.Role)
	}

	// 3. Kurulum artık gerekli olmamalıdır.
	req, err = userService.IsSetupRequired()
	if err != nil {
		t.Fatalf("Kurulum kontrol hatası: %v", err)
	}
	if req {
		t.Fatal("Kurulum tamamlandıktan sonra hala kurulum gerekli görünüyor")
	}

	// 4. İkinci kez ilk kurulum yapılamamalıdır.
	_, err = userService.SetupFirstUser("admin2", "adminpass")
	if err == nil {
		t.Fatal("İlk kurulum tamamlandıktan sonra tekrar kurulum yapılamamalıdır")
	}

	// 5. Kullanıcı girişi ve şifre doğrulaması.
	token, authenticatedUser, err := userService.Authenticate("admin", "adminpass")
	if err != nil {
		t.Fatalf("Giriş hatası: %v", err)
	}
	if token == "" || authenticatedUser.Username != "admin" {
		t.Fatal("Giriş başarısız oldu veya token boş döndü")
	}

	// 6. Normal üye (Member) oluşturulması.
	u2, err := userService.CreateUser("uye1", "uyepassword")
	if err != nil {
		t.Fatalf("Üye ekleme hatası: %v", err)
	}
	if u2.Role != model.RoleMember {
		t.Fatalf("Normal kullanıcının rolü düz üye (member) olmalıdır, alınan: %s", u2.Role)
	}
}

// TestLinkShortening link kısaltma kurallarını, çakışmaları, rezerve kelimeleri ve asıl URL'i geri getirmeyi test eder.
func TestLinkShortening(t *testing.T) {
	userService, linkService := setupTestDB(t)
	defer db.CloseDB()

	user, err := userService.SetupFirstUser("admin", "password123")
	if err != nil {
		t.Fatalf("Kullanıcı oluşturulamadı: %v", err)
	}

	// 1. Geçerli URL kısaltma (Rastgele kodlu).
	link, err := linkService.ShortenURL("https://google.com", "", 0, user.ID)
	if err != nil {
		t.Fatalf("Link kısaltma hatası: %v", err)
	}
	if len(link.ShortCode) != 6 {
		t.Fatalf("Rastgele kısa kod 6 haneli olmalıdır, alınan: %s (uzunluk %d)", link.ShortCode, len(link.ShortCode))
	}

	// 2. Özel alias ile kısaltma.
	aliasLink, err := linkService.ShortenURL("https://github.com", "kod-deposu", 0, user.ID)
	if err != nil {
		t.Fatalf("Özel takma ad ile kısaltma hatası: %v", err)
	}
	if aliasLink.ShortCode != "kod-deposu" {
		t.Fatalf("Özel alias eşleşmiyor, beklenen 'kod-deposu', alınan: %s", aliasLink.ShortCode)
	}

	// 3. Aynı alias çakışması engellenmelidir.
	_, err = linkService.ShortenURL("https://gitlab.com", "kod-deposu", 0, user.ID)
	if err == nil {
		t.Fatal("Aynı özel alias ikinci kez kullanılabilmemelidir")
	}

	// 4. Sistem rezerve kelimeleri engellenmelidir.
	_, err = linkService.ShortenURL("https://gitlab.com", "login", 0, user.ID)
	if err == nil {
		t.Fatal("Rezerve alias ('login') kullanılamamalıdır")
	}

	// 5. Protokolsüz URL kısaltma otomatik https:// eklemeli ve başarılı olmalı
	protoLink, err := linkService.ShortenURL("google.com", "", 0, user.ID)
	if err != nil {
		t.Fatalf("Protokolsüz URL kısaltılamadı: %v", err)
	}
	if protoLink.OriginalURL != "https://google.com" {
		t.Fatalf("Otomatik protokol ekleme hatası, beklenen 'https://google.com', alınan: %s", protoLink.OriginalURL)
	}

	// 6. Geri getirme ve tıklanma sayacının artırılması.
	retrieved, err := linkService.GetOriginalURL("kod-deposu")
	if err != nil {
		t.Fatalf("Link geri çağırma hatası: %v", err)
	}
	if retrieved.OriginalURL != "https://github.com" {
		t.Fatalf("Geri çağırılan URL hatalı, beklenen 'https://github.com', alınan: %s", retrieved.OriginalURL)
	}
}
