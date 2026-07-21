<p align="center">
  <img src="readmegorsel.png" alt="Linklik Dashboard" width="100%">
</p>

<h1 align="center">🔗 Linklik — Hafif, Yüksek Performanslı ve API-First Link Kısaltma Servisi</h1>

<p align="center">
  Go & SQLite ile geliştirilmiş, minimal bağımlılığa sahip, üretim için tasarlanmış açık kaynaklı URL kısaltma servisi.
</p>

---

## 📖 Nedir?

**Linklik**, Go (Golang) ve SQLite kullanılarak geliştirilmiş, minimal bağımlılığa sahip, yüksek performanslı ve modern tasarımlı açık kaynaklı bir URL kısaltma servisidir.

Proje, hem kullanıcı dostu bir yönetim paneline sahip olması hem de AI ajanlarının (AI Agents) ve dış servislerin HTTP istekleriyle saniyeler içinde entegre olabilmesi için **API-First (API Öncelikli)** felsefesiyle tasarlanmıştır.

## 🚀 Öne Çıkan Özellikler

- **Go & Chi Router** — Hızlı, ölçeklenebilir ve standart kütüphaneye yakın modern web mimarisi.
- **WAL Modu SQLite** — CGO gerektirmeyen (pure Go) modern SQLite sürücüsü ve Write-Ahead Logging (WAL) etkinleştirilmiş veri tabanı katmanı sayesinde yüksek eşzamanlı okuma/yazma performansı.
- **İlk Kurulum (Setup) Akışı** — Sistemde hiç kullanıcı bulunmadığında `/setup` sayfası üzerinden kaydolan ilk kişi otomatik **Superadmin** olur ve ardından bu rota kalıcı olarak kapatılır.
- **Asenkron Analitik** — Yönlendirmeler esnasında IP adresi, User-Agent, Referrer ve Ülke (GeoIP API asenkron fallback) çözümleme işlemleri ana yönlendirme akışını yavaşlatmamak için **Go Goroutines** ile arka planda asenkron çalışır.
- **API-First Yetkilendirme** — Tüm API istekleri hem web paneli çerezleri hem de kullanıcıya özel üretilen `X-API-KEY` başlığı ile güvenceye alınır.
- **Premium Karanlık Temalı Arayüz** — Modern Glassmorphism efektleri, Bento Grid düzeni ve tıklanma istatistiklerini günlere göre çizen Chart.js çizgi grafik entegrasyonu.
- **Kullanıcı Yönetimi** — Superadmin yetkili hesapların, diğer üyeleri oluşturabileceği ve silebileceği yönetim modülü.

## 💾 Donanım ve Kaynak Tüketim Raporu

Linklik, Go diliyle derlendiği ve SQLite gömülü veritabanını kullandığı için sistem kaynaklarını neredeyse tüketmez:

| Metrik | Değer |
| --- | --- |
| **RAM (Boşta)** | 10 – 15 MB |
| **RAM (Yoğun Yük)** | 30 – 50 MB |
| **Binary Boyutu** | ~15 MB |
| **Disk / 10K link + 100K tıklama** | ~20 – 25 MB |
| **Throughput (1 vCPU / 1 GB VPS)** | 10.000+ RPS |

SQLite WAL (Write-Ahead Logging) modlu tek yazıcı mimarisi sayesinde, standart 1 CPU / 1 GB RAM VPS sunucusunda saniyede **10.000+ yönlendirme isteği (RPS)** kilitlenme hatası alınmadan karşılanabilir. Her bir yönlendirme/analitik kaydı yaklaşık **150–200 byte** yer kaplar.

## 🛠️ Klasör Yapısı

