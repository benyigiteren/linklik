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
  <strong>Mikro Kaynak Tüketimli, Yapay Zekâ (MCP) Uyumlu ve Üretime Hazır Modern URL Kısaltma & Analitik Platformu</strong>
</p>

<p align="center">
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go Version"></a>
  <a href="https://sqlite.org/"><img src="https://img.shields.io/badge/SQLite-WAL%20(Zero%20CGO)-003B57?style=flat-square&logo=sqlite&logoColor=white" alt="SQLite"></a>
  <a href="https://modelcontextprotocol.io/"><img src="https://img.shields.io/badge/MCP-Native%202024--11--05-7C3AED?style=flat-square&logo=anthropic&logoColor=white" alt="MCP Compatible"></a>
  <a href="#-performans-ve-donan%C4%B1m-verimlili%C4%9Fi"><img src="https://img.shields.io/badge/RAM-10--15%20MB%20(Idle)-10B981?style=flat-square&logo=speedtest&logoColor=white" alt="RAM Idle"></a>
  <a href="#-performans-ve-donan%C4%B1m-verimlili%C4%9Fi"><img src="https://img.shields.io/badge/Throughput-10.000%2B%20RPS-F59E0B?style=flat-square&logo=fastapi&logoColor=white" alt="RPS"></a>
  <a href="https://www.docker.com/"><img src="https://img.shields.io/badge/Docker-Multi--Arch%20Ready-2496ED?style=flat-square&logo=docker&logoColor=white" alt="Docker Ready"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-6366F1?style=flat-square" alt="MIT License"></a>
</p>

<p align="center">
  <a href="#-10-saniyede-h%C4%B1zl%C4%B1-ba%C5%9Flang%C4%B1%C3%A7-quick-start"><strong>Hızlı Başlangıç</strong></a> •
  <a href="#-linklik-nedir-ve-ne-i%C5%9Fe-yarar">Ne İşe Yarar?</a> •
  <a href="#-neden-linklik-farklar%C4%B1-nelerdir">Farkları Nelerdir?</a> •
  <a href="#-temel-yetkinlikler">Özellikler</a> •
  <a href="#-model-context-protocol-mcp-ai-ajanlar%C4%B1">MCP & AI</a> •
  <a href="#-rest-api-referans%C4%B1">REST API</a> •
  <a href="#-konfig%C3%BCrasyon">Konfigürasyon</a>
</p>

---

## ⚡ 10 Saniyede Hızlı Başlangıç (Quick Start)

Linklik'i sunucunuzda veya yerel ortamınızda çalıştırmak için hazır **GHCR (GitHub Container Registry)** imajını tek komutla başlatabilirsiniz:

### 🐳 Seçenek 1: Tek Komutla Docker Run (Önerilen)

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

> **🎉 Kurulum Tamamlandı:** Tarayıcınızdan `http://localhost:8080` adresine gidin. Sistem otomatik olarak sizi `/setup` sihirbazına yönlendirecek ve ilk Superadmin hesabınızı oluşturacaktır.

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

## 📖 Linklik Nedir ve Ne İşe Yarar?

**Linklik**, kendi sunucunuzda barındırabileceğiniz (self-hosted), Go ve SQLite ile geliştirilmiş, ultra hafif, modern ve yapay zekâ uyumlu bir URL kısaltma ve analiz platformudur.

Uzun, karmaşık web bağlantılarını akılda kalıcı kısa linklere dönüştürmenin ötesinde, tam kapsamlı bir link yaşam döngüsü ve ziyaretçi analizi sunar:

