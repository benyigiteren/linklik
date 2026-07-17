package model

import "time"

// Rol tanımları
const (
	RoleSuperadmin = "superadmin"
	RoleMember     = "member"
)

// User veritabanındaki kullanıcı kaydını temsil eder.
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"` // "superadmin" veya "member"
	APIKey       string    `json:"api_key"`
	CreatedAt    time.Time `json:"created_at"`
}

// Link kısaltılmış bir link kaydını temsil eder.
type Link struct {
	ID          int64     `json:"id"`
	OriginalURL string    `json:"original_url"`
	ShortCode   string    `json:"short_code"`
	CustomAlias string    `json:"custom_alias,omitempty"`
	CreatedByID int64     `json:"created_by_id"`
	ClickCount  int64     `json:"click_count"`
	MaxClicks   int64     `json:"max_clicks"` // 0 veya negatif sınırsız anlamına gelir
	CreatedAt   time.Time `json:"created_at"`
}

// Analytics bir linke yapılan yönlendirme/tıklama verisini temsil eder.
type Analytics struct {
	ID        int64     `json:"id"`
	LinkID    int64     `json:"link_id"`
	IPAddress string    `json:"ip_address"`
	Referrer  string    `json:"referrer"`
	UserAgent string    `json:"user_agent"`
	Browser   string    `json:"browser"`
	OS        string    `json:"os"`
	Country   string    `json:"country"`
	ClickedAt time.Time `json:"clicked_at"`
}

// APIResponse standart API yanıt formatıdır.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SetupRequest ilk kurulum için istek gövdesidir.
type SetupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginRequest kullanıcı girişi için istek gövdesidir.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ShortenRequest link kısaltmak için gelen istek gövdesidir.
type ShortenRequest struct {
	URL         string `json:"url"`
	CustomAlias string `json:"custom_alias,omitempty"`
	MaxClicks   int64  `json:"max_clicks,omitempty"`
}

// LinkUpdateRequest var olan linki düzenlemek için gönderilen istek gövdesidir.
type LinkUpdateRequest struct {
	URL         string `json:"url"`
	CustomAlias string `json:"custom_alias,omitempty"`
	MaxClicks   int64  `json:"max_clicks"`
}

// UserCreateRequest yeni üye oluşturmak için admin tarafından gönderilen istek gövdesidir.
type UserCreateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LinkStats bir linkin detaylı analitik istatistiklerinin özetidir.
type LinkStats struct {
	TotalClicks int64          `json:"total_clicks"`
	Countries   map[string]int `json:"countries"`
	Browsers    map[string]int `json:"browsers"`
	OS          map[string]int `json:"os"`
	Referrers   map[string]int `json:"referrers"`
	DailyClicks map[string]int `json:"daily_clicks"`
}

