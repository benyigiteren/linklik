package handler

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"linklik/internal/config"
	"linklik/internal/middleware"
	"linklik/internal/model"
	"linklik/internal/service"
	"net/http"
	"os"
	"strings"
	"sync"
)

// MCPHandler Model Context Protocol (MCP) isteklerini karşılayan handler'dır.
type MCPHandler struct {
	linkService      *service.LinkService
	analyticsService *service.AnalyticsService
	userService      *service.UserService
	sessions         sync.Map // sessionId -> chan []byte
}

// NewMCPHandler yeni bir MCPHandler oluşturur.
func NewMCPHandler(ls *service.LinkService, as *service.AnalyticsService, us *service.UserService) *MCPHandler {
	return &MCPHandler{
		linkService:      ls,
		analyticsService: as,
		userService:      us,
	}
}

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// HandleSSE AI istemcileri (Cursor, Claude vb.) için SSE akışını başlatır.
func (h *MCPHandler) HandleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE desteklenmiyor", http.StatusInternalServerError)
		return
	}

	sessionBytes := make([]byte, 16)
	_, _ = rand.Read(sessionBytes)
	sessionID := hex.EncodeToString(sessionBytes)

	msgChan := make(chan []byte, 32)
	h.sessions.Store(sessionID, msgChan)
	defer func() {
		h.sessions.Delete(sessionID)
		close(msgChan)
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// İstemciye mesaj gönderim endpoint URL'ini bildir
	endpointURL := fmt.Sprintf("/mcp/message?sessionId=%s", sessionID)
	_, _ = fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
	flusher.Flush()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg, ok := <-msgChan:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(msg))
			flusher.Flush()
		}
	}
}

// HandleMessage SSE oturumu üzerinden gelen veya doğrudan POST edilen JSON-RPC isteklerini karşılar.
func (h *MCPHandler) HandleMessage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		// 1. X-API-KEY başlığı
		apiKey := r.Header.Get("X-API-KEY")
		if apiKey == "" {
			// 2. Authorization: Bearer <key>
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}
		if apiKey == "" {
			// 3. URL Query param ?api_key= (SSE istemcileri için)
			apiKey = r.URL.Query().Get("api_key")
			if apiKey == "" {
				apiKey = r.URL.Query().Get("apiKey")
			}
		}
		if apiKey != "" {
			u, err := h.userService.GetUserByAPIKey(apiKey)
			if err == nil && u != nil {
				user = u
			}
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "İstek okunamadı", http.StatusBadRequest)
		return
	}

	sessionID := r.URL.Query().Get("sessionId")
	respBytes := h.ProcessRequest(r.Context(), body, user)

	// Eğer aktif bir SSE oturumu varsa yanıtı oraya da gönder
	if sessionID != "" {
		if val, ok := h.sessions.Load(sessionID); ok {
			ch := val.(chan []byte)
			select {
			case ch <- respBytes:
			default:
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBytes)
}

// ProcessRequest tek bir JSON-RPC mesajını işler ve JSON bayt dizisi döndürür.
func (h *MCPHandler) ProcessRequest(ctx context.Context, data []byte, user *model.User) []byte {
	var req jsonRPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		resp, _ := json.Marshal(jsonRPCResponse{
			JSONRPC: "2.0",
			Error:   map[string]interface{}{"code": -32700, "message": "Parse error"},
		})
		return resp
	}

	var res jsonRPCResponse
	res.JSONRPC = "2.0"
	res.ID = req.ID

	switch req.Method {
	case "initialize":
		res.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "linklik-mcp",
				"version": "1.0.0",
			},
		}

	case "notifications/initialized":
		return nil

	case "ping":
		res.Result = map[string]interface{}{}

	case "tools/list":
		if user == nil {
			res.Error = map[string]interface{}{
				"code":    -32000,
				"message": "Yetkilendirme gerekli. Lütfen X-API-KEY başlığı veya parametresi sağlayın.",
			}
			break
		}
		res.Result = map[string]interface{}{
			"tools": h.getToolsList(),
		}

	case "tools/call":
		if user == nil {
			res.Error = map[string]interface{}{
				"code":    -32000,
				"message": "Yetkilendirme gerekli. Lütfen X-API-KEY başlığı veya parametresi sağlayın.",
			}
			break
		}
		var callParams struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			res.Error = map[string]interface{}{"code": -32602, "message": "Geçersiz parametreler"}
		} else {
			toolResult, err := h.executeTool(callParams.Name, callParams.Arguments, user)
			if err != nil {
				res.Result = map[string]interface{}{
					"isError": true,
					"content": []map[string]interface{}{
						{"type": "text", "text": fmt.Sprintf("Hata: %v", err)},
					},
				}
			} else {
				res.Result = map[string]interface{}{
					"content": []map[string]interface{}{
						{"type": "text", "text": toolResult},
					},
				}
			}
		}

	default:
		res.Error = map[string]interface{}{"code": -32601, "message": "Metot bulunamadı"}
	}

	respBytes, _ := json.Marshal(res)
	return respBytes
}

