package handler

import (
	"encoding/json"
	"linklik/internal/config"
	"linklik/internal/middleware"
	"linklik/internal/model"
	"linklik/internal/service"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// AuthHandler yetkilendirme ve kullanıcı yönetimi isteklerini karşılar.
type AuthHandler struct {
	userService *service.UserService
}

// NewAuthHandler yeni bir AuthHandler örneği oluşturur.
func NewAuthHandler(us *service.UserService) *AuthHandler {
	return &AuthHandler{userService: us}
}

// SetupStatus ilk kurulum durumunu döner.
func (h *AuthHandler) SetupStatus(w http.ResponseWriter, r *http.Request) {
	required, err := h.userService.IsSetupRequired()
	if err != nil {
		log.Printf("[auth_handler] setup durumu kontrol hatası: %v", err)
		respondError(w, http.StatusInternalServerError, "Kurulum durumu kontrol edilemedi")
		return
	}
	respondSuccess(w, map[string]bool{"setup_required": required})
}

// Setup ilk kullanıcıyı (Superadmin) kaydeder.
func (h *AuthHandler) Setup(w http.ResponseWriter, r *http.Request) {
	var req model.SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Geçersiz JSON verisi")
		return
	}

	if req.Username == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "Kullanıcı adı ve şifre boş bırakılamaz")
		return
	}

	// Hızlı boş/doluluk kontrolü; detaylı biçim ve şifre politikası servis katmanında yapılır.
	user, err := h.userService.SetupFirstUser(req.Username, req.Password)
	if err != nil {
		respondError(w, http.StatusForbidden, err.Error())
		return
	}

	// Otomatik olarak JWT token üret ve cookie olarak ata (Kurulum sonrası giriş kolaylığı için)
	token, _, err := h.userService.Authenticate(req.Username, req.Password)
	if err == nil {
		setAuthCookie(w, token)
	}

	// Güvenlik: API anahtarı yanıtta gizlenir; dashboard'da "API Bağlantısı" sekmesinden alınır.
	user.APIKey = ""
	respondCreated(w, map[string]interface{}{
		"message": "İlk kurulum başarıyla tamamlandı. Artık Superadmin olarak giriş yaptınız.",
		"user":    user,
	})
}

// Login kullanıcı girişi yapar ve JWT cookie ayarlar.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Geçersiz JSON verisi")
		return
	}

	token, user, err := h.userService.Authenticate(req.Username, req.Password)
	if err != nil {
		respondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	setAuthCookie(w, token)

	// Güvenlik: token zaten HttpOnly çerez olarak ayarlandı; API anahtarı dashboard
	// uç noktasından alınmalıdır, login yanıtında sızıntı yüzeyini büyütmemek için
	// bu alanlar yanıta eklenmez.
	user.APIKey = ""
	respondSuccess(w, map[string]interface{}{
		"message": "Giriş başarılı",
		"user":    user,
	})
}

// Logout kullanıcının oturum çerezini temizler.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	clearAuthCookie(w)
	respondSuccess(w, map[string]string{"message": "Oturum başarıyla kapatıldı"})
}

// CreateUser yeni bir düz üye oluşturur (Yalnızca Superadmin).
func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req model.UserCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Geçersiz JSON verisi")
		return
	}

	if req.Username == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "Kullanıcı adı ve şifre boş bırakılamaz")
		return
	}

	// Detaylı biçim/şifre politikası servis katmanında; burada yalnızca ön boşluk kontrolü.
	user, err := h.userService.CreateUser(req.Username, req.Password)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// API anahtarı admin tarafına değil, kullanıcıya gösterilmek üzere gizlenir.
	user.APIKey = ""
	respondCreated(w, map[string]interface{}{
		"message": "Kullanıcı başarıyla oluşturuldu.",
		"user":    user,
	})
}

// GetUsers sistemdeki tüm kullanıcıları listeler (Yalnızca Superadmin).
func (h *AuthHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.userService.GetAllUsers()
	if err != nil {
		log.Printf("[auth_handler] kullanıcı listesi hatası: %v", err)
		respondError(w, http.StatusInternalServerError, "Kullanıcı listesi alınamadı, lütfen tekrar deneyin")
		return
	}
	respondSuccess(w, users)
}

// RegenerateAPIKey oturum açmış kullanıcının API anahtarını yeniler.
func (h *AuthHandler) RegenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}

	newKey, err := h.userService.RegenerateAPIKey(user.ID)
	if err != nil {
		log.Printf("[auth_handler] api key yenileme hatası (user_id=%d): %v", user.ID, err)
		respondError(w, http.StatusInternalServerError, "API anahtarı yenilenemedi, lütfen tekrar deneyin")
		return
	}

	respondSuccess(w, map[string]string{
		"api_key": newKey,
		"message": "API anahtarı başarıyla yenilendi. Lütfen yeni anahtarı güvenli bir yere kaydedin.",
	})
}

// setAuthCookie HTTP-only JWT token çerezini ayarlar. Secure bayrağı yapılandırma
// tarafından üretilir; canlı (https) dağıtımlarda true olur.
func setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Secure:   config.GlobalConfig.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearAuthCookie JWT token çerezini siler.
func clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   config.GlobalConfig.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// DeleteUser sistemden bir kullanıcıyı siler (Yalnızca Superadmin).
func (h *AuthHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	requester := middleware.GetUserFromContext(r.Context())
	if requester == nil {
		respondError(w, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}

	userIDStr := chi.URLParam(r, "id")
	if userIDStr == "" {
		respondError(w, http.StatusBadRequest, "Eksik kullanıcı ID'si")
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Geçersiz kullanıcı ID'si")
		return
	}

	err = h.userService.DeleteUser(userID, requester.ID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondSuccess(w, map[string]string{"message": "Kullanıcı başarıyla silindi."})
}

// ProfileUpdateRequest profil güncelleme isteği yapısıdır.
type ProfileUpdateRequest struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
}

// UpdateProfile kullanıcının kendi profil bilgilerini (kullanıcı adı ve şifre) günceller.
func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}

	var req ProfileUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Geçersiz JSON verisi")
		return
	}

	if req.Username == "" {
		respondError(w, http.StatusBadRequest, "Kullanıcı adı boş bırakılamaz")
		return
	}

	if req.Password != "" && len(req.Password) < 8 {
		respondError(w, http.StatusBadRequest, "Şifre en az 8 karakter olmalıdır")
		return
	}

	updatedUser, newToken, err := h.userService.UpdateProfile(user.ID, req.Username, req.Password)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Oturum çerezini güncelle
	setAuthCookie(w, newToken)

	updatedUser.APIKey = ""
	respondSuccess(w, map[string]interface{}{
		"message": "Profiliniz başarıyla güncellendi.",
		"user":    updatedUser,
	})
}
