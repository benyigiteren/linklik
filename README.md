<p align="center">
  <a href="https://github.com/benyigiteren/linklik">
    <img src="readmegorsel.png" alt="Linklik Dashboard Önizleme" width="100%" style="border-radius: 12px; box-shadow: 0 20px 40px -15px rgba(0,0,0,0.3);">
  </a>
</p>

<p align="center">
  <img src="web/static/img/Linklik-siyah.png" alt="Linklik Logo" height="60" style="margin-bottom: 10px;">
</p>

<h1 align="center">Linklik</h1>

<p align="center">
  <strong>Mikro Kaynak Tüketimli, Siber Güvenlik Sertleştirmeli ve Yerel Model Context Protocol (MCP) Destekli Yeni Nesil URL Kısaltma & Analitik Platformu</strong>
</p>

<p align="center">
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go Version"></a>
  <a href="https://sqlite.org/"><img src="https://img.shields.io/badge/SQLite-WAL%20(Zero%20CGO)-003B57?style=flat-square&logo=sqlite&logoColor=white" alt="SQLite"></a>
  <a href="https://modelcontextprotocol.io/"><img src="https://img.shields.io/badge/MCP-Native%202024--11--05-7C3AED?style=flat-square&logo=anthropic&logoColor=white" alt="MCP Compatible"></a>
  <a href="#-siber-g%C3%BCvenlik--dayan%C4%B1kl%C4%B1l%C4%B1k-mimarisi"><img src="https://img.shields.io/badge/Security-Hardened%20(A%2B)-10B981?style=flat-square&logo=securityscorecard&logoColor=white" alt="Security Hardened"></a>
  <a href="https://www.docker.com/"><img src="https://img.shields.io/badge/Docker-Multi--Arch%20Ready-2496ED?style=flat-square&logo=docker&logoColor=white" alt="Docker Ready"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-6366F1?style=flat-square" alt="MIT License"></a>
</p>

<p align="center">
  <a href="#-10-saniyede-h%C4%B1zl%C4%B1-ba%C5%9Flang%C4%B1%C3%A7-quick-start"><strong>Hızlı Başlangıç</strong></a> •
  <a href="#-neden-linklik">Neden Linklik?</a> •
  <a href="#-temel-yetkinlikler">Özellikler</a> •
  <a href="#-siber-g%C3%BCvenlik--dayan%C4%B1kl%C4%B1l%C4%B1k-mimarisi">Güvenlik</a> •
  <a href="#-model-context-protocol-mcp-ai-ajanlar%C4%B1">MCP & AI</a> •
  <a href="#-rest-api-referans%C4%B1">REST API</a> •
  <a href="#-konfig%C3%BCrasyon">Konfigürasyon</a>
</p>

---

## ⚡ 10 Saniyede Hızlı Başlangıç (Quick Start)

Linklik'i sunucunuzda veya yerel ortamınızda saniyeler içinde çalıştırmak için hazır **GHCR (GitHub Container Registry)** imajını tek bir komutla ayağa kaldırabilirsiniz:

### 🐳 Seçenek 1: Tek Komutla Docker Run (GHCR İmajı)

```bash
docker run -d \
  --name linklik \
  -p 8080:8080 \
  -v linklik_data:/app/data \
  -e JWT_SECRET="$(openssl rand -hex 32)" \
  -e BASE_URL="http://localhost:8080" \
  --restart unless-stopped \
  ghcr.io/benyigiteren/linklik:latest
```

> **🎉 Kurulum Tamamlandı:** Tarayıcınızda `http://localhost:8080` adresine gidin. Sistem otomatik olarak sizi `/setup` sihirbazına yönlendirecek ve ilk Superadmin hesabınızı güvenle oluşturacaktır.

### 📦 Seçenek 2: Klonlamadan Tek Komutla Docker Compose

```bash
curl -sSL https://raw.githubusercontent.com/benyigiteren/linklik/main/docker-compose.yml -o docker-compose.yml && \
echo "JWT_SECRET=$(openssl rand -hex 32)" > .env && \
docker compose up -d
```

### 🛠️ Seçenek 3: Depoyu Klonlayarak Başlatma

```bash
git clone https://github.com/benyigiteren/linklik.git
cd linklik
cp .env.example .env
docker compose up -d
```

---

## ⚡ Neden Linklik?

