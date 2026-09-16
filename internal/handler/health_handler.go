package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// HealthHandler sistem sağlık durumu kontrollerini yürütür.
type HealthHandler struct {
	db *sql.DB
}

// NewHealthHandler yeni bir HealthHandler oluşturur.
func NewHealthHandler(database *sql.DB) *HealthHandler {
	return &HealthHandler{db: database}
}

// Healthz Liveness kontrolü (Konteyner ve servis ayakta mı?)
func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// Readyz Readiness kontrolü (Veritabanı bağlantısı sağlıklı ve yazılabilir mi?)
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := h.db.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "unavailable",
			"error":  err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":   "ready",
		"database": "connected",
	})
}
