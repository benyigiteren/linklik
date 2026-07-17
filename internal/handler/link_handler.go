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

	link, err := h.linkService.ShortenURL(req.URL, req.CustomAlias, req.MaxClicks, user.ID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Kısaltılmış linkin tam URL'ini oluştur
	shortURL := fmt.Sprintf("%s/%s", config.GlobalConfig.BaseURL, link.ShortCode)

	respondCreated(w, map[string]interface{}{
		"link":      link,
		"short_url": shortURL,
	})
}

// GetLinks kullanıcının linklerini (veya Superadmin ise tüm linkleri) döner.
func (h *LinkHandler) GetLinks(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}

	var links []*model.Link
	var err error

	if user.Role == model.RoleSuperadmin {
		// Yönetici tüm linkleri görebilir
		links, err = h.linkService.GetAllLinks()
	} else {
		// Düz üye sadece kendi linklerini görebilir
		links, err = h.linkService.GetLinksByUserID(user.ID)
	}

	if err != nil {
		// Güvenlik: iç hata detaylarını ifşa etme; logla.
		log.Printf("[link_handler] link listesi alınamadı (user_id=%d): %v", user.ID, err)
		respondError(w, http.StatusInternalServerError, "Linkler alınamadı, lütfen tekrar deneyin")
		return
	}

	// Listeyi dönerken her linke kısa URL alanını ekleyerek dönelim
	type linkResponse struct {
		*model.Link
		ShortURL string `json:"short_url"`
	}

	respList := make([]linkResponse, len(links))
	for i, l := range links {
		respList[i] = linkResponse{
			Link:     l,
			ShortURL: fmt.Sprintf("%s/%s", config.GlobalConfig.BaseURL, l.ShortCode),
		}
	}

	respondSuccess(w, respList)
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

	link, err := h.linkService.UpdateLink(shortCode, req.URL, req.CustomAlias, req.MaxClicks, user.ID, user.Role)
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
				},
			},
			{
				"path":        "/api/v1/links/{short_code}",
				"method":      "PUT",
				"description": "Var olan bir linki düzenler (URL, takma ad ve limit).",
				"request_body": map[string]string{
					"url":          "Yeni orijinal internet adresi (Zorunlu)",
					"custom_alias": "Yeni özel kısa kod/takma ad (İsteğe bağlı)",
					"max_clicks":   "Yeni maksimum tıklanma sınırı (Zorunlu, 0 = sınırsız)",
				},
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
				"path":        "/api/v1/users/refresh-token",
				"method":      "POST",
				"description": "Mevcut X-API-KEY değerinizi iptal edip yeni bir anahtar üretir.",
			},
		},
	}
	respondSuccess(w, helpData)
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
