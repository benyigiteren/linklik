package main

import (
	"context"
	"errors"
	"linklik/internal/config"
	"linklik/internal/db"
	"linklik/internal/handler"
	"linklik/internal/middleware"
	"linklik/internal/repository"
	"linklik/internal/service"
	"linklik/web"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chi_middleware "github.com/go-chi/chi/v5/middleware"
)

func main() {
	// 1. Konfigürasyonu Yükle
	config.LoadConfig()

	// 2. Veritabanını Başlat (SQLite & WAL Modu)
	if err := db.InitDB(config.GlobalConfig.DBPath); err != nil {
		log.Fatalf("Veritabanı başlatma hatası: %v", err)
	}
	defer db.CloseDB()

	// 3. Repository Katmanlarını Oluştur
	userRepo := repository.NewUserRepository()
	linkRepo := repository.NewLinkRepository()
	analyticsRepo := repository.NewAnalyticsRepository()

	// 4. Servis Katmanlarını Oluştur
	userService := service.NewUserService(userRepo)
	linkService := service.NewLinkService(linkRepo)
	analyticsService := service.NewAnalyticsService(analyticsRepo)

	// 5. Handler (İstek Karşılayıcı) Katmanlarını Oluştur
	authHandler := handler.NewAuthHandler(userService)
	linkHandler := handler.NewLinkHandler(linkService)
	analyticsHandler := handler.NewAnalyticsHandler(analyticsService, linkService)
	redirectHandler := handler.NewRedirectHandler(linkService, analyticsService)
	uiHandler := handler.NewUIHandler(userService)

	// 6. Chi Router ve Middleware Yapılandırması
	r := chi.NewRouter()

	r.Use(chi_middleware.RequestID)
	// Güvenlik: chi v5 RealIP kullanımdan kaldırıldı (GHSA-3fxj-6jh8-hvhx) ve IP
	// sahteciliğine açıktı. Bunun yerine ClientIPFromRemoteAddr kullanırız (raw
	// TCP PeerAddr). Reverse-proxy arkasında çalışıyorsanız ClientIPFromXFFTrustedProxies
	// kullanın; yoksa saldırgan X-Forwarded-For ile IP'yi sahteleyebilir.
	r.Use(chi_middleware.ClientIPFromRemoteAddr)
	r.Use(middleware.SecurityHeaders)
	r.Use(chi_middleware.Logger)
	r.Use(chi_middleware.Recoverer)
	r.Use(middleware.CORS)

	// Genel gövde boyutu kısıtı (1 MiB) — çalışma zamanı ileti DoS'u için
	r.Use(middleware.MaxBodySize(1 << 20))

	// Statik Dosyaları Gömülü Dosya Sisteminden Sun (CSS, JS)
	r.Handle("/static/*", http.FileServer(http.FS(web.Assets)))

	// ==========================================
	// 7. WEB ARAYÜZÜ (HTML) ROTALARI
	// ==========================================
	r.With(middleware.SessionAuthOptional(userService)).Get("/setup", uiHandler.SetupPage)
	r.With(middleware.SessionAuthOptional(userService)).Get("/login", uiHandler.LoginPage)
	r.With(middleware.SessionAuth(userService, false)).Get("/", uiHandler.DashboardPage)

	// ==========================================
	// 8. GENEL VE MİSAFİR API ROTALARI
	// ==========================================
	// Güvenlik: Setup ve Login uç noktalarına sıkı hız limiti (brute-force / setup race engeli)
	r.With(middleware.RateLimit(5, time.Minute)).Post("/api/v1/setup", authHandler.Setup)
	// Güvenlik: kimlik doğrulamasız durum sorgusu da DB'ye iner; sıkıştırma saldırılarına karşı limitli
	r.With(middleware.RateLimit(60, time.Minute)).Get("/api/v1/setup/status", authHandler.SetupStatus)
	r.With(middleware.RateLimit(10, time.Minute)).Post("/api/v1/login", authHandler.Login)
	r.Post("/api/v1/logout", authHandler.Logout)

	// ==========================================
	// 9. YETKİLİ API ROTALARI (API-Key VEYA Oturum Destekli)
	// ==========================================
	r.Group(func(r chi.Router) {
		// Hem API Key ile harici ajanlar hem de panelden JWT ile erişimi destekler
		r.Use(middleware.AuthEither(userService, true))
		// Kimlik doğrulanmış istemciler için daha geniş limit (link spam engeli)
		r.Use(middleware.RateLimit(100, time.Minute))

		// Link İşlemleri
		r.Post("/api/v1/links", linkHandler.Shorten)
		r.Get("/api/v1/links", linkHandler.GetLinks)
		r.Put("/api/v1/links/{short_code}", linkHandler.Update)
		r.Delete("/api/v1/links/{short_code}", linkHandler.DeleteLink)

		// Analitik Raporu
		r.Get("/api/v1/analytics/{short_code}", analyticsHandler.GetStats)

		// API Yardım Dokümantasyonu
		r.Get("/api/v1/help", linkHandler.APIHelp)

		// API Key Yenileme
		r.Post("/api/v1/users/refresh-token", authHandler.RegenerateAPIKey)

		// Profil Güncelleme
		r.Put("/api/v1/users/profile", authHandler.UpdateProfile)
	})

	// ==========================================
	// 10. SUPERADMIN YETKİLİ ADMİN ROTALARI
	// ==========================================
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthEither(userService, true))
		r.Use(middleware.RequireRole("superadmin"))

		// Üye Yönetimi
		r.Post("/api/v1/admin/users", authHandler.CreateUser)
		r.Get("/api/v1/admin/users", authHandler.GetUsers)
		r.Delete("/api/v1/admin/users/{id}", authHandler.DeleteUser)
	})

	// ==========================================
	// 11. YÖNLENDİRME ROTASI (HER ŞEYİN EN ALTINDA OLMALI)
	// ==========================================
	// Güvenlik: Yönlendirme rotası kimlik doğrulamasızdır ve her istekte DB yazması +
	// asenkron analitik goroutine'i tetikler. IP başına 300/dk limiti; meşru trafiği
	// (NAT arkası kurumsal tıklamalar dahil) etkilemeden click-fraud / yazma
	// amplifikasyonu (DoS) saldırılarını keser.
	r.With(middleware.RateLimit(300, time.Minute)).Get("/{short_code}", redirectHandler.Redirect)

	// 12. HTTP Sunucusunu Başlat ve Graceful Shutdown Yapılandır
	serverAddr := ":" + config.GlobalConfig.Port
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Arka planda sunucuyu başlat
	go func() {
		scheme := "http"
		if config.GlobalConfig.TLSCertPath != "" && config.GlobalConfig.TLSKeyPath != "" {
			scheme = "https"
			log.Printf("Linklik sunucusu başlatılıyor: https://localhost%s", serverAddr)
			if err := srv.ListenAndServeTLS(config.GlobalConfig.TLSCertPath, config.GlobalConfig.TLSKeyPath); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("Sunucu hatası: %v", err)
			}
			return
		}
		log.Printf("Linklik sunucusu başlatılıyor: %s://localhost%s", scheme, serverAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Sunucu hatası: %v", err)
		}
	}()

	// Kapatma sinyallerini dinle (SIGINT, SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("Sunucu kapatılıyor...")

	// 5 saniye içinde kapatma garantisi ver
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Sunucu zorla kapatıldı: %v", err)
	}

	log.Println("Sunucu güvenli bir şekilde kapatıldı.")
}
