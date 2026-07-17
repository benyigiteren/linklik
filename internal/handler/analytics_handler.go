package handler

import (
	"linklik/internal/middleware"
	"linklik/internal/model"
	"linklik/internal/service"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// AnalyticsHandler analitik raporlama isteklerini yöneten handler'dır.
type AnalyticsHandler struct {
	analyticsService *service.AnalyticsService
	linkService      *service.LinkService
}

// NewAnalyticsHandler yeni bir AnalyticsHandler örneği oluşturur.
func NewAnalyticsHandler(analyticsService *service.AnalyticsService, linkService *service.LinkService) *AnalyticsHandler {
	return &AnalyticsHandler{
		analyticsService: analyticsService,
		linkService:      linkService,
	}
}

// GetStats belirli bir kısa koda (veya takma ada) ait detaylı analitik istatistiklerini döner.
func (h *AnalyticsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Oturum açmanız veya geçerli bir API anahtarı kullanmanız gerekiyor")
		return
	}

	shortCode := chi.URLParam(r, "short_code")
	if shortCode == "" || !isValidShortCode(shortCode) {
		respondError(w, http.StatusBadRequest, "Geçersiz short_code parametresi")
		return
	}

	// Linkin veritabanı kaydını bul (tıklamayı artırmadan)
	link, err := h.linkService.GetLinkByShortCode(shortCode)
	if err != nil {
		// Güvenlik: iç hata detaylarını yanıtta ifşa etme; logla.
		log.Printf("[analytics_handler] link sorgulama hatası (%s): %v", shortCode, err)
		respondError(w, http.StatusInternalServerError, "Link sorgulanamadı, lütfen tekrar deneyin")
		return
	}

	if link == nil {
		respondError(w, http.StatusNotFound, "Kısaltılmış link bulunamadı")
		return
	}

	// Yetki Kontrolü: Superadmin değilse ve linkin sahibi değilse erişimi engelle
	if user.Role != model.RoleSuperadmin && link.CreatedByID != user.ID {
		respondError(w, http.StatusForbidden, "Bu linkin analitik verilerini görüntüleme yetkiniz bulunmamaktadır")
		return
	}

	// Analitik istatistikleri çek
	stats, err := h.analyticsService.GetStats(link.ID)
	if err != nil {
		log.Printf("[analytics_handler] analitik okuma hatası (link_id=%d): %v", link.ID, err)
		respondError(w, http.StatusInternalServerError, "Analitik verileri alınamadı, lütfen tekrar deneyin")
		return
	}

	respondSuccess(w, stats)
}
