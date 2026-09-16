package handler

import (
	"fmt"
	"html"
	"linklik/internal/service"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
)

// RedirectHandler kısa link yönlendirmelerini yönetir.
type RedirectHandler struct {
	linkService      *service.LinkService
	analyticsService *service.AnalyticsService
}

// NewRedirectHandler yeni bir RedirectHandler örneği oluşturur.
func NewRedirectHandler(ls *service.LinkService, as *service.AnalyticsService) *RedirectHandler {
	return &RedirectHandler{
		linkService:      ls,
		analyticsService: as,
	}
}

// Redirect kısa kod üzerinden asıl URL'e yönlendirme yapar ve analitik yazar.
func (h *RedirectHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "short_code")
	if shortCode == "" || !isValidShortCode(shortCode) {
		http.Error(w, "Geçersiz İstek", http.StatusBadRequest)
		return
	}

	// Orijinal URL'i sorgula
	link, err := h.linkService.GetOriginalURL(shortCode)
	if err != nil {
		switch err.Error() {
		case "MAX_CLICKS_REACHED":
			renderStatusPage(w, http.StatusForbidden, "Limit Aşıldı", "⚠️", "Bu kısaltılmış bağlantı için belirlenen maksimum tıklanma / görüntülenme sınırına ulaşıldığı için yönlendirme yapılamıyor.")
			return
		case "LINK_INACTIVE":
			renderStatusPage(w, http.StatusForbidden, "Bağlantı Durduruldu", "⏸️", "Bu kısaltılmış bağlantı link sahibi tarafından geçici olarak erişime kapatılmıştır.")
			return
		case "LINK_EXPIRED":
			renderStatusPage(w, http.StatusGone, "Süresi Doldu", "⏳", "Bu kısaltılmış bağlantının geçerlilik süresi (son kullanma tarihi) dolmuştur.")
			return
		case "PASSWORD_REQUIRED":
			renderPasswordPromptPage(w, shortCode, "")
			return
		default:
			http.Error(w, "Sistem Hatası", http.StatusInternalServerError)
			return
		}
	}

	if link == nil {
		renderStatusPage(w, http.StatusNotFound, "Bağlantı Bulunamadı", "❓", "Aradığınız kısaltılmış bağlantı sistemimizde bulunamadı veya silinmiş olabilir.")
		return
	}

	// Asenkron analitik kaydı
	ip := r.RemoteAddr
	countryHeader := r.Header.Get("CF-IPCountry")
	if countryHeader == "" {
		countryHeader = r.Header.Get("X-Country-Code")
	}

	h.analyticsService.LogRedirectAsync(
		link.ID,
		ip,
		r.UserAgent(),
		r.Referer(),
		countryHeader,
	)

	// UTM ve Query Parametrelerini koruyarak yönlendir
	finalTargetURL := appendQueryParams(link.OriginalURL, r.URL.RawQuery)
	http.Redirect(w, r, finalTargetURL, http.StatusFound)
}