### 🎯 Nerelerde Kullanılır?
* **Pazarlama & Sosyal Medya:** Kampanyalarınız için özel takma adlı (custom alias) linkler oluşturun. Gelen reklam etiketlerini (UTM parametreleri) hedef siteye kayıpsız iletir.
* **Süreli / Flaş Kampanyalar:** Belirli bir tarih ve saatte geçerliliğini yitiren (TTL) linkler tanımlayın. Süre dolduğunda yönlendirme otomatik durur.
* **Kontenjanlı Paylaşımlar:** Belirli bir tıklama sınırına (örn: ilk 500 kişi) ulaştığında kendini kilitleyen bağlantılar oluşturun.
* **Özel / Gizli Dökümanlar:** Ziyaretçiden parola isteyen şifre korumalı linkler oluşturarak dosya veya özel sayfalarınızı kontrollü paylaşın.
* **Hızlı Durdurma (Kill-Switch):** Bir kampanyayı silmeden tek tıkla pasife alın, dilediğinizde tek tıkla tekrar açın.
* **Baskı & Tanıtım İçin QR:** Her link için anında yüksek çözünürlüklü dinamik PNG QR kodlar üretin.
* **Ziyaretçi Analizi:** Linklerinize hangi ülkelerden, hangi tarayıcılardan ve hangi günlerde tıklandığını 3. parti izleyici kodlara ihtiyaç duymadan takip edin.
* **Yapay Zekâ ile Otomasyon:** Claude Desktop, Cursor veya terminal asistanlarınıza Linklik'i bağlayın; AI ajanlarınız sizin adınıza sohbet içerisinden link oluştursun ve analiz etsin.

---

## 🏆 Neden Linklik? Farkları Nelerdir?

Piyasadaki popüler URL kısaltıcılar (Bitly, Shlink, Dub, TinyURL) ile karşılaştırıldığında Linklik'in öne çıkan mimari farkları:

| Karşılaştırma Kriteri | Linklik | Geleneksel Çözümler (Shlink, Kutt vb.) | Bulut Servisleri (Bitly, Dub vb.) |
| :--- | :---: | :---: | :---: |
| **Bellek Tüketimi (RAM)** | **10–15 MB (İnanılmaz Hafif)** | 180–450 MB (PHP/Node runtime) | Bulut (Ücretli Kota Sınırları) |
| **Harici Veritabanı** | **Gerekmez** (Gömülü Pure-Go SQLite) | MySQL veya PostgreSQL şart | Sağlayıcı Altyapısı |
| **Harici Önbellek (Redis)** | **Gerekmez** (Bellek içi önbellek) | Redis veya Memcached önerilir | Redis/Upstash bağımlılığı |
| **Yapay Zekâ / MCP Desteği** | ✅ **Yerleşik Model Context Protocol** | ❌ Yok | ❌ Kısmi Webhook |
| **Tıklama Gecikmesi** | **0 ms Ek Gecikme** (Asenkron kuyruk) | Çoğu çözümde senkron DB kaydı | Ağ mesafesine bağlı |
| **Veri Gizliliği** | ✅ **%100 Kendi Sunucunuzda** | ✅ Kendi Sunucunuzda | ❌ Veriler 3. şahıslarda |
| **Tek Binary Dağıtımı** | ✅ **Var (~15 MB tek dosya)** | ❌ Yok (Yüzlerce dosya/vendor) | ❌ Yok |
| **Ücretsiz & Sınırsız** | ✅ **Tamamen Açık Kaynak (MIT)** | ✅ Açık Kaynak | ❌ Aylık link/tıklama limitli |

---

## 💎 Temel Yetkinlikler

### 🔗 1. Akıllı Link Yönetimi
* **Özel Takma Ad (Alias):** `linklik.com/yaz-indirimi` gibi markanıza özel kısa adresler.
* **Son Kullanma Tarihi (TTL):** Etkinlik veya kampanya bitişinde otomatik kapanma.
* **Maksimum Tıklama Sayacı:** Atomik sayaç kontrolüyle kota dolduğunda otomatik kilitleme.
* **Parola Korumalı Bağlantı:** Ziyaretçiyi şık bir şifre giriş formuyla karşılayan güvenli yönlendirme.
* **Tek Tıkla Açma/Kapama:** Linki silmeden dilediğiniz an durdurabilme.
* **Gelişmiş Arama & Filtreleme:** Durumuna göre (`Tümü`, `Aktif`, `Pasif`, `Süresi Dolan`) anında filtreleme.
* **Dinamik QR Kod:** Doğrudan panelden veya API'den (`/qr?size=300`) 24 saat önbellekli PNG çıktısı.

