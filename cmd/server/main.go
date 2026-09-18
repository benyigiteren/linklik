package main

import (
	"context"
	"errors"
	"flag"
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
	mcpMode := flag.Bool("mcp", false, "MCP stdio modunda çalıştır (Claude Desktop vb. için)")
	mcpAPIKey := flag.String("api-key", "", "MCP stdio modu için kullanılacak X-API-KEY")
	flag.Parse()

	// 1. Konfigürasyonu Yükle
	config.LoadConfig()

	// 2. Veritabanını Başlat (SQLite & WAL Modu & Otomatik Göçler)
	if err := db.InitDB(config.GlobalConfig.DBPath); err != nil {
		log.Fatalf("Veritabanı başlatma hatası: %v", err)
	}
	defer db.CloseDB()

	// 3. Repository Katmanlarını Oluştur
	userRepo := repository.NewUserRepository()
	linkRepo := repository.NewLinkRepository()
	analyticsRepo := repository.NewAnalyticsRepository()

	// 4. Servis Katmanlarını Oluştur (Worker Pool başlatılır)
	userService := service.NewUserService(userRepo)
	linkService := service.NewLinkService(linkRepo)
	analyticsService := service.NewAnalyticsService(analyticsRepo)

	// 5. Handler Katmanlarını Oluştur
	authHandler := handler.NewAuthHandler(userService)
	linkHandler := handler.NewLinkHandler(linkService)
	analyticsHandler := handler.NewAnalyticsHandler(analyticsService, linkService)
	redirectHandler := handler.NewRedirectHandler(linkService, analyticsService)
	uiHandler := handler.NewUIHandler(userService)
	qrHandler := handler.NewQRHandler(linkService)
	healthHandler := handler.NewHealthHandler(db.Db)
	mcpHandler := handler.NewMCPHandler(linkService, analyticsService, userService)
	openAPIHandler := handler.NewOpenAPIHandler()

	// Eğer --mcp parametresi verilmişse stdio üzerinden çalıştır
	if *mcpMode {
		key := *mcpAPIKey
		if key == "" {
			key = os.Getenv("LINKLIK_API_KEY")
		}
		user, err := userService.GetUserByAPIKey(key)
		if err != nil || user == nil {
			log.Fatalf("[MCP Stdio] Geçersiz veya eksik API anahtarı. Lütfen --api-key veya LINKLIK_API_KEY belirtin.")
		}
		log.Printf("[MCP Stdio] Kullanıcı doğrulandı: %s. Stdio dinleniyor...", user.Username)
		mcpHandler.RunStdio(user)
		return
	}

	// 6. Chi Router ve Middleware Yapılandırması
	r := chi.NewRouter()

	r.Use(chi_middleware.RequestID)
	// Güvenlik & Proxy: Docker, Cloudflare ve Nginx arkasında gerçek istemci IP tespiti
	r.Use(middleware.TrustedProxyMiddleware)
	r.Use(middleware.SecurityHeaders)
	r.Use(chi_middleware.Logger)
	r.Use(chi_middleware.Recoverer)
	r.Use(middleware.CORS)

	// Genel gövde boyutu kısıtı (1 MiB)
	r.Use(middleware.MaxBodySize(1 << 20))

	// Sistem Sağlık Uç Noktaları (Docker/K8s/Load Balancer için)
	r.Get("/healthz", healthHandler.Healthz)
	r.Get("/readyz", healthHandler.Readyz)

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
	r.With(middleware.RateLimit(5, time.Minute)).Post("/api/v1/setup", authHandler.Setup)
	r.With(middleware.RateLimit(60, time.Minute)).Get("/api/v1/setup/status", authHandler.SetupStatus)
	r.With(middleware.RateLimit(10, time.Minute)).Post("/api/v1/login", authHandler.Login)
	r.Post("/api/v1/logout", authHandler.Logout)

	// QR Kod Uç Noktası (Önbelleklenebilir ve genel/erişilebilir)
	r.Get("/api/v1/links/{short_code}/qr", qrHandler.GenerateQR)

	// OpenAPI 3.0 Şeması (ChatGPT Actions, Swagger ve AI Ajanları)
	r.Get("/openapi.json", openAPIHandler.HandleOpenAPI)
	r.Get("/api/v1/openapi.json", openAPIHandler.HandleOpenAPI)

	// ==========================================
	// 9. MODEL CONTEXT PROTOCOL (MCP) EVRENSEL ROTALARI
	// ==========================================
	r.Route("/mcp", func(r chi.Router) {
		r.Use(middleware.RateLimit(300, time.Minute))
		r.Options("/*", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-KEY")
			w.WriteHeader(http.StatusNoContent)
		})
		// Tek ve evrensel MCP uç noktası: GET -> SSE akışı, POST -> JSON-RPC (Streamable HTTP)
		r.Get("/", mcpHandler.HandleUnified)
		r.Post("/", mcpHandler.HandleMessage)
		// Standart alt rotalar (SSE & Message)
		r.Get("/sse", mcpHandler.HandleSSE)
		r.Post("/message", mcpHandler.HandleMessage)
	})
	// Kök dizin takma adları (bazı MCP istemcileri için)
	r.Get("/sse", mcpHandler.HandleSSE)
	r.Post("/message", mcpHandler.HandleMessage)

	// ==========================================
	// 10. YETKİLİ API ROTALARI (API-Key VEYA Oturum Destekli)
	// ==========================================
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthEither(userService, true))
		r.Use(middleware.RateLimit(100, time.Minute))

		// Link İşlemleri
		r.Post("/api/v1/links", linkHandler.Shorten)
		r.Get("/api/v1/links", linkHandler.GetLinks)
		r.Put("/api/v1/links/{short_code}", linkHandler.Update)
		r.Patch("/api/v1/links/{short_code}/toggle", linkHandler.ToggleActive)
		r.Post("/api/v1/links/{short_code}/reset-stats", linkHandler.ResetStats)
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
	// 11. SUPERADMIN YETKİLİ ADMİN ROTALARI
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
	// 12. ŞİFRE DOĞRULAMA VE YÖNLENDİRME ROTALARI
	// ==========================================
	r.With(middleware.RateLimit(30, time.Minute)).Post("/{short_code}/verify-password", redirectHandler.VerifyPassword)
	r.With(middleware.RateLimit(300, time.Minute)).Get("/{short_code}", redirectHandler.Redirect)

	// 13. HTTP Sunucusunu Başlat ve Graceful Shutdown Yapılandır
	serverAddr := ":" + config.GlobalConfig.Port
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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

	// Kapatma sinyallerini dinle
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("Sunucu kapatılıyor...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP sunucusu kapatma uyarısı: %v", err)
	}

	// Analitik iş kuyruğunu tüket ve güvenle kapat
	analyticsService.Shutdown(ctx)

	log.Println("Sunucu güvenli bir şekilde kapatıldı.")
}
