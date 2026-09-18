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

type mcpSession struct {
	msgChan chan []byte
	user    *model.User
	apiKey  string
}

// MCPHandler Model Context Protocol (MCP) isteklerini karşılayan handler'dır.
type MCPHandler struct {
	linkService      *service.LinkService
	analyticsService *service.AnalyticsService
	userService      *service.UserService
	sessions         sync.Map // sessionId -> *mcpSession
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
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// extractUser istekten (Context, Header veya Query parametresi) kullanıcıyı ve API anahtarını ayıklar.
func (h *MCPHandler) extractUser(r *http.Request) (*model.User, string) {
	// 1. Context'ten (middleware oturumu veya API anahtarı)
	if user := middleware.GetUserFromContext(r.Context()); user != nil {
		return user, user.APIKey
	}

	// 2. X-API-KEY başlığı
	apiKey := r.Header.Get("X-API-KEY")
	if apiKey == "" {
		apiKey = r.Header.Get("x-api-key")
	}

	// 3. Authorization başlığı (Bearer <key> veya doğrudan <key>)
	if apiKey == "" {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			apiKey = strings.TrimSpace(authHeader[7:])
		} else if authHeader != "" && !strings.Contains(authHeader, " ") {
			// Doğrudan anahtar formatında girilmişse
			apiKey = authHeader
		}
	}

	// 4. URL Query parametreleri (?api_key=, ?apiKey=, ?token=, ?key=)
	if apiKey == "" {
		apiKey = r.URL.Query().Get("api_key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("apiKey")
		}
		if apiKey == "" {
			apiKey = r.URL.Query().Get("token")
		}
		if apiKey == "" {
			apiKey = r.URL.Query().Get("key")
		}
	}

	if apiKey != "" && h.userService != nil {
		u, err := h.userService.GetUserByAPIKey(apiKey)
		if err == nil && u != nil {
			return u, apiKey
		}
	}

	return nil, apiKey
}

// HandleUnified tek bir standart URL (/mcp) üzerinden hem SSE hem Streamable HTTP desteği sunar.
// GET /mcp (Accept: text/event-stream) -> SSE akışını başlatır.
// GET /mcp (Probe/Normal) -> Anında 200 OK ile servis durumunu döner (Gemini/istemci URL doğrulaması için).
// POST /mcp -> Doğrudan JSON-RPC mesajını işler (Streamable HTTP).
// OPTIONS /mcp -> CORS preflight yanıtı döner.
// HEAD /mcp -> Anında 200 OK döner.
func (h *MCPHandler) HandleUnified(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	} else {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, HEAD")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-KEY, Accept, *")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Type, Authorization, X-API-KEY, Location")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == http.MethodPost {
		h.HandleMessage(w, r)
		return
	}

	// GET istekleri:
	// Eğer istemci SSE istiyorsa (Accept: text/event-stream) SSE akışını başlat
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/event-stream") {
		h.HandleSSE(w, r)
		return
	}

	// Normal HTTP GET: Gemini ve diğer araçların bağlantı testi (probe) için hemen 200 OK JSON yanıtı döner.
	user, apiKey := h.extractUser(r)
	authStatus := "authenticated"
	if user == nil {
		authStatus = "unauthenticated"
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "ok",
		"service":         "linklik-mcp",
		"protocolVersion": "2024-11-05",
		"authentication":  authStatus,
		"hasKey":          apiKey != "",
		"transports":      []string{"sse", "streamable-http"},
		"endpoints": map[string]string{
			"unified": "/mcp",
			"sse":     "/mcp/sse",
			"message": "/mcp/message",
		},
		"capabilities": map[string]interface{}{
			"tools":     true,
			"resources": true,
			"prompts":   true,
		},
	})
}