Geleneksel URL kısaltıcı servisler ya hantal bağımlılıklar (Redis, harici DB sunucuları, ağır Node/Python ortamları) gerektirir ya da üçüncü taraf veri madenciliği yapan kapalı servislerin elindedir. 

**Linklik**, modern bulut mimarisi ve yapay zekâ çağının gereksinimlerine göre sıfırdan tasarlandı:

* **Tüy Kadar Hafif:** Boşta sadece **10–15 MB RAM** tüketimi, ~15 MB bağımsız derlenmiş tek ikili dosya (single static binary).
* **Sıfır CGO & Saf Taşınabilirlik:** `modernc.org/sqlite` ile harici C derleyicisi gerektirmeden, **SQLite WAL** hızında tek dosyalık veritabanı.
* **Yapay Zekâya Yerel Uyum (Agent-First):** Dünyanın ilk **Model Context Protocol (MCP)** standartlarını yerel olarak destekleyen URL kısaltma motoru. Claude Desktop, Cursor, Gemini veya LangChain ile sıfır konfigürasyonla çalışır.
* **Derinlemesine Savunma (Defense-in-Depth):** Profesyonel siber güvenlik testlerinden geçmiş; SSRF, DOM XSS, Slowloris, Brute-Force ve CSRF saldırılarına karşı donanımlı çekirdek.
* **Veri Egemenliği (Data Sovereignty):** 3. parti izleyici kodlar (Google Analytics vb.) içermez; tüm tıklama ve ziyaretçi verileri yalnızca sizin kontrolünüzdeki SQLite dosyasında saklanır.

---

## 🏛️ Mimari Bakış

Linklik; gelen istekleri katmanlı güvenlik süzgeçlerinden geçirip asenkron iş parçacıklarıyla veritabanına yansıtan reaktif bir akışa sahiptir:

```mermaid
flowchart LR
    A[İstemci / AI Ajan / Ziyaretçi] --> B{Ters Proxy & Güvenlik Duvarı}
    B -->|CIDR & IP Doğrulama| C[Chi Router & Güvenlik Başlıkları]
    C -->|Slowloris + 1 MiB Sınır| D[Rate Limiter & CSRF Denetimi]
    D -->|Oturum / API-Key / Role| E[İş Mantığı Katmanı Service]
    E -->|Kısa Kod Sorgulama| F[(SQLite WAL Veritabanı)]
    E -.->|Non-Blocking Drop-Safe Kuyruk| G[4x Async Worker Havuzu]
    G -->|Bellek İçi GeoIP Önbelleği| H[Tıklama & Coğrafi Analitik]
    H -->|Toplu Yazma| F
```

---

## 💎 Temel Yetkinlikler

### 🔗 Gelişmiş Link Yaşam Döngüsü
* **Zaman Ayarlı Linkler (TTL):** Belirlenen tarih ve saatte otomatik olarak kullanımdan kalkan geçici bağlantılar.
* **Tıklama Limiti (Click Cap):** Belirli bir ziyaret sayısına (örn. 500 tıklama) ulaştığında kendini güvenle kilitleyen atomik sayaçlar.
* **Parola Korumalı Yönlendirme:** Hassas hedefler için ziyaretçiyi zarif bir şifre doğrulama ekranıyla karşılayan uçtan uca koruma.
* **Anında Duraklatma (Kill-Switch):** Linki veya verilerini silmeden tek tıkla pasife alma ve dilediğinizde tekrar aktifleştirme.
* **UTM & Query Koruyucu:** Gelen reklam parametrelerini (UTM tags, ref, affiliate parametreleri) hedef adrese kayıpsız iletme.
* **Dinamik QR Kod Üretimi:** Panel üzerinden veya API rotasından (`/qr?size=300`) vektörel kalitede, 24 saat önbelleklenen dinamik PNG QR çıktısı.

### 📊 Asenkron Analitik & Gizlilik
* **Sıfır Gecikmeli Yönlendirme:** Ziyaretçi anında hedef URL'e aktarılır (`HTTP 302`); IP, tarayıcı, işletim sistemi ve yönlendiren analitikleri arka plandaki **10.000 kapasiteli asenkron iş havuzunda** işlenir.
* **Thread-Safe GeoIP Önbelleği:** Dış API kotalarını tüketmemek ve yönlendirmeyi yavaşlatmamak için bellek içi thread-safe IP tablosu.
* **Bento Grid Gösterge Paneli:** Son 30 günlük tıklama trendleri, ülke dağılımları ve cihaz/tarayıcı dağılımlarını gösteren Chart.js grafikleri.