### 📊 2. Asenkron & Gizlilik Odaklı Analitik
* **Sıfır Gecikmeli Yönlendirme:** Ziyaretçi bekletilmeden anında hedef adrese aktarılır (`HTTP 302`); tıklama verileri 4 worker'lı arka plan kuyruğunda işlenir.
* **İç Bellek GeoIP Önbelleği:** Dış servislere bağımlılığı ve kota harcamasını engelleyen dahili IP-ülke tablosu.
* **Bento Grid Grafikleri:** Son 30 günlük tıklama trendleri, ülke dağılımları ve cihaz/tarayıcı dağılımlarını gösteren görsel raporlama.

### 👥 3. Rol Tabanlı Çoklu Kullanıcı
* **Superadmin:** Tüm linkleri yönetebilir, yeni üyeler ekleyebilir, kullanıcıları silebilir veya şifrelerini sıfırlayabilir.
* **Üye (Member):** Yalnızca kendi oluşturduğu linkleri ve bunlara ait analitikleri görüntüleyebilir.
* **İlk Kurulum Koruması:** Sistemde kullanıcı yokken açılan kurulum sihirbazı, ilk kayıt sonrası kalıcı olarak kapanır.

---

## 🏛️ Çalışma Mimarisi

```mermaid
flowchart LR
    A[Kullanıcı / AI Ajan / Ziyaretçi] --> B{Ters Proxy & Yük Dengeleyici}
    B --> C[Chi Router & HTTP Katmanı]
    C --> D[Hız Sınırlayıcı Rate Limiter]
    D --> E[İş Mantığı Katmanı Service]
    E -->|Kısa Kod Sorgulama| F[(Gömülü SQLite WAL)]
    E -.->|Gecikmesiz Asenkron Kuyruk| G[4x Arka Plan Worker Havuzu]
    G -->|Bellek İçi Önbellek| H[Tıklama & Coğrafi Analitik]
    H -->|Toplu Kayıt| F
```

---

## ⚡ Performans ve Donanım Verimliliği

Linklik, en ucuz VPS sunucularında (1 vCPU, 512 MB – 1 GB RAM) bile binlerce istek altında sorunsuz çalışacak şekilde optimize edilmiştir:

| Metrik | Değer |
| :--- | :--- |
| **Bellek Tüketimi (Boşta)** | 10 – 15 MB |
| **Bellek Tüketimi (Yüksek Yük)** | 30 – 50 MB |
| **Derlenmiş Dosya Boyutu** | ~15 MB |
| **Disk İhtiyacı (10.000 link + 100.000 tıklama)** | ~20 – 25 MB |
| **İşlem Kapasitesi (1 vCPU VPS)** | 10.000+ İstek/Saniye (RPS) |

---

## 🤖 Model Context Protocol (MCP) AI Ajanları

Linklik, **Model Context Protocol (MCP)** standardını doğrudan destekler. AI asistanlarınız (Claude Desktop, Cursor, Gemini, Claude Code vb.) panel açmanıza gerek kalmadan sohbet üzerinden işlem yapabilir.

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

### 3. Claude Code / Terminal CLI Entegrasyonu

```bash
claude mcp add --transport http linklik http://localhost:8080/mcp --header "Authorization: Bearer lk_your_api_key_here"
```

### AI Tarafından Kullanılabilen Hazır MCP Araçları:

| Araç Adı | Ne İşe Yarar? | Ana Parametreler |
| :--- | :--- | :--- |
| `shorten_link` | Yeni kısa link oluşturur (alias, limit, parola, TTL destekli). | `url`, `custom_alias`, `max_clicks`, `expires_at`, `password` |
| `list_links` | Linkleri arama, filtreleme ve sayfalama ile listeler. | `page`, `limit`, `search`, `status` |
| `get_link_analytics` | Toplam tıklama, ülke ve tarayıcı istatistiklerini getirir. | `short_code` |
| `toggle_link` | Bir linkin durumunu anında aktif veya pasif yapar. | `short_code` |
| `reset_link_stats` | Linkin tıklama sayacını ve geçmiş analitiğini sıfırlar. | `short_code` |
| `delete_link` | Linki kalıcı olarak siler. | `short_code` |

---

## 💻 REST API Referansı

Her kullanıcının panelden alabileceği bir `X-API-KEY` anahtarı bulunur.

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
    "password": "istege-bagli-sifre",
    "is_active": true
  }'
```

**Örnek Başarılı Yanıt (`201 Created`):**
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

### 3. Analitik Raporu Alma
```bash
curl -X GET http://localhost:8080/api/v1/analytics/ozel-kampanya \
  -H "X-API-KEY: lk_your_key_here"
```

### 4. Link Durumunu Değiştirme (Aktif / Pasif)
```bash
curl -X PATCH http://localhost:8080/api/v1/links/ozel-kampanya/toggle \
  -H "X-API-KEY: lk_your_key_here"
```

---

## 🛠️ Yerel Geliştirme & Kaynak Koddan Derleme

Konteyner kullanmadan doğrudan Go ile çalıştırmak veya ikili dosya derlemek için:

```bash
# Bağımlılıkları kontrol edin
go mod download

# Testleri çalıştırın
go test ./... -v

# Binary olarak derleyin
go build -ldflags="-s -w" -o linklik ./cmd/server

# Çalıştırın
./linklik
```

---

## ⚙️ Konfigürasyon

Linklik tamamen ortam değişkenleri (`.env` veya sistem değişkenleri) üzerinden yapılandırılır:

| Değişken | Varsayılan | Açıklama |
| :--- | :---: | :--- |
| `PORT` | `8080` | Sunucunun dinleyeceği port. |
| `DB_PATH` | `linklik.db` | SQLite veritabanı dosyasının yolu. |
| `BASE_URL` | `http://localhost:8080` | Kısaltılan linklerin sunulacağı alan adı. |
| `JWT_SECRET` | *(Otomatik Üretilir)* | Oturum doğrulaması için en az 32 karakterli gizli anahtar. |
| `COOKIE_SECURE` | `false` | HTTPS arkasında çalışıyorsanız `true` yapın. |
| `ALLOWED_ORIGINS` | `""` | Virgülle ayrılmış CORS izinli adresler (boşsa BASE_URL kullanılır). |
| `TLS_CERT_PATH` | `""` | (Opsiyonel) SSL sertifika dosyası. |
| `TLS_KEY_PATH` | `""` | (Opsiyonel) SSL özel anahtar dosyası. |

---

## 📦 Üretim Dağıtımı (Production)

* **Otomatik Veritabanı Şeması:** Sunucu başladığında `schema_migrations` tablosunu denetler; sürüm yükseltmelerinde veritabanınızı silmeniz gerekmez.
* **Konteyner Mimarisi:** `docker-compose.yml` içinde root olmayan kullanıcı (`UID 10001`), salt okunur dosya sistemi (`read_only: true`) ve güvenli izin kısıtları standarttır.
* **Kesintisiz Kapanma (Graceful Shutdown):** `SIGTERM` sinyali geldiğinde bekleyen istekler tamamlanır ve kuyruktaki analitik işleri SQLite'a yazıldıktan sonra temiz bir şekilde kapanır.

---

## 📄 Lisans

Bu proje açık kaynaklı olup, [MIT Lisansı](LICENSE) altında dağıtılmaktadır.

<p align="center">
  <sub>Modern, hafif ve amaca yönelik bir açık kaynak vizyonuyla inşa edildi. 🤍</sub>
</p>
