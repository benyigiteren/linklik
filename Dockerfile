# --- Derleme Aşaması (Build Stage) ---
FROM golang:1.26-alpine AS builder

# Gerekli sistem paketlerini yükle
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Bağımlılıkları kopyala ve indir
COPY go.mod go.sum ./
RUN go mod download

# Kaynak kodları kopyala
COPY . .

# Uygulamayı statik olarak derle (CGO_ENABLED=0 pure Go SQLite kullandığımız için kolayca derlenir)
RUN CWD=$(pwd) && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o linklik ./cmd/server/main.go

# --- Çalışma Zamanı Aşaması (Runtime Stage) ---
# Güvenlik: en az ana sürüm sabitlenmiş (drift riskini azaltır)
FROM alpine:3.20

# SSL sertifikalarını yükle (GeoIP HTTPS istekleri ve dış yönlendirmeler için gerekli)
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 -h /app appuser && \
    mkdir -p /app/data && \
    chown -R appuser:appuser /app

WORKDIR /app

# Derlenen çalıştırılabilir dosyayı kopyala
COPY --from=builder /app/linklik .

# Çevresel değişken varsayılanları
ENV PORT=8080
ENV DB_PATH=/app/data/linklik.db
ENV BASE_URL=http://localhost:8080

# Portu aç
EXPOSE 8080

# Güvenlik: kapsayıcıyı root yetkileri olmayan bir kullanıcı ile çalıştır
USER appuser

# Sağlık Kontrolü (Docker & Kubernetes Container Healthcheck)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

# Uygulamayı çalıştır
CMD ["./linklik"]