func (h *MCPHandler) getToolsList() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "shorten_link",
			"description": "Yeni bir URL'i kısaltır. İsteğe bağlı olarak özel takma ad (custom_alias), tıklama sınırı (max_clicks), son kullanma tarihi ve aktiflik durumu belirlenebilir.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url":          map[string]interface{}{"type": "string", "description": "Kısaltılacak tam web adresi (örn: https://example.com)"},
					"custom_alias": map[string]interface{}{"type": "string", "description": "İsteğe bağlı özel kısa kod/takma ad"},
					"max_clicks":   map[string]interface{}{"type": "number", "description": "Maksimum tıklanma sınırı (0 veya boş = sınırsız)"},
					"expires_at":   map[string]interface{}{"type": "string", "description": "Son kullanma tarihi (örn: 2026-12-31T23:59:00Z)"},
					"is_active":    map[string]interface{}{"type": "boolean", "description": "Linkin başlangıç aktiflik durumu (varsayılan: true)"},
				},
				"required": []string{"url"},
			},
		},
		{
			"name":        "list_links",
			"description": "Kullanıcıya ait (veya admin ise sistemdeki) kısaltılmış linkleri sayfalanmış ve arama/filtreleme destekli listeler.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"page":   map[string]interface{}{"type": "number", "description": "Sayfa numarası (varsayılan: 1)"},
					"limit":  map[string]interface{}{"type": "number", "description": "Sayfa başına kayıt (varsayılan: 20)"},
					"search": map[string]interface{}{"type": "string", "description": "URL veya kısa kod içinde aranacak metin"},
					"status": map[string]interface{}{"type": "string", "enum": []string{"active", "inactive", "expired"}, "description": "Durum filtresi"},
				},
			},
		},
		{
			"name":        "get_link_analytics",
			"description": "Belirli bir kısa kodun toplam tıklanma, coğrafi dağılım (ülkeler), tarayıcı, işletim sistemi ve günlük tıklama istatistiklerini getirir.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"short_code": map[string]interface{}{"type": "string", "description": "İstatistikleri istenen kısa kod veya alias"},
				},
				"required": []string{"short_code"},
			},
		},
		{
			"name":        "toggle_link",
			"description": "Bir linkin aktiflik durumunu tersine çevirir (aktifse pasife alır, pasifse aktifleştirir).",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"short_code": map[string]interface{}{"type": "string", "description": "Aktif/pasif yapılacak kısa kod"},
				},
				"required": []string{"short_code"},
			},
		},
		{
			"name":        "reset_link_stats",
			"description": "Bir linkin tıklama sayacını 0 yapar ve tüm analitik geçmişini sıfırlar.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"short_code": map[string]interface{}{"type": "string", "description": "İstatistikleri sıfırlanacak kısa kod"},
				},
				"required": []string{"short_code"},
			},
		},
		{
			"name":        "delete_link",
			"description": "Kısaltılmış bir linki sistemden kalıcı olarak siler.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"short_code": map[string]interface{}{"type": "string", "description": "Silinecek kısa kod"},
				},
				"required": []string{"short_code"},
			},
		},
	}
}

