package handler

import (
	"encoding/json"
	"linklik/internal/model"
	"net/http"
	"regexp"
)

// shortCodeRegex sistemde üretilebilen tüm kısa kod biçimlerini kapsar:
// rastgele Base62 kodları ve özel takma adlar (harf, rakam, tire, alt çizgi).
// Güvenlik: URL yol parametresi loglara yazılmadan ve DB sorgusuna gitmeden önce
// doğrulanır; kontrol karakterli (%0A vb.) girdilerle log enjeksiyonu ve anlamsız
// veritabanı sorguları engellenir. Mevcut tüm kodlar bu biçime uyar; davranış değişmez.
var shortCodeRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// isValidShortCode kısa kod parametresinin geçerli sistem biçiminde olup olmadığını döner.
func isValidShortCode(code string) bool {
	return shortCodeRegex.MatchString(code)
}

// respondJSON standart biçimlendirilmiş bir JSON yanıtı döndürür.
func respondJSON(w http.ResponseWriter, status int, success bool, data interface{}, err string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(model.APIResponse{
		Success: success,
		Data:    data,
		Error:   err,
	})
}

// respondSuccess başarılı bir API yanıtı döndürür.
func respondSuccess(w http.ResponseWriter, data interface{}) {
	respondJSON(w, http.StatusOK, true, data, "")
}

// respondCreated başarılı bir oluşturma (Created) API yanıtı döndürür.
func respondCreated(w http.ResponseWriter, data interface{}) {
	respondJSON(w, http.StatusCreated, true, data, "")
}

// respondError hata içeren bir API yanıtı döndürür.
func respondError(w http.ResponseWriter, status int, err string) {
	respondJSON(w, status, false, nil, err)
}