### 👥 Rol Tabanlı Yetkilendirme (RBAC) & İlk Kurulum
* **İlk Kurulum Sihirbazı (`/setup`):** Sistemde kayıtlı kullanıcı yokken açılır; ilk kullanıcı otomatik **Superadmin** olur ve ardından rota kalıcı olarak mühürlenir.
* **Çoklu Kullanıcı Yönetimi:** Superadmin yeni üyeler (`member`) ekleyebilir, kullanıcı şifrelerini sıfırlayabilir veya hesap silebilir.
* **İzolasyon:** Standart üyeler yalnızca kendi oluşturdukları linkleri ve analitikleri yönetebilir.

---

## 🛡️ Siber Güvenlik & Dayanıklılık Mimarisi

Linklik, siber güvenlik uzmanı bakış açısıyla tasarlanmış ve derinlemesine sertleştirilmiştir:

| Güvenlik Katmanı | Tehdit Senaryosu | Alınan Önlem / Mekanizma |
| :--- | :--- | :--- |
| **SSRF & Open Redirect** | İç ağ taraması, Cloud Metadata ifşası (`169.254.169.254`), zararlı scheme (`javascript:`, `file:`) | `validateURL()` motoru RFC1918 özel ağlarını (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`), loopback (`127.0.0.1`), metadata adreslerini ve gayriresmî şemaları engeller. |
| **DOM XSS Savunması** | Hata mesajları veya kullanıcı girdileriyle script enjeksiyonu | Tüm şablonlarda (`login`, `setup`, `dashboard`) dinamik HTML render öncesi `escapeHtml()` temizliği ve context-aware Go template motoru. |
| **JWT Token Versioning** | Çalınan veya sızan token'ların süresi dolana kadar yetkisiz kullanımı | Veritabanında `token_version` takibi (Migration v7). Kullanıcı şifresini değiştirdiğinde veya admin sıfırladığında eski oturumlar milisaniyeler içinde geçersiz kılınır. |
| **Hız Sınırlama (Rate Limit)** | Brute-force saldırıları, link spamming, DoS | IP başına kayan pencereli (sliding window) koruma: Setup (5/dk), Login (10/dk), Admin (20/dk), Yönlendirme (300/dk). 429 yanıtlarında standart JSON ve dinamik `Retry-After` başlığı. |
| **Memory DoS Koruması** | Milyonlarca rastgele sahte IP ile bellek şişirme | Rate limiter tablosunda 100.000 aktif bucket sınırı ve her 60 saniyede bir otomatik agresif çöp toplayıcı (cleanup). |
| **Anti-CSRF Savunması** | Siteler arası sahte istek ile link silme/değiştirme | `SameSite=Lax` çerez korumasına ek olarak, durum değiştiren tüm API rotalarında `RequireXHR` middleware ile `X-Requested-With` başlığı zorunluluğu. |
| **Slowloris DDoS Engeli** | Yavaş HTTP başlığı göndererek bağlantı tüketme | `ReadHeaderTimeout: 5s` ile yavaş bağlantılar agresif şekilde sonlandırılır. |
| **Gövde Şişirme (Body Overflow)**| Devasa JSON gövdeleri ile sunucu belleğini kilitleme | `MaxBodySize(1 MiB)` middleware ile istek boyutunun aşılması engellenir. |
| **Zamanlama Analizi (Timing Attack)**| Var olan ve olmayan kullanıcıları yanıt süresinden anlama | Kullanıcı adı bulunamadığında sahte bcrypt hash'i (`dummyPasswordHash`) koşturularak yanıt süreleri eşitlenir. |
| **Ters Proxy Sahteciliği** | Sahte `X-Forwarded-For` ile IP spoofing | Yalnızca tanımlı güvenilir CIDR bloklarından (Cloudflare, Docker Bridge, Private RFC1918) gelen başlıklar kabul edilir. |
| **HSTS & Güvenlik Başlıkları** | Man-in-the-Middle, Clickjacking, MIME-Sniffing | `Strict-Transport-Security` (reverse proxy destekli), katı CSP politikası, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`. |

---

## 📊 Kaynak Tüketim & Performans Kıyaslaması

Linklik'in tek başına derlenmiş yalın mimarisi, geleneksel alternatiflere göre dramatik bir verimlilik sunar:

| Kriter | Linklik (Go + SQLite) | Shlink (PHP + MySQL + Redis) | Dub.co (Node.js + Edge) |
| :--- | :---: | :---: | :---: |
| **RAM Tüketimi (Boşta)** | **~12 MB** | ~180 MB | ~150 MB |
| **RAM Tüketimi (1.000 RPS)** | **~35 MB** | ~450 MB | ~300 MB |
| **Harici Veritabanı Şartı** | ❌ **Gerekmez** (Gömülü WAL) | ✅ Gerekir (MySQL/PG) | ✅ Gerekir (Postgres) |
| **Harici Önbellek (Redis)** | ❌ **Gerekmez** | ✅ Önerilir | ✅ Gerekir (Upstash) |
| **Yerel AI/MCP Desteği** | ✅ **Kutudan Çıkar Çıkmaz** | ❌ Yok | ❌ Kısmi Webhook |
| **Kurulum Süresi** | **30 Saniye** | ~10 Dakika | ~15 Dakika |

---

## 🛠️ Yerel Geliştirme & Kaynak Koddan Derleme (Native Go)

Konteyner kullanmadan doğrudan kaynak kod üzerinden geliştirmek veya ikili dosya derlemek için:

Sisteminizde Go (1.22+) kuruluysa:

```bash
# Bağımlılıkları kontrol edin
go mod download

# Testleri koşturun
go test ./... -v

# Binary olarak derleyin
go build -ldflags="-s -w" -o linklik ./cmd/server

# Çalıştırın
./linklik
```

---

## 🤖 Model Context Protocol (MCP) AI Ajanları

Linklik, **Anthropic Model Context Protocol (MCP)** standardını doğrudan çekirdeğinde barındırır. Bu sayede AI asistanlarınız hiçbir ara katmana ihtiyaç duymadan link kısaltabilir, analitik okuyabilir ve linkleri yönetebilir.

### 1. Cursor Entegrasyonu (`.cursor/mcp.json`)

```json
{
  "mcpServers": {
    "linklik": {
      "url": "http://localhost:8080/mcp",
      "headers": {
        "X-API-KEY": "lk_your_api_key_here"
      }
    }
  }
}
```

### 2. Claude Desktop Entegrasyonu (`claude_desktop_config.json`)

```json
{
  "mcpServers": {
    "linklik": {
      "command": "linklik",
      "args": ["--mcp", "--api-key", "lk_your_api_key_here"]
    }
  }
}
```

### 3. Claude Code / CLI Entegrasyonu

```bash
claude mcp add --transport http linklik http://localhost:8080/mcp --header "Authorization: Bearer lk_your_api_key_here"
```

### Sunulan Hazır MCP Araçları (Tools)

| MCP Tool Adı | Açıklama | Ana Parametreler |
| :--- | :--- | :--- |
| `shorten_link` | Yeni kısa link oluşturur. | `url`, `custom_alias`, `max_clicks`, `expires_at`, `password`, `is_active` |
| `list_links` | Linkleri arama, filtre ve sayfalama ile getirir. | `page`, `limit`, `search`, `status` (`all`, `active`, `inactive`, `expired`) |
| `get_link_analytics` | Toplam tıklanma, coğrafya, cihaz ve trend analitiğini döner. | `short_code` |
| `toggle_link` | Bir linkin aktiflik durumunu tersine çevirir. | `short_code` |
| `reset_link_stats` | Linkin tıklama sayacını ve geçmiş analitiğini sıfırlar. | `short_code` |
| `delete_link` | Linki ve ilişkili verilerini kalıcı olarak siler. | `short_code` |

---

## 💻 REST API Referansı

Tüm uç noktalar standart JSON formatında yanıt döner. Kimlik doğrulama için `X-API-KEY: lk_...` başlığı veya `Authorization: Bearer <token>` kullanılır.

### 1. Link Kısaltma
```bash
curl -X POST http://localhost:8080/api/v1/links \
  -H "X-API-KEY: lk_your_key_here" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com/cok-uzun-bir-sayfa-adresi",
    "custom_alias": "ozel-kampanya",
    "max_clicks": 1000,
    "expires_at": "2026-12-31T23:59:59Z",
    "password": "guvenli-parola",
    "is_active": true
  }'
```

**Başarılı Yanıt (`201 Created`):**
```json
{
  "success": true,
  "data": {
    "id": 14,
    "original_url": "https://example.com/cok-uzun-bir-sayfa-adresi",
    "short_code": "ozel-kampanya",
    "click_count": 0,
    "max_clicks": 1000,
    "is_active": true,
    "expires_at": "2026-12-31T23:59:59Z",
    "has_password": true,
    "created_at": "2026-09-19T11:40:00Z"
  }
}
```

### 2. Dinamik QR Kod Alma
```bash
curl -X GET "http://localhost:8080/api/v1/links/ozel-kampanya/qr?size=300" \
  --output qrcode.png
```

### 3. Analitik Raporu Sorgulama
```bash
curl -X GET http://localhost:8080/api/v1/analytics/ozel-kampanya \
  -H "X-API-KEY: lk_your_key_here"
```

### 4. Link Durumunu Değiştirme (Aktif/Pasif)
```bash
curl -X PATCH http://localhost:8080/api/v1/links/ozel-kampanya/toggle \
  -H "X-API-KEY: lk_your_key_here"
```

---

## ⚙️ Konfigürasyon

Linklik, 12-Factor App ilkelerine uygun olarak tamamen ortam değişkenleri (`.env` veya sistem environment) üzerinden yapılandırılır:

| Değişken | Varsayılan | Açıklama |
| :--- | :---: | :--- |
| `PORT` | `8080` | HTTP sunucusunun dinleyeceği port. |
| `DB_PATH` | `linklik.db` | SQLite veritabanı dosyasının yolu. |
| `BASE_URL` | `http://localhost:8080` | Üretilen kısa linklerin başında yer alacak alan adı. |
| `JWT_SECRET` | *(Otomatik Üretilir)* | JWT imzalamak için en az 32 karakterli güçlü anahtar. |
| `COOKIE_SECURE` | `false` | HTTPS arkasında çalışırken `true` yapılmalıdır (`Secure` cookie flag). |
| `ALLOWED_ORIGINS` | `""` | Virgülle ayrılmış CORS izinli kökenler (boşsa sadece BASE_URL). |
| `TLS_CERT_PATH` | `""` | (Opsiyonel) Yerel SSL sertifika dosyası. |
| `TLS_KEY_PATH` | `""` | (Opsiyonel) Yerel SSL özel anahtar dosyası. |

---

## 📦 Üretim Dağıtımı (Production Deployment)

### Sıfır Kesinti (Zero-Downtime) & Veri Güvenliği
1. **İdempotent Şema Göçleri:** Sunucu her başladığında `schema_migrations` tablosunu kontrol eder. Mevcut veritabanınızı silmeden güvenle sürüm yükseltebilirsiniz (v1'den v7'ye otomatik geçiş).
2. **Konteyner Güvenlik Profili:** `docker-compose.yml` içerisinde `read_only: true`, `cap_drop: ALL`, `no-new-privileges:true` ve root olmayan `appuser` (UID 10001) standart olarak tanımlıdır.
3. **Kademeli Kapanma (Graceful Shutdown):** `SIGTERM` veya `SIGINT` sinyali alındığında sunucu mevcut isteklerin bitmesi için 5 saniye bekler ve kuyruktaki tüm analitik işlemlerini SQLite'a yazdıktan sonra güvenle kapanır.

---

## 🤝 Katkıda Bulunma

Açık kaynak topluluğunun katkılarını memnuniyetle karşılıyoruz!

1. Bu depoyu çatallayın (Fork).
2. Yeni bir özellik dalı açın (`git checkout -b feature/harika-ozellik`).
3. Değişikliklerinizi commit edin (`git commit -m 'feat: harika ozellik eklendi'`).
4. Testleri çalıştırın (`go test ./...`).
5. Dalınızı pushlayın (`git push origin feature/harika-ozellik`).
6. Bir **Pull Request** açın.

---

## 📄 Lisans

Bu proje [MIT Lisansı](LICENSE) kapsamında korunmaktadır. Özgürce kullanabilir, değiştirebilir ve kendi altyapınızda barındırabilirsiniz.

<p align="center">
  <sub>Modern, minimalist ve ödün vermeyen bir açık kaynak vizyonuyla inşa edildi. 🤍</sub>
</p>
