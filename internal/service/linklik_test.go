package service_test

import (
	"linklik/internal/config"
	"linklik/internal/db"
	"linklik/internal/model"
	"linklik/internal/repository"
	"linklik/internal/service"
	"testing"
	"time"
)

// setupTestDB testler için bellek-içi (in-memory) SQLite veritabanını başlatır.
func setupTestDB(t *testing.T) (*service.UserService, *service.LinkService, *service.AnalyticsService) {
	config.GlobalConfig = &config.Config{
		Port:      "8080",
		DBPath:    ":memory:",
		JWTSecret: []byte("test_secret_key_1234567890_at_least_32_bytes_long"),
		BaseURL:   "http://localhost:8080",
	}

	err := db.InitDB(config.GlobalConfig.DBPath)
	if err != nil {
		t.Fatalf("Test veritabanı başlatılamadı: %v", err)
	}

	userRepo := repository.NewUserRepository()
	linkRepo := repository.NewLinkRepository()
	analyticsRepo := repository.NewAnalyticsRepository()

	return service.NewUserService(userRepo), service.NewLinkService(linkRepo), service.NewAnalyticsService(analyticsRepo)
}

// TestSetupAndUserCreation ilk kurulum, superadmin yetkilendirmesi, normal kullanıcı oluşturma ve giriş doğrulamalarını test eder.
func TestSetupAndUserCreation(t *testing.T) {
	userService, _, _ := setupTestDB(t)
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
	userService, linkService, _ := setupTestDB(t)
	defer db.CloseDB()

	user, err := userService.SetupFirstUser("admin", "password123")
	if err != nil {
		t.Fatalf("Kullanıcı oluşturulamadı: %v", err)
	}

	// 1. Geçerli URL kısaltma (Rastgele kodlu).
	link, err := linkService.ShortenURL("https://google.com", "", 0, nil, "", nil, user.ID)
	if err != nil {
		t.Fatalf("Link kısaltma hatası: %v", err)
	}
	if len(link.ShortCode) != 6 {
		t.Fatalf("Rastgele kısa kod 6 haneli olmalıdır, alınan: %s (uzunluk %d)", link.ShortCode, len(link.ShortCode))
	}

	// 2. Özel alias ile kısaltma.
	aliasLink, err := linkService.ShortenURL("https://github.com", "kod-deposu", 0, nil, "", nil, user.ID)
	if err != nil {
		t.Fatalf("Özel takma ad ile kısaltma hatası: %v", err)
	}
	if aliasLink.ShortCode != "kod-deposu" {
		t.Fatalf("Özel alias eşleşmiyor, beklenen 'kod-deposu', alınan: %s", aliasLink.ShortCode)
	}

	// 3. Aynı alias çakışması engellenmelidir.
	_, err = linkService.ShortenURL("https://gitlab.com", "kod-deposu", 0, nil, "", nil, user.ID)
	if err == nil {
		t.Fatal("Aynı özel alias ikinci kez kullanılabilmemelidir")
	}

	// 4. Sistem rezerve kelimeleri engellenmelidir.
	_, err = linkService.ShortenURL("https://gitlab.com", "login", 0, nil, "", nil, user.ID)
	if err == nil {
		t.Fatal("Rezerve alias ('login') kullanılamamalıdır")
	}

	// 5. Protokolsüz URL kısaltma otomatik https:// eklemeli ve başarılı olmalı
	protoLink, err := linkService.ShortenURL("google.com", "", 0, nil, "", nil, user.ID)
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

// TestAdvancedFeatures TTL, Toggle Active, Reset Stats ve Şifre korumasını test eder.
func TestAdvancedFeatures(t *testing.T) {
	userService, linkService, _ := setupTestDB(t)
	defer db.CloseDB()

	user, err := userService.SetupFirstUser("superadmin", "password123")
	if err != nil {
		t.Fatalf("Kullanıcı oluşturulamadı: %v", err)
	}

	// 1. Aktif/Pasif Toggle Testi
	activeLink, err := linkService.ShortenURL("https://test.com/active", "toggle-test", 0, nil, "", nil, user.ID)
	if err != nil {
		t.Fatalf("Link oluşturulamadı: %v", err)
	}
	if !activeLink.IsActive {
		t.Fatal("Yeni oluşturulan link varsayılan olarak aktif olmalıdır")
	}

	// Pasife al
	newStatus, err := linkService.ToggleLinkActive("toggle-test", user.ID, model.RoleSuperadmin)
	if err != nil || newStatus {
		t.Fatalf("ToggleActive pasif yapamadı: %v", err)
	}

	// Pasif link sorgulandığında LINK_INACTIVE hatası dönmeli
	_, err = linkService.GetOriginalURL("toggle-test")
	if err == nil || err.Error() != "LINK_INACTIVE" {
		t.Fatalf("Pasif link için LINK_INACTIVE bekleniyordu, alınan: %v", err)
	}

	// Tekrar aktife al
	newStatus, err = linkService.ToggleLinkActive("toggle-test", user.ID, model.RoleSuperadmin)
	if err != nil || !newStatus {
		t.Fatalf("ToggleActive tekrar aktif yapamadı: %v", err)
	}

	// 2. İstatistik Sıfırlama Testi
	_ = linkService.ResetLinkStats("toggle-test", user.ID, model.RoleSuperadmin)
	freshLink, _ := linkService.GetLinkByShortCode("toggle-test")
	if freshLink.ClickCount != 0 {
		t.Fatalf("Sıfırlama sonrası tıklama 0 olmalı, alınan: %d", freshLink.ClickCount)
	}

	// 3. Şifre Korumalı Link Testi
	passLink, err := linkService.ShortenURL("https://secret.com", "secret-link", 0, nil, "gizlisifre", nil, user.ID)
	if err != nil {
		t.Fatalf("Şifreli link oluşturulamadı: %v", err)
	}
	if !passLink.HasPassword {
		t.Fatal("Linkin has_password alanı true olmalıdır")
	}

	// Doğrudan GetOriginalURL çağrıldığında PASSWORD_REQUIRED dönmeli
	_, err = linkService.GetOriginalURL("secret-link")
	if err == nil || err.Error() != "PASSWORD_REQUIRED" {
		t.Fatalf("Şifreli link için PASSWORD_REQUIRED bekleniyordu, alınan: %v", err)
	}

	// Hatalı şifre ile doğrulama
	_, err = linkService.VerifyPasswordAndGetURL("secret-link", "yanlissifre")
	if err == nil {
		t.Fatal("Hatalı şifre reddedilmeliydi")
	}

	// Doğru şifre ile doğrulama
	verified, err := linkService.VerifyPasswordAndGetURL("secret-link", "gizlisifre")
	if err != nil || verified.OriginalURL != "https://secret.com" {
		t.Fatalf("Doğru şifre doğrulanmadı: %v", err)
	}

	// 4. Son Kullanma Tarihi (TTL) Testi
	futureDate := time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04:05Z")
	ttlLink, err := linkService.ShortenURL("https://timed.com", "timed-link", 0, &futureDate, "", nil, user.ID)
	if err != nil {
		t.Fatalf("TTL link oluşturulamadı: %v", err)
	}
	if ttlLink.ExpiresAt == nil {
		t.Fatal("Linkin expires_at tarihi atanmış olmalı")
	}
}

// TestResetUserPasswordAndLinkCount admin şifre sıfırlama ve kullanıcı link sayısı listesini test eder.
func TestResetUserPasswordAndLinkCount(t *testing.T) {
	userService, linkService, _ := setupTestDB(t)
	defer db.CloseDB()

	// 1. Superadmin ve normal üye oluştur
	admin, err := userService.SetupFirstUser("adminuser", "superadminpass123")
	if err != nil {
		t.Fatalf("Admin oluşturulamadı: %v", err)
	}

	member, err := userService.CreateUser("team_member", "initialpass123")
	if err != nil {
		t.Fatalf("Üye oluşturulamadı: %v", err)
	}

	// 2. Üye için bir link oluştur
	_, err = linkService.ShortenURL("https://member-test.com", "m-link", 0, nil, "", nil, member.ID)
	if err != nil {
		t.Fatalf("Link oluşturulamadı: %v", err)
	}

	// 3. Kullanıcı listesini çek ve link_count değerini kontrol et
	users, err := userService.GetAllUsers()
	if err != nil {
		t.Fatalf("Kullanıcılar listelenemedi: %v", err)
	}

	var foundMember *model.User
	for _, u := range users {
		if u.ID == member.ID {
			foundMember = u
			break
		}
	}
	if foundMember == nil {
		t.Fatal("Üye listede bulunamadı")
	}
	if foundMember.LinkCount != 1 {
		t.Fatalf("Üyenin link_count değeri 1 olmalı, alınan: %d", foundMember.LinkCount)
	}

	// 4. Admin tarafından üyenin şifresini sıfırla
	err = userService.ResetUserPassword(member.ID, "brand_new_secure_pass123", admin.ID)
	if err != nil {
		t.Fatalf("Şifre sıfırlama hatası: %v", err)
	}

	// 5. Eski şifre ile giriş başarısız olmalı
	_, _, err = userService.Authenticate("team_member", "initialpass123")
	if err == nil {
		t.Fatal("Eski şifre ile giriş reddedilmeliydi")
	}

	// 6. Yeni şifre ile giriş başarılı olmalı
	_, authenticatedUser, err := userService.Authenticate("team_member", "brand_new_secure_pass123")
	if err != nil {
		t.Fatalf("Yeni şifre ile giriş yapılamadı: %v", err)
	}
	if authenticatedUser.ID != member.ID {
		t.Fatalf("Doğrulanan kullanıcı ID'si uyuşmuyor: %d vs %d", authenticatedUser.ID, member.ID)
	}

	// 7. Güvenlik: Admin başka bir superadmin'in şifresini sıfırlayamaz kuralını test et
	err = userService.ResetUserPassword(admin.ID, "short", admin.ID) // en az 8 karakter kuralı
	if err == nil {
		t.Fatal("Kısa şifre (short) reddedilmeliydi")
	}
}