```text
├── cmd/
│   └── server/          # Uygulamanın giriş noktası (main.go)
├── internal/
│   ├── config/          # Çevresel değişkenler ve ayarlar
│   ├── db/              # SQLite WAL aktivasyonu ve şema kurucu
│   ├── handler/         # HTTP Controller katmanı (Auth, Link, Analytics, Redirect, UI)
│   ├── middleware/      # X-API-KEY, CORS ve JWT oturum doğrulama ara yazılımları
│   ├── model/           # Go veri modelleri ve JSON istek/cevap yapıları
│   ├── repository/      # SQLite veritabanı SQL sorguları
│   └── service/         # Şifreleme, kısa kod üretimi ve asenkron analitik mantığı
├── web/
│   ├── static/          # CSS ve JS varlıkları (Gömülü)
│   ├── templates/       # HTML şablonları (Gömülü)
│   └── web.go           # 'go:embed' ile şablon paketleme tanımı
├── Dockerfile           # Konteynerize dağıtım dosyası
├── docker-compose.yml   # Kolay kurulum orkestrasyon dosyası
├── go.mod / go.sum      # Go bağımlılık yönetimi
└── README.md            # Kullanım kılavuzu
```

## 🤖 API Kullanımı ve AI Ajan Entegrasyonu

Her kullanıcının panelden görüntüleyebileceği ve yenileyebileceği bir `X-API-KEY` değeri bulunur. AI ajanları ve harici betikler bu anahtarı HTTP istek başlığına ekleyerek tüm işlemleri otomatize edebilir.

### 1. Link Kısaltma

- **Rota:** `POST /api/v1/links`
- **İstek Başlığı:** `X-API-KEY: lk_your_api_key_here`
- **İstek Gövdesi (JSON):**

```json
{
  "url": "https://deepmind.google/technologies/gemini/",
  "custom_alias": "gemini-model",
  "max_clicks": 500
}
```

> `custom_alias` ve `max_clicks` isteğe bağlıdır.

### 2. Link Düzenleme

- **Rota:** `PUT /api/v1/links/{short_code}`
- **İstek Başlığı:** `X-API-KEY: lk_your_api_key_here`
- **İstek Gövdesi (JSON):**

```json
{
  "url": "https://deepmind.google/technologies/gemini/v2",
  "custom_alias": "gemini-model-v2",
  "max_clicks": 1000
}
```

### 3. Kullanıcı Silme (Sadece Superadmin)

- **Rota:** `DELETE /api/v1/admin/users/{id}`
- **İstek Başlığı:** `X-API-KEY: lk_superadmin_api_key_here`

### 4. Detaylı Yardım Dokümanı

- **Rota:** `GET /api/v1/help`
- **İstek Başlığı:** `X-API-KEY: lk_your_api_key_here`

## ⚡ SQLite WAL Modu ve Performans Optimizasyonu

SQLite varsayılan olarak her yazma işleminde tüm veritabanı dosyasını kilitler. Linklik'te bu kilitlemeyi azaltmak ve yüksek tıklama trafiğinde kilitlenme (`locked database`) hataları yaşamamak için şu optimizasyonlar uygulanmıştır:

1. **WAL Modu Etkinleştirme** — `PRAGMA journal_mode=WAL;` ile okuma ve yazma işlemlerinin birbirini engellemeden eşzamanlı yapılması sağlanır.
2. **Eşzamanlılık Ayarı** — `PRAGMA synchronous=NORMAL;` ile yazma hızları artırılır.
3. **Bekleme Süresi** — `PRAGMA busy_timeout=5000;` ile kilit durumlarında SQLite hata dönmeden önce 5 saniye boyunca kilidin açılmasını bekler.
4. **Asenkron Goroutine Kaydı** — Yönlendirme biter bitmez kullanıcı asıl siteye anında yönlendirilir. Analitik satırının veritabanına eklenmesi arka plandaki bir goroutine ile yürütülerek veritabanı yazma yoğunluğunun web arayüz yanıt süresini etkilemesi engellenir.

## 🔒 Kurulum ve Kullanıcı Mantığı

