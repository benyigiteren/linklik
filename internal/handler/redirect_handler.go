package handler

import (
	"linklik/internal/service"
	"net/http"

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
		if err.Error() == "MAX_CLICKS_REACHED" {
			// Limit Aşım Uyarısı
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`
				<!DOCTYPE html>
				<html lang="tr">
				<head>
					<meta charset="UTF-8">
					<meta name="viewport" content="width=device-width, initial-scale=1.0">
					<title>Limit Aşıldı - Linklik</title>
					<style>
						:root {
							--bg-primary: #f8fafc;
							--bg-secondary: #ffffff;
							--border-color: #e2e8f0;
							--text-primary: #0f172a;
							--text-secondary: #475569;
							--accent-danger: #dc2626;
						}
						.dark-theme {
							--bg-primary: #09090b;
							--bg-secondary: #121214;
							--border-color: #242427;
							--text-primary: #f4f4f5;
							--text-secondary: #a1a1aa;
							--accent-danger: #ef4444;
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
							transition: background-color 0.2s, color 0.2s;
						}
						.card {
							background-color: var(--bg-secondary);
							border: 1px solid var(--border-color);
							border-radius: 16px;
							padding: 48px 32px;
							max-width: 480px;
							width: 100%;
							box-shadow: 0 10px 25px -5px rgba(0,0,0,0.02), 0 8px 10px -6px rgba(0,0,0,0.01);
						}
						.warning-icon {
							font-size: 44px;
							margin-bottom: 16px;
						}
						h1 {
							font-size: 26px;
							margin-top: 0;
							margin-bottom: 12px;
							font-weight: 800;
							letter-spacing: -0.5px;
						}
						p {
							color: var(--text-secondary);
							font-size: 15px;
							margin-bottom: 28px;
							line-height: 1.6;
						}
						.btn-home {
							display: inline-block;
							padding: 12px 24px;
							background-color: var(--text-primary);
							color: var(--bg-secondary);
							text-decoration: none;
							border-radius: 8px;
							font-weight: 600;
							font-size: 14px;
							transition: opacity 0.2s;
						}
						.btn-home:hover {
							opacity: 0.9;
						}
					</style>
				</head>
				<body>
					<div class="card">
						<img id="logo" src="/static/img/Linklik-siyah.png" alt="Linklik Logo" style="height: 60px; margin-bottom: 24px; object-fit: contain;">
						<div class="warning-icon">⚠️</div>
						<h1>Limit Aşıldı</h1>
						<p>Bu kısaltılmış bağlantı için belirlenen maksimum tıklanma / görüntülenme sınırına ulaşıldığı için yönlendirme yapılamıyor.</p>
						<a href="/" class="btn-home">Yönetim Paneline Git</a>
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
			`))
			return
		}
		http.Error(w, "Sistem Hatası", http.StatusInternalServerError)
		return
	}

	if link == nil {
		// Eğer bulunamazsa 404 döndür
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`
			<!DOCTYPE html>
			<html lang="tr">
			<head>
				<meta charset="UTF-8">
				<meta name="viewport" content="width=device-width, initial-scale=1.0">
				<title>Link Bulunamadı - Linklik</title>
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
						transition: background-color 0.2s, color 0.2s;
					}
					.card {
						background-color: var(--bg-secondary);
						border: 1px solid var(--border-color);
						border-radius: 16px;
						padding: 48px 32px;
						max-width: 480px;
						width: 100%;
						box-shadow: 0 10px 25px -5px rgba(0,0,0,0.02), 0 8px 10px -6px rgba(0,0,0,0.01);
					}
					.warning-icon {
						font-size: 44px;
						margin-bottom: 16px;
					}
					h1 {
						font-size: 26px;
						margin-top: 0;
						margin-bottom: 12px;
						font-weight: 800;
						letter-spacing: -0.5px;
					}
					p {
						color: var(--text-secondary);
						font-size: 15px;
						margin-bottom: 28px;
						line-height: 1.6;
					}
					.btn-home {
						display: inline-block;
						padding: 12px 24px;
						background-color: var(--text-primary);
						color: var(--bg-secondary);
						text-decoration: none;
						border-radius: 8px;
						font-weight: 600;
						font-size: 14px;
						transition: opacity 0.2s;
					}
					.btn-home:hover {
						opacity: 0.9;
					}
				</style>
			</head>
			<body>
				<div class="card">
					<img id="logo" src="/static/img/Linklik-siyah.png" alt="Linklik Logo" style="height: 60px; margin-bottom: 24px; object-fit: contain;">
					<div class="warning-icon">❓</div>
					<h1>404 - Bağlantı Bulunamadı</h1>
					<p>Aradığınız kısaltılmış bağlantı sistemimizde bulunamadı veya süresi dolmuş olabilir.</p>
					<a href="/" class="btn-home">Yönetim Paneline Git</a>
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
		`))
		return
	}

	// IP Adresini belirle.
	// Güvenlik: chi_middleware.RealIP güvenilir proxy arkasında zaten RemoteAddr'ı doldurur;
	// burada doğrudan X-Forwarded-For okumak istemci tarafından sahtelenebilir.
	ip := r.RemoteAddr

	// Cloudflare ülke başlığı veya diğer başlıklar
	countryHeader := r.Header.Get("CF-IPCountry")
	if countryHeader == "" {
		countryHeader = r.Header.Get("X-Country-Code")
	}

	// Asenkron olarak analitik detaylarını kaydet
	h.analyticsService.LogRedirectAsync(
		link.ID,
		ip,
		r.UserAgent(),
		r.Referer(),
		countryHeader,
	)

	// Orijinal URL'e yönlendir (302 Found)
	http.Redirect(w, r, link.OriginalURL, http.StatusFound)
}
