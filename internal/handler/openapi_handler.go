package handler

import (
	"encoding/json"
	"linklik/internal/config"
	"net/http"
	"strings"
)

// OpenAPIHandler ChatGPT Actions ve OpenAPI istemcileri için şema sunar.
type OpenAPIHandler struct{}

// NewOpenAPIHandler yeni bir OpenAPIHandler oluşturur.
func NewOpenAPIHandler() *OpenAPIHandler {
	return &OpenAPIHandler{}
}

// HandleOpenAPI OpenAPI 3.0.3 JSON şemasını döner.
func (h *OpenAPIHandler) HandleOpenAPI(w http.ResponseWriter, r *http.Request) {
	baseURL := "http://localhost:8080"
	if config.GlobalConfig != nil && config.GlobalConfig.BaseURL != "" {
		baseURL = strings.TrimRight(config.GlobalConfig.BaseURL, "/")
	}

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "Linklik URL Kısaltma ve Analitik Servisi",
			"description": "Yüksek performanslı, güvenli, TTL ve şifre korumalı modern link kısaltma ve izleme servisi API'si.",
			"version":     "1.0.0",
		},
		"servers": []map[string]interface{}{
			{
				"url":         baseURL,
				"description": "Linklik API Sunucusu",
			},
		},
		"paths": map[string]interface{}{
			"/api/v1/links": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Yeni Link Kısalt",
					"description": "Verilen uzun URL'i kısaltır, isteğe bağlı özel takma ad, limit, şifre ve son kullanma tarihi belirler.",
					"operationId": "shortenLink",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"url"},
									"properties": map[string]interface{}{
										"url":          map[string]interface{}{"type": "string", "description": "Kısaltılacak orijinal web adresi"},
										"custom_alias": map[string]interface{}{"type": "string", "description": "İsteğe bağlı özel takma ad"},
										"max_clicks":   map[string]interface{}{"type": "integer", "description": "Maksimum tıklama limiti (0 = sınırsız)"},
										"expires_at":   map[string]interface{}{"type": "string", "description": "Son kullanma tarihi (ISO 8601, örn: 2026-12-31T23:59:00Z)"},
										"password":     map[string]interface{}{"type": "string", "description": "Erişim şifresi koruması (opsiyonel)"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"201": map[string]interface{}{"description": "Link başarıyla oluşturuldu"},
						"400": map[string]interface{}{"description": "Geçersiz istek"},
						"401": map[string]interface{}{"description": "Yetkisiz istek"},
					},
					"security": []map[string]interface{}{
						{"ApiKeyAuth": []string{}},
						{"BearerAuth": []string{}},
					},
				},
				"get": map[string]interface{}{
					"summary":     "Linkleri Listele",
					"description": "Kullanıcının kısaltılmış linklerini sayfalanmış ve filtreli olarak getirir.",
					"operationId": "listLinks",
					"parameters": []map[string]interface{}{
						{"name": "page", "in": "query", "schema": map[string]interface{}{"type": "integer", "default": 1}, "description": "Sayfa numarası"},
						{"name": "limit", "in": "query", "schema": map[string]interface{}{"type": "integer", "default": 20}, "description": "Sayfa başına kayıt"},
						{"name": "search", "in": "query", "schema": map[string]interface{}{"type": "string"}, "description": "URL veya alias içinde arama"},
						{"name": "status", "in": "query", "schema": map[string]interface{}{"type": "string", "enum": []string{"all", "active", "inactive", "expired"}}, "description": "Durum filtresi"},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Link listesi"},
					},
					"security": []map[string]interface{}{
						{"ApiKeyAuth": []string{}},
						{"BearerAuth": []string{}},
					},
				},
			},
			"/api/v1/links/{short_code}": map[string]interface{}{
				"delete": map[string]interface{}{
					"summary":     "Link Sil",
					"description": "Kısa kodu verilen linki kalıcı olarak siler.",
					"operationId": "deleteLink",
					"parameters": []map[string]interface{}{
						{"name": "short_code", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}, "description": "Silinecek kısa kod"},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Link silindi"},
					},
					"security": []map[string]interface{}{
						{"ApiKeyAuth": []string{}},
						{"BearerAuth": []string{}},
					},
				},
			},
			"/api/v1/links/{short_code}/toggle": map[string]interface{}{
				"patch": map[string]interface{}{
					"summary":     "Link Durumunu Değiştir",
					"description": "Linki aktif veya pasif duruma getirir.",
					"operationId": "toggleLink",
					"parameters": []map[string]interface{}{
						{"name": "short_code", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}, "description": "Kısa kod"},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Durum güncellendi"},
					},
					"security": []map[string]interface{}{
						{"ApiKeyAuth": []string{}},
						{"BearerAuth": []string{}},
					},
				},
			},
			"/api/v1/links/{short_code}/reset-stats": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "İstatistikleri Sıfırla",
					"description": "Linkin tıklama sayacını ve analitik kayıtlarını sıfırlar.",
					"operationId": "resetStats",
					"parameters": []map[string]interface{}{
						{"name": "short_code", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}, "description": "Kısa kod"},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "İstatistikler sıfırlandı"},
					},
					"security": []map[string]interface{}{
						{"ApiKeyAuth": []string{}},
						{"BearerAuth": []string{}},
					},
				},
			},
			"/api/v1/analytics/{short_code}": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Link Analitiği Getir",
					"description": "Tıklama, ülke, tarayıcı ve günlük performans raporlarını döner.",
					"operationId": "getAnalytics",
					"parameters": []map[string]interface{}{
						{"name": "short_code", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}, "description": "Kısa kod"},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Analitik raporu"},
					},
					"security": []map[string]interface{}{
						{"ApiKeyAuth": []string{}},
						{"BearerAuth": []string{}},
					},
				},
			},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"ApiKeyAuth": map[string]interface{}{
					"type": "apiKey",
					"in":   "header",
					"name": "X-API-KEY",
				},
				"BearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "API Key",
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(spec)
}
