package middleware

import (
	"net"
	"net/http"
	"os"
	"strings"
)

var defaultTrustedNets []*net.IPNet

func init() {
	// Varsayılan güvenilir CIDR aralıkları (loopback + Docker + özel ağlar)
	defaultCIDRs := []string{
		"127.0.0.0/8",    // IPv4 Loopback
		"::1/128",        // IPv6 Loopback
		"10.0.0.0/8",     // RFC1918 Private
		"172.16.0.0/12",  // RFC1918 Private (Docker default bridge: 172.17.x, 172.18.x)
		"192.168.0.0/16", // RFC1918 Private
		"169.254.0.0/16", // Link-local
		"fc00::/7",       // IPv6 Unique Local
		"fe80::/10",      // IPv6 Link-local
	}

	custom := os.Getenv("TRUSTED_PROXIES")
	if custom != "" {
		parts := strings.Split(custom, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				defaultCIDRs = append(defaultCIDRs, p)
			}
		}
	}

	for _, cidr := range defaultCIDRs {
		if !strings.Contains(cidr, "/") {
			if strings.Contains(cidr, ":") {
				cidr += "/128"
			} else {
				cidr += "/32"
			}
		}
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			defaultTrustedNets = append(defaultTrustedNets, ipNet)
		}
	}
}

// isTrustedProxy gelen TCP soket IP'sinin güvenilir bir ters proxy (Nginx, Docker, Cloudflare) olup olmadığını kontrol eder.
func isTrustedProxy(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, network := range defaultTrustedNets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// TrustedProxyMiddleware Cloudflare, Nginx, Caddy veya Docker arkasında gerçek istemci IP'sini tespit eder.
// Böylece tüm kullanıcıların 127.0.0.1 veya Docker köprü IP'sine düşüp rate limit yemesi engellenir.
func TrustedProxyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			remoteHost = r.RemoteAddr
		}

		clientIP := remoteHost

		// Eğer doğrudan bağlanan istemci güvenilir bir proxy ise başlıklara bak
		if isTrustedProxy(remoteHost) {
			// 1. Cloudflare başlığı
			if cfIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cfIP != "" {
				if parsed := net.ParseIP(cfIP); parsed != nil {
					clientIP = cfIP
				}
			} else if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
				// 2. X-Forwarded-For: sol baştaki ilk IP gerçek istemcidir
				ips := strings.Split(xff, ",")
				for _, raw := range ips {
					raw = strings.TrimSpace(raw)
					if parsed := net.ParseIP(raw); parsed != nil {
						clientIP = raw
						break
					}
				}
			} else if xRealIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); xRealIP != "" {
				// 3. X-Real-IP
				if parsed := net.ParseIP(xRealIP); parsed != nil {
					clientIP = xRealIP
				}
			}
		}

		r.RemoteAddr = net.JoinHostPort(clientIP, "0")
		next.ServeHTTP(w, r)
	})
}