// VerifyPassword şifreli linklerin parolasını doğrular.
func (h *RedirectHandler) VerifyPassword(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "short_code")
	if shortCode == "" || !isValidShortCode(shortCode) {
		http.Error(w, "Geçersiz İstek", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Form ayrıştırma hatası", http.StatusBadRequest)
		return
	}

	password := r.FormValue("password")
	link, err := h.linkService.VerifyPasswordAndGetURL(shortCode, password)
	if err != nil {
		if err.Error() == "hatalı şifre" {
			renderPasswordPromptPage(w, shortCode, "Girdiğiniz şifre hatalı. Lütfen tekrar deneyin.")
			return
		}
		if err.Error() == "LINK_INACTIVE" {
			renderStatusPage(w, http.StatusForbidden, "Bağlantı Durduruldu", "⏸️", "Bu kısaltılmış bağlantı durdurulmuştur.")
			return
		}
		if err.Error() == "LINK_EXPIRED" {
			renderStatusPage(w, http.StatusGone, "Süresi Doldu", "⏳", "Bu kısaltılmış bağlantının geçerlilik süresi dolmuştur.")
			return
		}
		if err.Error() == "MAX_CLICKS_REACHED" {
			renderStatusPage(w, http.StatusForbidden, "Limit Aşıldı", "⚠️", "Maksimum tıklanma sınırına ulaşıldı.")
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Analitik kaydet
	ip := r.RemoteAddr
	countryHeader := r.Header.Get("CF-IPCountry")
	if countryHeader == "" {
		countryHeader = r.Header.Get("X-Country-Code")
	}
	h.analyticsService.LogRedirectAsync(link.ID, ip, r.UserAgent(), r.Referer(), countryHeader)

	finalTargetURL := appendQueryParams(link.OriginalURL, r.URL.RawQuery)
	http.Redirect(w, r, finalTargetURL, http.StatusFound)
}

// appendQueryParams orijinal hedef URL'e gelen istekteki UTM ve diğer arama parametrelerini ekler.
func appendQueryParams(originalURL, incomingQuery string) string {
	if incomingQuery == "" {
		return originalURL
	}

	parsed, err := url.Parse(originalURL)
	if err != nil {
		return originalURL
	}

	originalQ := parsed.Query()
	incomingQ, err := url.ParseQuery(incomingQuery)
	if err == nil {
		for k, v := range incomingQ {
			// Hedef URL'de yoksa gelen parametreyi ekle
			if !originalQ.Has(k) {
				for _, val := range v {
					originalQ.Add(k, val)
				}
			}
		}
		parsed.RawQuery = originalQ.Encode()
		return parsed.String()
	}

	if strings.Contains(originalURL, "?") {
		return originalURL + "&" + incomingQuery
	}
	return originalURL + "?" + incomingQuery
}

// renderPasswordPromptPage şifre giriş formunu gösterir.
func renderPasswordPromptPage(w http.ResponseWriter, shortCode, errorMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	errorHtml := ""
	if errorMsg != "" {
		errorHtml = fmt.Sprintf(`<div style="background-color: rgba(239, 68, 68, 0.1); border: 1px solid #ef4444; color: #ef4444; padding: 10px; border-radius: 8px; margin-bottom: 16px; font-size: 14px;">%s</div>`, html.EscapeString(errorMsg))
	}

	htmlContent := fmt.Sprintf(`
		<!DOCTYPE html>
		<html lang="tr">
		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0">
			<title>Şifre Korumalı Bağlantı - Linklik</title>
			<style>
				:root {
					--bg-primary: #f8fafc;
					--bg-secondary: #ffffff;
					--border-color: #e2e8f0;
					--text-primary: #0f172a;
					--text-secondary: #475569;
				}
				.dark-theme {
					--bg-primary: #09090b;
					--bg-secondary: #121214;
					--border-color: #242427;
					--text-primary: #f4f4f5;
					--text-secondary: #a1a1aa;
				}
				body {
					font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
					background-color: var(--bg-primary);
					color: var(--text-primary);
					padding: 40px 20px;
					margin: 0;
					display: flex;
					justify-content: center;
					align-items: center;
					min-height: 90vh;
				}
				.card {
					background-color: var(--bg-secondary);
					border: 1px solid var(--border-color);
					border-radius: 16px;
					padding: 40px 32px;
					max-width: 420px;
					width: 100%%;
					text-align: center;
					box-shadow: 0 10px 25px -5px rgba(0,0,0,0.05);
				}
				h1 { font-size: 22px; font-weight: 700; margin: 12px 0 8px 0; }
				p { color: var(--text-secondary); font-size: 14px; margin-bottom: 24px; line-height: 1.5; }
				input[type="password"] {
					width: 100%%;
					padding: 12px 14px;
					border-radius: 8px;
					border: 1px solid var(--border-color);
					background-color: transparent;
					color: var(--text-primary);
					font-size: 15px;
					box-sizing: border-box;
					margin-bottom: 16px;
					outline: none;
				}
				button {
					width: 100%%;
					padding: 12px;
					background-color: var(--text-primary);
					color: var(--bg-secondary);
					border: none;
					border-radius: 8px;
					font-weight: 600;
					font-size: 15px;
					cursor: pointer;
				}
				button:hover { opacity: 0.9; }
			</style>
		</head>
		<body>
			<div class="card">
				<img id="logo" src="/static/img/Linklik-siyah.png" alt="Linklik Logo" style="height: 50px; margin-bottom: 16px; object-fit: contain;">
				<div style="font-size: 38px;">🔒</div>
				<h1>Şifre Korumalı Bağlantı</h1>
				<p>Bu bağlantıya erişmek için link sahibinin belirlediği şifreyi girmeniz gerekmektedir.</p>
				%s
				<form method="POST" action="/%s/verify-password">
					<input type="password" name="password" placeholder="Erişim Şifresi" required autofocus>
					<button type="submit">Doğrula ve Yönlendir</button>
				</form>
			</div>
			<script>
				const theme = localStorage.getItem('theme') || (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
				if (theme === 'dark') {
					document.documentElement.className = 'dark-theme';
					document.getElementById('logo').src = '/static/img/Linklik-beyaz.png';
				}
			</script>
		</body>
		</html>
	`, errorHtml, shortCode)

	_, _ = w.Write([]byte(htmlContent))
}

// renderStatusPage durum mesajı (hata, limit, süre vb.) sayfasını oluşturur.
func renderStatusPage(w http.ResponseWriter, status int, title, icon, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	htmlContent := fmt.Sprintf(`
		<!DOCTYPE html>
		<html lang="tr">
		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0">
			<title>%s - Linklik</title>
			<style>
				:root {
					--bg-primary: #f8fafc;
					--bg-secondary: #ffffff;
					--border-color: #e2e8f0;
					--text-primary: #0f172a;
					--text-secondary: #475569;
				}
				.dark-theme {
					--bg-primary: #09090b;
					--bg-secondary: #121214;
					--border-color: #242427;
					--text-primary: #f4f4f5;
					--text-secondary: #a1a1aa;
				}
				body {
					font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
					background-color: var(--bg-primary);
					color: var(--text-primary);
					text-align: center;
					padding: 40px 20px;
					margin: 0;
					display: flex;
					justify-content: center;
					align-items: center;
					min-height: 90vh;
				}
				.card {
					background-color: var(--bg-secondary);
					border: 1px solid var(--border-color);
					border-radius: 16px;
					padding: 48px 32px;
					max-width: 480px;
					width: 100%%;
					box-shadow: 0 10px 25px -5px rgba(0,0,0,0.04);
				}
				.icon { font-size: 44px; margin-bottom: 16px; }
				h1 { font-size: 24px; margin-top: 0; margin-bottom: 12px; font-weight: 800; }
				p { color: var(--text-secondary); font-size: 15px; margin-bottom: 28px; line-height: 1.6; }
				.btn-home {
					display: inline-block;
					padding: 12px 24px;
					background-color: var(--text-primary);
					color: var(--bg-secondary);
					text-decoration: none;
					border-radius: 8px;
					font-weight: 600;
					font-size: 14px;
				}
			</style>
		</head>
		<body>
			<div class="card">
				<img id="logo" src="/static/img/Linklik-siyah.png" alt="Linklik Logo" style="height: 55px; margin-bottom: 20px; object-fit: contain;">
				<div class="icon">%s</div>
				<h1>%s</h1>
				<p>%s</p>
				<a href="/" class="btn-home">Ana Sayfaya Git</a>
			</div>
			<script>
				const theme = localStorage.getItem('theme') || (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
				if (theme === 'dark') {
					document.documentElement.className = 'dark-theme';
					document.getElementById('logo').src = '/static/img/Linklik-beyaz.png';
				}
			</script>
		</body>
		</html>
	`, html.EscapeString(title), icon, html.EscapeString(title), html.EscapeString(message))

	_, _ = w.Write([]byte(htmlContent))
}
