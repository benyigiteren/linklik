package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"linklik/internal/db"
	"linklik/internal/model"
	"strings"
	"time"
)

// LinkRepository link işlemlerini gerçekleştiren veritabanı katmanıdır.
type LinkRepository struct {
	db *sql.DB
}

// NewLinkRepository yeni bir LinkRepository örneği döndürür.
func NewLinkRepository() *LinkRepository {
	return &LinkRepository{db: db.Db}
}

// Create yeni bir kısaltılmış linki veritabanına kaydeder.
func (r *LinkRepository) Create(l *model.Link) error {
	query := `INSERT INTO links (original_url, short_code, custom_alias, created_by_id, click_count, max_clicks, is_active, expires_at, password_hash, created_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")

	var alias interface{}
	if l.CustomAlias != "" {
		alias = l.CustomAlias
	}

	isActiveInt := 1
	if !l.IsActive {
		isActiveInt = 0
	}

	var expiresAtStr interface{}
	if l.ExpiresAt != nil {
		expiresAtStr = l.ExpiresAt.Format("2006-01-02 15:04:05")
	}

	var passwordHash interface{}
	if l.PasswordHash != "" {
		passwordHash = l.PasswordHash
	}

	res, err := r.db.Exec(query, l.OriginalURL, l.ShortCode, alias, l.CreatedByID, l.ClickCount, l.MaxClicks, isActiveInt, expiresAtStr, passwordHash, nowStr)
	if err != nil {
		return err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	l.ID = id
	l.CreatedAt = now
	l.HasPassword = l.PasswordHash != ""
	return nil
}

// GetByShortCode kısa koduna veya özel takma adına göre linki getirir.
func (r *LinkRepository) GetByShortCode(shortCode string) (*model.Link, error) {
	query := `SELECT id, original_url, short_code, custom_alias, created_by_id, click_count, max_clicks, is_active, expires_at, password_hash, created_at 
	          FROM links 
	          WHERE short_code = ? OR custom_alias = ?`

	l := &model.Link{}
	var alias, expiresAtStr, passHash sql.NullString
	var isActiveInt int
	err := r.db.QueryRow(query, shortCode, shortCode).Scan(
		&l.ID, &l.OriginalURL, &l.ShortCode, &alias, &l.CreatedByID, &l.ClickCount, &l.MaxClicks,
		&isActiveInt, &expiresAtStr, &passHash, &l.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if alias.Valid {
		l.CustomAlias = alias.String
	}
	l.IsActive = isActiveInt == 1
	if expiresAtStr.Valid && expiresAtStr.String != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", expiresAtStr.String); err == nil {
			l.ExpiresAt = &t
		} else if t, err := time.Parse(time.RFC3339, expiresAtStr.String); err == nil {
			l.ExpiresAt = &t
		}
	}
	if passHash.Valid {
		l.PasswordHash = passHash.String
		l.HasPassword = l.PasswordHash != ""
	}
	return l, nil
}

// IncrementClick linkin tıklanma sayacını 1 artırır.
func (r *LinkRepository) IncrementClick(id int64) error {
	query := `UPDATE links SET click_count = click_count + 1 WHERE id = ?`
	_, err := r.db.Exec(query, id)
	return err
}

// IncrementClickIfBelowLimit tıklanma sayacını yalnızca limitin altındaysa atomik olarak artırır.
func (r *LinkRepository) IncrementClickIfBelowLimit(id int64) (bool, error) {
	query := `UPDATE links SET click_count = click_count + 1
	          WHERE id = ? AND (max_clicks <= 0 OR click_count < max_clicks)`
	res, err := r.db.Exec(query, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// ResetStats linkin tıklanma sayısını 0 yapar ve ilişkili tüm analitik kayıtlarını siler.
func (r *LinkRepository) ResetStats(id int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM analytics WHERE link_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE links SET click_count = 0 WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// ToggleActive linkin aktiflik durumunu tersine çevirir ve yeni durumu döner.
func (r *LinkRepository) ToggleActive(id int64) (bool, error) {
	query := `UPDATE links SET is_active = CASE WHEN is_active = 1 THEN 0 ELSE 1 END WHERE id = ?`
	if _, err := r.db.Exec(query, id); err != nil {
		return false, err
	}

	var isActiveInt int
	if err := r.db.QueryRow(`SELECT is_active FROM links WHERE id = ?`, id).Scan(&isActiveInt); err != nil {
		return false, err
	}
	return isActiveInt == 1, nil
}

// GetByUserID belirli bir kullanıcının kısalttığı linkleri getirir.
func (r *LinkRepository) GetByUserID(userID int64) ([]*model.Link, error) {
	query := `SELECT id, original_url, short_code, custom_alias, created_by_id, click_count, max_clicks, is_active, expires_at, password_hash, created_at 
	          FROM links 
	          WHERE created_by_id = ? 
	          ORDER BY created_at DESC`

	return r.scanLinks(query, userID)
}

// GetAll sistemdeki tüm linkleri listeler (Superadmin için).
func (r *LinkRepository) GetAll() ([]*model.Link, error) {
	query := `SELECT id, original_url, short_code, custom_alias, created_by_id, click_count, max_clicks, is_active, expires_at, password_hash, created_at 
	          FROM links 
	          ORDER BY created_at DESC`

	return r.scanLinks(query)
}

// GetPaginated sayfalama, arama ve durum filtresi ile linkleri listeler.
func (r *LinkRepository) GetPaginated(userID int64, isSuperadmin bool, page, limit int, search, status string) ([]*model.Link, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var whereClauses []string
	var args []interface{}

	if !isSuperadmin {
		whereClauses = append(whereClauses, "created_by_id = ?")
		args = append(args, userID)
	}

	search = strings.TrimSpace(search)
	if search != "" {
		likePattern := "%" + search + "%"
		whereClauses = append(whereClauses, "(original_url LIKE ? OR short_code LIKE ? OR custom_alias LIKE ?)")
		args = append(args, likePattern, likePattern, likePattern)
	}

	nowStr := time.Now().Format("2006-01-02 15:04:05")
	switch strings.ToLower(status) {
	case "active":
		whereClauses = append(whereClauses, "is_active = 1 AND (expires_at IS NULL OR expires_at > ?)")
		args = append(args, nowStr)
	case "inactive":
		whereClauses = append(whereClauses, "is_active = 0")
	case "expired":
		whereClauses = append(whereClauses, "expires_at IS NOT NULL AND expires_at <= ?")
		args = append(args, nowStr)
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Toplam kayıt sayısını hesapla
	countQuery := "SELECT COUNT(*) FROM links" + whereSQL
	var totalCount int64
	if err := r.db.QueryRow(countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	// Sayfalanmış kayıtları çek
	selectSQL := fmt.Sprintf(`SELECT id, original_url, short_code, custom_alias, created_by_id, click_count, max_clicks, is_active, expires_at, password_hash, created_at 
	                         FROM links %s ORDER BY created_at DESC LIMIT ? OFFSET ?`, whereSQL)

	argsWithLimit := append(args, limit, offset)
	links, err := r.scanLinks(selectSQL, argsWithLimit...)
	if err != nil {
		return nil, 0, err
	}

	return links, totalCount, nil
}

// Update link bilgilerini günceller.
func (r *LinkRepository) Update(l *model.Link) error {
	query := `UPDATE links 
	          SET original_url = ?, short_code = ?, custom_alias = ?, max_clicks = ?, is_active = ?, expires_at = ?, password_hash = ? 
	          WHERE id = ?`

	var alias interface{}
	if l.CustomAlias != "" {
		alias = l.CustomAlias
	}

	isActiveInt := 1
	if !l.IsActive {
		isActiveInt = 0
	}

	var expiresAtStr interface{}
	if l.ExpiresAt != nil {
		expiresAtStr = l.ExpiresAt.Format("2006-01-02 15:04:05")
	}

	var passHash interface{}
	if l.PasswordHash != "" {
		passHash = l.PasswordHash
	}

	_, err := r.db.Exec(query, l.OriginalURL, l.ShortCode, alias, l.MaxClicks, isActiveInt, expiresAtStr, passHash, l.ID)
	return err
}

// Delete linki siler.
func (r *LinkRepository) Delete(id int64) error {
	query := `DELETE FROM links WHERE id = ?`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *LinkRepository) scanLinks(query string, args ...interface{}) ([]*model.Link, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []*model.Link
	for rows.Next() {
		l := &model.Link{}
		var alias, expiresAtStr, passHash sql.NullString
		var isActiveInt int
		err := rows.Scan(
			&l.ID, &l.OriginalURL, &l.ShortCode, &alias, &l.CreatedByID, &l.ClickCount, &l.MaxClicks,
			&isActiveInt, &expiresAtStr, &passHash, &l.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		if alias.Valid {
			l.CustomAlias = alias.String
		}
		l.IsActive = isActiveInt == 1
		if expiresAtStr.Valid && expiresAtStr.String != "" {
			if t, err := time.Parse("2006-01-02 15:04:05", expiresAtStr.String); err == nil {
				l.ExpiresAt = &t
			} else if t, err := time.Parse(time.RFC3339, expiresAtStr.String); err == nil {
				l.ExpiresAt = &t
			}
		}
		if passHash.Valid {
			l.PasswordHash = passHash.String
			l.HasPassword = l.PasswordHash != ""
		}
		links = append(links, l)
	}
	return links, nil
}
