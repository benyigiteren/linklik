package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPHandler_Initialize(t *testing.T) {
	h := NewMCPHandler(nil, nil, nil)

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"clientInfo": map[string]string{
				"name":    "TestClient",
				"version": "1.0",
			},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleMessage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("expected no error, got %v", resp.Error)
	}

	resMap, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", resp.Result)
	}

	if resMap["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocolVersion 2024-11-05, got %v", resMap["protocolVersion"])
	}
}

func TestMCPHandler_ToolsList(t *testing.T) {
	h := NewMCPHandler(nil, nil, nil)

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleMessage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	resMap, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", resp.Result)
	}

	tools, ok := resMap["tools"].([]interface{})
	if !ok || len(tools) != 6 {
		t.Fatalf("expected 6 tools, got %v", len(tools))
	}
}

func TestMCPHandler_UnauthenticatedToolCall(t *testing.T) {
	h := NewMCPHandler(nil, nil, nil)

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "list_links",
			"arguments": map[string]interface{}{},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleMessage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	resMap, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", resp.Result)
	}

	if isErr, ok := resMap["isError"].(bool); !ok || !isErr {
		t.Fatalf("expected isError to be true for unauthenticated call")
	}
}

func TestMCPHandler_Notifications(t *testing.T) {
	h := NewMCPHandler(nil, nil, nil)

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleMessage(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 Accepted for notifications, got %d", w.Code)
	}
}

func TestMCPHandler_UnifiedProbe(t *testing.T) {
	h := NewMCPHandler(nil, nil, nil)

	// Test GET probe without Accept: text/event-stream
	req := httptest.NewRequest("GET", "/mcp", nil)
	w := httptest.NewRecorder()
	h.HandleUnified(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET probe, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "linklik-mcp") {
		t.Fatalf("expected linklik-mcp in response, got %s", w.Body.String())
	}

	// Test OPTIONS preflight
	optReq := httptest.NewRequest("OPTIONS", "/mcp", nil)
	optReq.Header.Set("Origin", "https://gemini.google.com")
	optW := httptest.NewRecorder()
	h.HandleUnified(optW, optReq)

	if optW.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content for OPTIONS, got %d", optW.Code)
	}
	if optW.Header().Get("Access-Control-Allow-Origin") != "https://gemini.google.com" {
		t.Fatalf("expected CORS origin to be https://gemini.google.com, got %s", optW.Header().Get("Access-Control-Allow-Origin"))
	}

	// Test HEAD
	headReq := httptest.NewRequest("HEAD", "/mcp", nil)
	headW := httptest.NewRecorder()
	h.HandleUnified(headW, headReq)

	if headW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HEAD, got %d", headW.Code)
	}
}

func TestMCPHandler_ExtractUser_AuthHeaders(t *testing.T) {
	h := NewMCPHandler(nil, nil, nil)

	// Case 1: Bearer prefix
	req1 := httptest.NewRequest("GET", "/mcp", nil)
	req1.Header.Set("Authorization", "Bearer my_secret_token_123")
	_, key1 := h.extractUser(req1)
	if key1 != "my_secret_token_123" {
		t.Fatalf("expected my_secret_token_123, got %s", key1)
	}

	// Case 2: Raw token without Bearer prefix
	req2 := httptest.NewRequest("GET", "/mcp", nil)
	req2.Header.Set("Authorization", "my_secret_token_456")
	_, key2 := h.extractUser(req2)
	if key2 != "my_secret_token_456" {
		t.Fatalf("expected my_secret_token_456, got %s", key2)
	}

	// Case 3: X-API-KEY header
	req3 := httptest.NewRequest("GET", "/mcp", nil)
	req3.Header.Set("X-API-KEY", "my_secret_token_789")
	_, key3 := h.extractUser(req3)
	if key3 != "my_secret_token_789" {
		t.Fatalf("expected my_secret_token_789, got %s", key3)
	}

	// Case 4: Query param ?api_key=
	req4 := httptest.NewRequest("GET", "/mcp?api_key=my_secret_token_query", nil)
	_, key4 := h.extractUser(req4)
	if key4 != "my_secret_token_query" {
		t.Fatalf("expected my_secret_token_query, got %s", key4)
	}
}

