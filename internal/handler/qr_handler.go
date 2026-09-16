package handler

import (
	"fmt"
	"linklik/internal/config"
	"linklik/internal/service"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/skip2/go-qrcode"
)

// QRHandler kısaltılmış linkler için dinamik QR kod üreten handler'dır.
type QRHandler struct {
	linkService *service.LinkService
}

// NewQRHandler yeni bir QRHandler oluşturur.
func NewQRHandler(ls *service.LinkService) *QRHandler {
	return &QRHandler{linkService: ls}
}

// GenerateQR kısaltılmış link için PNG QR kod üretir ve döner.
func (h *QRHandler) GenerateQR(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "short_code")
	if shortCode == "" || !isValidShortCode(shortCode) {
		http.Error(w, "Geçersiz kısa kod", http.StatusBadRequest)
		return
	}

	link, err := h.linkService.GetLinkByShortCode(shortCode)
	if err != nil || link == nil {
		http.Error(w, "Link bulunamadı", http.StatusNotFound)
		return
	}

	size := 256
	if s := r.URL.Query().Get("size"); s != "" {
		if parsed, err := strconv.Atoi(s); err == nil && parsed >= 64 && parsed <= 1024 {
			size = parsed
		}
	}

	// Tam kısa link URL'i
	fullURL := fmt.Sprintf("%s/%s", config.GlobalConfig.BaseURL, link.ShortCode)

	pngData, err := qrcode.Encode(fullURL, qrcode.Medium, size)
	if err != nil {
		http.Error(w, "QR kod üretilemedi", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400") // 24 saat önbellek
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pngData)
}
