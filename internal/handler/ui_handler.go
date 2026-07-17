package handler

import (
	"html/template"
	"linklik/internal/config"
	"linklik/internal/middleware"
	"linklik/internal/service"
	"linklik/web"
	"log"
	"net/http"
)

// UIHandler web arayüz sayfalarını yönetir.
type UIHandler struct {
	userService *service.UserService
	templates   *template.Template
}

// NewUIHandler yeni bir UIHandler örneği oluşturur ve gömülü şablonları derler.
func NewUIHandler(us *service.UserService) *UIHandler {
	tmpl, err := template.ParseFS(web.Assets, "templates/*.html")
	if err != nil {
		log.Fatalf("HTML şablonları derlenemedi: %v", err)
	}

	return &UIHandler{
		userService: us,
		templates:   tmpl,
	}
}

// SetupPage ilk kurulum arayüzünü döner.
func (h *UIHandler) SetupPage(w http.ResponseWriter, r *http.Request) {
	required, err := h.userService.IsSetupRequired()
	if err != nil {
		http.Error(w, "Sistem hatası", http.StatusInternalServerError)
		return
	}

	// Eğer kurulum zaten yapılmışsa giriş sayfasına yönlendir
	if !required {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = h.templates.ExecuteTemplate(w, "setup.html", nil)
	if err != nil {
		log.Printf("setup.html render hatası: %v", err)
		http.Error(w, "Şablon hatası", http.StatusInternalServerError)
	}
}

// LoginPage kullanıcı giriş arayüzünü döner.
func (h *UIHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	// Eğer kurulum yapılmadıysa kurulum sayfasına yönlendir
	required, err := h.userService.IsSetupRequired()
	if err == nil && required {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}

	// Eğer kullanıcı zaten giriş yapmışsa ana sayfaya (dashboard) yönlendir
	user := middleware.GetUserFromContext(r.Context())
	if user != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = h.templates.ExecuteTemplate(w, "login.html", nil)
	if err != nil {
		log.Printf("login.html render hatası: %v", err)
		http.Error(w, "Şablon hatası", http.StatusInternalServerError)
	}
}

// DashboardPage kullanıcı yönetim panelini (ana sayfa) döner.
func (h *UIHandler) DashboardPage(w http.ResponseWriter, r *http.Request) {
	// Eğer kurulum yapılmadıysa kuruluma yönlendir
	required, err := h.userService.IsSetupRequired()
	if err == nil && required {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}

	// Oturum kontrolü
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]interface{}{
		"User":    user,
		"BaseURL": config.GlobalConfig.BaseURL,
	}

	err = h.templates.ExecuteTemplate(w, "dashboard.html", data)
	if err != nil {
		log.Printf("dashboard.html render hatası: %v", err)
		http.Error(w, "Şablon hatası", http.StatusInternalServerError)
	}
}
