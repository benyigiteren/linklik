package handler

import (
	"encoding/json"
	"fmt"
	"linklik/internal/config"
	"linklik/internal/middleware"
	"linklik/internal/model"
	"linklik/internal/service"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// LinkHandler link işlemlerini yöneten handler'dır.
type LinkHandler struct {
	linkService *service.LinkService
}

// NewLinkHandler yeni bir LinkHandler örneği oluşturur.
func NewLinkHandler(ls *service.LinkService) *LinkHandler {
	return &LinkHandler{linkService: ls}
}

// Shorten yeni bir URL kısaltır.
func (h *LinkHandler) Shorten(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Oturum açmanız veya geçerli bir API anahtarı kullanmanız gerekiyor")
		return
	}

	var req model.ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Geçersiz JSON verisi")
		return
	}

	link, err := h.linkService.ShortenURL(req.URL, req.CustomAlias, req.MaxClicks, req.ExpiresAt, req.Password, req.IsActive, user.ID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	shortURL := fmt.Sprintf("%s/%s", config.GlobalConfig.BaseURL, link.ShortCode)

	respondCreated(w, map[string]interface{}{
		"link":      link,
		"short_url": shortURL,
	})
}

// GetLinks kullanıcının linklerini sayfalama, arama ve filtreleme ile döner.
func (h *LinkHandler) GetLinks(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")

	isSuperadmin := user.Role == model.RoleSuperadmin
	links, totalCount, err := h.linkService.GetPaginatedLinks(user.ID, isSuperadmin, page, limit, search, status)
	if err != nil {
		log.Printf("[link_handler] link listesi alınamadı (user_id=%d): %v", user.ID, err)
		respondError(w, http.StatusInternalServerError, "Linkler alınamadı, lütfen tekrar deneyin")
		return
	}

	type linkResponse struct {
		*model.Link
		ShortURL string `json:"short_url"`
		QRURL    string `json:"qr_url"`
	}

	respList := make([]linkResponse, len(links))
	for i, l := range links {
		respList[i] = linkResponse{
			Link:     l,
			ShortURL: fmt.Sprintf("%s/%s", config.GlobalConfig.BaseURL, l.ShortCode),
			QRURL:    fmt.Sprintf("%s/api/v1/links/%s/qr", config.GlobalConfig.BaseURL, l.ShortCode),
		}
	}

	totalPages := int((totalCount + int64(limit) - 1) / int64(limit))
	if totalPages == 0 {
		totalPages = 1
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(model.PaginatedResponse{
		Success:    true,
		Data:       respList,
		TotalCount: totalCount,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	})
}

// Update var olan bir kısaltılmış linki günceller.
func (h *LinkHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	var req model.LinkUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Geçersiz JSON verisi")
		return
	}

	link, err := h.linkService.UpdateLink(shortCode, req.URL, req.CustomAlias, req.MaxClicks, req.ExpiresAt, req.Password, req.IsActive, user.ID, user.Role)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	shortURL := fmt.Sprintf("%s/%s", config.GlobalConfig.BaseURL, link.ShortCode)

	respondSuccess(w, map[string]interface{}{
		"link":      link,
		"short_url": shortURL,
		"message":   "Link başarıyla güncellendi.",
	})
}

// ToggleActive linkin aktif/pasif durumunu değiştirir.
func (h *LinkHandler) ToggleActive(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}

	shortCode := chi.URLParam(r, "short_code")
	if shortCode == "" || !isValidShortCode(shortCode) {
		respondError(w, http.StatusBadRequest, "Geçersiz short_code parametresi")
		return
	}

	newStatus, err := h.linkService.ToggleLinkActive(shortCode, user.ID, user.Role)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	msg := "Link pasife alındı"
	if newStatus {
		msg = "Link aktifleştirildi"
	}

	respondSuccess(w, map[string]interface{}{
		"is_active": newStatus,
		"message":   msg,
	})
}

// ResetStats linkin tıklama sayısını ve analitik kayıtlarını sıfırlar.
func (h *LinkHandler) ResetStats(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}

	shortCode := chi.URLParam(r, "short_code")
	if shortCode == "" || !isValidShortCode(shortCode) {
		respondError(w, http.StatusBadRequest, "Geçersiz short_code parametresi")
		return
	}

	err := h.linkService.ResetLinkStats(shortCode, user.ID, user.Role)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondSuccess(w, map[string]string{
		"message": "Link istatistikleri ve tıklama verileri başarıyla sıfırlandı.",
	})
}

// DeleteLink kısa koda göre linki siler.
func (h *LinkHandler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}

	shortCode := chi.URLParam(r, "short_code")
	if shortCode == "" || !isValidShortCode(shortCode) {
		respondError(w, http.StatusBadRequest, "Geçersiz short_code parametresi")
		return
	}

	err := h.linkService.DeleteLinkByShortCode(shortCode, user.ID, user.Role)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondSuccess(w, map[string]string{"message": "Kısaltılmış link başarıyla silindi."})
}

// APIHelp API dokümantasyonu ve yardım bilgilerini JSON döndürür.
func (h *LinkHandler) APIHelp(w http.ResponseWriter, r *http.Request) {
	helpData := map[string]interface{}{
		"servis_adi": "Linklik API Servisi",
		"endpoints": []map[string]interface{}{
			{
				"path":        "/api/v1/links",
				"method":      "POST",
				"description": "Yeni bir link kısaltır.",
				"request_body": map[string]string{
					"url":          "Kısaltılacak orijinal internet adresi (Zorunlu)",
					"custom_alias": "Özel kısa kod/takma ad (İsteğe bağlı)",
					"max_clicks":   "Maksimum tıklanma sınırı (İsteğe bağlı, 0 = sınırsız)",
					"expires_at":   "Son kullanma tarihi ISO-8601 (İsteğe bağlı, örn: 2026-12-31T23:59:00Z)",
					"password":     "Erişim şifresi (İsteğe bağlı)",
					"is_active":    "Aktiflik durumu (İsteğe bağlı, varsayılan: true)",
				},
			},
			{
				"path":        "/api/v1/links",
				"method":      "GET",
				"description": "Linkleri sayfalama ve filtreleme ile listeler (?page=1&limit=20&search=...&status=active|inactive|expired).",
			},
			{
				"path":        "/api/v1/links/{short_code}",
				"method":      "PUT",
				"description": "Var olan bir linki düzenler.",
			},
			{
				"path":        "/api/v1/links/{short_code}/toggle",
				"method":      "PATCH",
				"description": "Linkin aktiflik durumunu (aktif/pasif) tersine çevirir.",
			},
			{
				"path":        "/api/v1/links/{short_code}/reset-stats",
				"method":      "POST",
				"description": "Linkin tıklama sayısını ve tüm analitik geçmişini sıfırlar.",
			},
			{
				"path":        "/api/v1/links/{short_code}/qr",
				"method":      "GET",
				"description": "Link için dinamik PNG QR kod görseli üretir (?size=256).",
			},
			{
				"path":        "/api/v1/links/{short_code}",
				"method":      "DELETE",
				"description": "Kısaltılmış linki siler.",
			},
			{
				"path":        "/api/v1/analytics/{short_code}",
				"method":      "GET",
				"description": "Linkin tıklanma, coğrafi dağılım, tarayıcı ve cihaz detaylarını getirir.",
			},
			{
				"path":        "/mcp/sse",
				"method":      "GET",
				"description": "AI Ajanları için Model Context Protocol (MCP) Server-Sent Events akışı.",
			},
		},
	}
	respondSuccess(w, helpData)
}