func (h *MCPHandler) executeTool(name string, argsRaw json.RawMessage, user *model.User) (string, error) {
	switch name {
	case "shorten_link":
		var p struct {
			URL         string  `json:"url"`
			CustomAlias string  `json:"custom_alias"`
			MaxClicks   int64   `json:"max_clicks"`
			ExpiresAt   *string `json:"expires_at"`
			IsActive    *bool   `json:"is_active"`
		}
		if err := json.Unmarshal(argsRaw, &p); err != nil {
			return "", err
		}
		link, err := h.linkService.ShortenURL(p.URL, p.CustomAlias, p.MaxClicks, p.ExpiresAt, "", p.IsActive, user.ID)
		if err != nil {
			return "", err
		}
		shortURL := fmt.Sprintf("%s/%s", config.GlobalConfig.BaseURL, link.ShortCode)
		return fmt.Sprintf("Link başarıyla kısaltıldı!\nKısa URL: %s\nKısa Kod: %s\nHedef: %s", shortURL, link.ShortCode, link.OriginalURL), nil

	case "list_links":
		var p struct {
			Page   int    `json:"page"`
			Limit  int    `json:"limit"`
			Search string `json:"search"`
			Status string `json:"status"`
		}
		_ = json.Unmarshal(argsRaw, &p)
		if p.Page < 1 {
			p.Page = 1
		}
		if p.Limit < 1 {
			p.Limit = 20
		}
		links, total, err := h.linkService.GetPaginatedLinks(user.ID, user.Role == model.RoleSuperadmin, p.Page, p.Limit, p.Search, p.Status)
		if err != nil {
			return "", err
		}
		out, _ := json.MarshalIndent(map[string]interface{}{
			"total_count": total,
			"page":        p.Page,
			"links":       links,
		}, "", "  ")
		return string(out), nil

	case "get_link_analytics":
		var p struct {
			ShortCode string `json:"short_code"`
		}
		if err := json.Unmarshal(argsRaw, &p); err != nil || p.ShortCode == "" {
			return "", fmt.Errorf("short_code zorunludur")
		}
		link, err := h.linkService.GetLinkByShortCode(p.ShortCode)
		if err != nil || link == nil {
			return "", fmt.Errorf("link bulunamadı")
		}
		if user.Role != model.RoleSuperadmin && link.CreatedByID != user.ID {
			return "", fmt.Errorf("bu linkin istatistiklerini görüntüleme yetkiniz yok")
		}
		stats, err := h.analyticsService.GetStats(link.ID)
		if err != nil {
			return "", err
		}
		out, _ := json.MarshalIndent(stats, "", "  ")
		return string(out), nil

	case "toggle_link":
		var p struct {
			ShortCode string `json:"short_code"`
		}
		if err := json.Unmarshal(argsRaw, &p); err != nil || p.ShortCode == "" {
			return "", fmt.Errorf("short_code zorunludur")
		}
		newState, err := h.linkService.ToggleLinkActive(p.ShortCode, user.ID, user.Role)
		if err != nil {
			return "", err
		}
		durum := "Pasif"
		if newState {
			durum = "Aktif"
		}
		return fmt.Sprintf("Link (%s) durumu başarıyla güncellendi: %s", p.ShortCode, durum), nil

	case "reset_link_stats":
		var p struct {
			ShortCode string `json:"short_code"`
		}
		if err := json.Unmarshal(argsRaw, &p); err != nil || p.ShortCode == "" {
			return "", fmt.Errorf("short_code zorunludur")
		}
		if err := h.linkService.ResetLinkStats(p.ShortCode, user.ID, user.Role); err != nil {
			return "", err
		}
		return fmt.Sprintf("Link (%s) tıklama sayacı ve analitik verileri başarıyla sıfırlandı.", p.ShortCode), nil

	case "delete_link":
		var p struct {
			ShortCode string `json:"short_code"`
		}
		if err := json.Unmarshal(argsRaw, &p); err != nil || p.ShortCode == "" {
			return "", fmt.Errorf("short_code zorunludur")
		}
		if err := h.linkService.DeleteLinkByShortCode(p.ShortCode, user.ID, user.Role); err != nil {
			return "", err
		}
		return fmt.Sprintf("Link (%s) başarıyla silindi.", p.ShortCode), nil

	default:
		return "", fmt.Errorf("bilinmeyen araç: %s", name)
	}
}

// RunStdio Claude Desktop gibi yerel araçlar için stdin/stdout üzerinden MCP protokolünü çalıştırır.
func (h *MCPHandler) RunStdio(user *model.User) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		resp := h.ProcessRequest(context.Background(), line, user)
		if len(resp) > 0 {
			os.Stdout.Write(resp)
			os.Stdout.WriteString("\n")
		}
	}
}