1. **İlk Kurulum** — Proje ilk kez başlatıldığında `http://localhost:8080/setup` adresine gidin. Buradan oluşturacağınız ilk üyelik sisteme **Superadmin** olarak atanacaktır.
2. **Kayıtların Kapanması** — İlk kayıttan sonra `/setup` rotası ve `POST /api/v1/setup` uç noktası tamamen kapatılır. Dışarıdan halka açık kayıt olma seçeneği yoktur.
3. **Yeni Kullanıcı Ekleme/Silme** — Sadece Superadmin olan kullanıcılar yönetim panelindeki "Kullanıcı Yönetimi" sekmesinden veya API üzerinden sisteme normal üye (Member) tanımlayabilir veya silebilir. Bir Superadmin kendi hesabını veya diğer Superadmin hesaplarını silemez.

---

## 💻 Kurulum ve Çalıştırma

### 1. GitHub Container Registry ile Çalıştırma

Her `main` dalı gönderiminde image, GitHub Container Registry'ye `ghcr.io/benyigiteren/linklik:latest` etiketiyle; sürüm etiketi gönderimlerinde de sürüm etiketiyle yayımlanır. İlk yayımdan sonra image paketinin GitHub'da **Public** görünür olduğundan emin olun.

```bash
docker run --detach \
  --name linklik \
  --restart unless-stopped \
  --publish 8080:8080 \
  --volume linklik_data:/app/data \
  --env JWT_SECRET='en-az-32-karakterlik-guclu-bir-gizli-anahtar' \
  --env BASE_URL='https://linklik.example.com' \
  --env COOKIE_SECURE=true \
  ghcr.io/benyigiteren/linklik:latest
```

ARM64 ve AMD64 Linux sistemleri desteklenir. Kendi image sürümünüzü seçmek için `latest` yerine örneğin `v1.0.0` kullanın.

### 2. Docker Compose ile Hızlı Başlatma (Tavsiye Edilen)

GitHub Container Registry'deki image'i kullanmak için:

```bash
docker compose pull
docker compose up -d --no-build
```

Yerel kaynak kodundan image derlemek için `docker compose up -d --build` komutunu kullanın.

`JWT_SECRET` değerini Compose çalıştırmadan önce ortam değişkeni olarak ayarlayın; en az 32 karakter olmalıdır.

```bash
# Linux/macOS
export JWT_SECRET='en-az-32-karakterlik-guclu-bir-gizli-anahtar'

# PowerShell
$env:JWT_SECRET = 'en-az-32-karakterlik-guclu-bir-gizli-anahtar'
```


### 3. Yerel Olarak Çalıştırma (Go ile)

Bilgisayarınızda Go (1.26 veya üzeri) yüklü olmalıdır.

```bash
# Bağımlılıkları kontrol edin/düzenleyin
go mod tidy

# Sunucuyu başlatın
go run cmd/server/main.go
```

Sunucu varsayılan olarak `http://localhost:8080` adresinde çalışmaya başlayacaktır.

### 4. Veritabanını Sıfırlama (Yeniden Kurulum)

Sistemi sıfırlamak, tüm verileri silmek ve ilk kurulum ekranına (`/setup`) geri dönmek için:

1. Çalışan uygulamayı veya Docker konteynerini durdurun.
2. Yerel çalıştırmada proje kökündeki, Docker Compose çalıştırmasında ise `linklik_data` Docker volume'ündeki şu dosyaları silin:
   - `linklik.db`
   - `linklik.db-wal` (varsa)
   - `linklik.db-shm` (varsa)
3. Uygulamayı yeniden çalıştırdığınızda veritabanı otomatik olarak sıfırdan oluşturulacak ve ilk kayıt için `/setup` sayfasına yönlendirileceksiniz.

---

## 📄 Lisans

Bu proje açık kaynaklı olup, **MIT Lisansı** altında dağıtılmaktadır. Dilediğiniz gibi kullanıp geliştirebilirsiniz.