// HandleSSE AI istemcileri (Claude Code, Cursor, Gemini, Claude Desktop vb.) için SSE akışını başlatır.
func (h *MCPHandler) HandleSSE(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	} else {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, HEAD")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-KEY, Accept, *")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Type, Authorization, X-API-KEY, Location")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE desteklenmiyor", http.StatusInternalServerError)
		return
	}

	user, apiKey := h.extractUser(r)

	sessionBytes := make([]byte, 16)
	_, _ = rand.Read(sessionBytes)
	sessionID := hex.EncodeToString(sessionBytes)

	sess := &mcpSession{
		msgChan: make(chan []byte, 64),
		user:    user,
		apiKey:  apiKey,
	}
	h.sessions.Store(sessionID, sess)
	defer func() {
		h.sessions.Delete(sessionID)
		close(sess.msgChan)
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// İstemciye mesaj gönderim endpoint URL'ini bildir (api_key parametresini koru)
	queryParts := []string{fmt.Sprintf("sessionId=%s", sessionID)}
	if apiKey != "" {
		queryParts = append(queryParts, fmt.Sprintf("api_key=%s", apiKey))
	} else if r.URL.RawQuery != "" {
		queryParts = append(queryParts, r.URL.RawQuery)
	}
	endpointQuery := strings.Join(queryParts, "&")

	var endpointURL string
	if config.GlobalConfig != nil && config.GlobalConfig.BaseURL != "" {
		baseURL := strings.TrimRight(config.GlobalConfig.BaseURL, "/")
		endpointURL = fmt.Sprintf("%s/mcp/message?%s", baseURL, endpointQuery)
	} else {
		endpointURL = fmt.Sprintf("/mcp/message?%s", endpointQuery)
	}

	_, _ = fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
	flusher.Flush()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg, ok := <-sess.msgChan:
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
	origin := r.Header.Get("Origin")
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	} else {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, HEAD")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-KEY, Accept, *")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Type, Authorization, X-API-KEY, Location")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	user, _ := h.extractUser(r)
	sessionID := r.URL.Query().Get("sessionId")
	var activeSession *mcpSession

	if sessionID != "" {
		if val, ok := h.sessions.Load(sessionID); ok {
			activeSession = val.(*mcpSession)
			if user == nil && activeSession.user != nil {
				user = activeSession.user
			}
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "İstek okunamadı", http.StatusBadRequest)
		return
	}

	respBytes := h.ProcessRequest(r.Context(), body, user)

	// Eğer aktif bir SSE oturumu varsa yanıtı SSE akışına da ilet
	if activeSession != nil && len(respBytes) > 0 {
		select {
		case activeSession.msgChan <- respBytes:
		default:
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if len(respBytes) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}
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

	// JSON-RPC bildirimi (notification) kontrolü
	isNotification := req.ID == nil

	var res jsonRPCResponse
	res.JSONRPC = "2.0"
	res.ID = req.ID

	switch req.Method {
	case "initialize":
		var initParams struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &initParams)
		protoVer := "2024-11-05"
		if initParams.ProtocolVersion != "" {
			protoVer = initParams.ProtocolVersion
		}

		res.Result = map[string]interface{}{
			"protocolVersion": protoVer,
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{
					"listChanged": false,
				},
				"resources": map[string]interface{}{
					"subscribe":   false,
					"listChanged": false,
				},
				"prompts": map[string]interface{}{
					"listChanged": false,
				},
			},
			"serverInfo": map[string]interface{}{
				"name":    "linklik",
				"version": "1.0.0",
			},
			"instructions": "Linklik URL Kısaltma ve Analitik MCP Servisi. Linkleri listelemek, yeni kısa link oluşturmak, link güncellemek veya silmek ve tıklama analitiklerini görüntülemek için bu araçları kullanabilirsiniz.",
		}

	case "notifications/initialized", "initialized":
		return nil

	case "notifications/cancelled", "$/cancelRequest":
		return nil

	case "ping":
		res.Result = map[string]interface{}{}

	case "resources/list":
		res.Result = map[string]interface{}{
			"resources": []interface{}{},
		}

	case "resources/templates/list":
		res.Result = map[string]interface{}{
			"resourceTemplates": []interface{}{},
		}

	case "prompts/list":
		res.Result = map[string]interface{}{
			"prompts": []interface{}{},
		}

	case "completion/complete":
		res.Result = map[string]interface{}{
			"completion": map[string]interface{}{
				"values":  []interface{}{},
				"hasMore": false,
			},
		}

	case "logging/setLevel":
		res.Result = map[string]interface{}{}

	case "tools/list":
		res.Result = map[string]interface{}{
			"tools": h.getToolsList(),
		}

	case "tools/call":
		var callParams struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			res.Error = map[string]interface{}{"code": -32602, "message": "Geçersiz parametreler"}
			break
		}

		if user == nil {
			res.Result = map[string]interface{}{
				"isError": true,
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "Yetkilendirme Hatası: Bu işlemi gerçekleştirmek için geçerli bir Linklik API anahtarı gereklidir. Lütfen MCP sunucu adresine ?api_key= parametresi veya Authorization: Bearer başlığı ekleyin.",
					},
				},
			}
			break
		}

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
				"isError": false,
				"content": []map[string]interface{}{
					{"type": "text", "text": toolResult},
				},
			}
		}

	default:
		if isNotification {
			return nil
		}
		res.Error = map[string]interface{}{
			"code":    -32601,
			"message": fmt.Sprintf("Metot desteklenmiyor: %s", req.Method),
		}
	}

	if isNotification {
		return nil
	}

	respBytes, _ := json.Marshal(res)
	return respBytes
}

func (h *MCPHandler) getToolsList() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "shorten_link",
			"description": "Yeni bir URL kısaltır. Özel takma ad (custom_alias), tıklama sınırı (max_clicks), son kullanma tarihi (expires_at) ve erişim şifresi (password) belirlenebilir.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url":          map[string]interface{}{"type": "string", "description": "Kısaltılacak tam web adresi (örn: https://example.com)"},
					"custom_alias": map[string]interface{}{"type": "string", "description": "İsteğe bağlı özel kısa kod/takma ad"},
					"max_clicks":   map[string]interface{}{"type": "integer", "description": "Maksimum tıklanma sınırı (0 veya boş = sınırsız)"},
					"expires_at":   map[string]interface{}{"type": "string", "description": "Son kullanma tarihi (ISO 8601, örn: 2026-12-31T23:59:00Z)"},
					"password":     map[string]interface{}{"type": "string", "description": "Ziyaretçilerden istenecek erişim şifresi (opsiyonel)"},
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
					"page":   map[string]interface{}{"type": "integer", "description": "Sayfa numarası (varsayılan: 1)"},
					"limit":  map[string]interface{}{"type": "integer", "description": "Sayfa başına kayıt (varsayılan: 20)"},
					"search": map[string]interface{}{"type": "string", "description": "URL veya kısa kod içinde aranacak metin"},
					"status": map[string]interface{}{"type": "string", "enum": []string{"all", "active", "inactive", "expired"}, "description": "Durum filtresi"},
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
			Password    string  `json:"password"`
			IsActive    *bool   `json:"is_active"`
		}
		if err := json.Unmarshal(argsRaw, &p); err != nil {
			return "", err
		}
		link, err := h.linkService.ShortenURL(p.URL, p.CustomAlias, p.MaxClicks, p.ExpiresAt, p.Password, p.IsActive, user.ID)
		if err != nil {
			return "", err
		}
		shortURL := fmt.Sprintf("%s/%s", strings.TrimRight(config.GlobalConfig.BaseURL, "/"), link.ShortCode)
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
