package repository

import (
	"database/sql"
	"errors"
	"linklik/internal/db"
	"linklik/internal/model"
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
	query := `INSERT INTO links (original_url, short_code, custom_alias, created_by_id, click_count, max_clicks, created_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?)`
	
	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")
	var alias interface{}
	if l.CustomAlias != "" {
		alias = l.CustomAlias
	}
	res, err := r.db.Exec(query, l.OriginalURL, l.ShortCode, alias, l.CreatedByID, l.ClickCount, l.MaxClicks, nowStr)
	if err != nil {
		return err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	l.ID = id
	l.CreatedAt = now
	return nil
}

// GetByShortCode kısa koduna veya özel takma adına göre linki getirir.
func (r *LinkRepository) GetByShortCode(shortCode string) (*model.Link, error) {
	query := `SELECT id, original_url, short_code, custom_alias, created_by_id, click_count, max_clicks, created_at 
	          FROM links 
	          WHERE short_code = ? OR custom_alias = ?`
	
	l := &model.Link{}
	var alias sql.NullString
	err := r.db.QueryRow(query, shortCode, shortCode).Scan(
		&l.ID, &l.OriginalURL, &l.ShortCode, &alias, &l.CreatedByID, &l.ClickCount, &l.MaxClicks, &l.CreatedAt,
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
	return l, nil
}

// IncrementClick linkin tıklanma sayacını 1 artırır.
func (r *LinkRepository) IncrementClick(id int64) error {
	query := `UPDATE links SET click_count = click_count + 1 WHERE id = ?`
	_, err := r.db.Exec(query, id)
	return err
}

// IncrementClickIfBelowLimit tıklanma sayacını yalnızca limitin altındaysa atomik olarak artırır.
// Güvenlik: Okuma-sonra-yazma (TOCTOU) yarışını önler; eşzamanlı istekler tıklama
// limitini aşamaz. Sayaç artırıldıysa true, limite ulaşılmışsa false döner.
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

// GetByUserID belirli bir kullanıcının kısalttığı linkleri getirir.
func (r *LinkRepository) GetByUserID(userID int64) ([]*model.Link, error) {
	query := `SELECT id, original_url, short_code, custom_alias, created_by_id, click_count, max_clicks, created_at 
	          FROM links 
	          WHERE created_by_id = ? 
	          ORDER BY created_at DESC`
	
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []*model.Link
	for rows.Next() {
		l := &model.Link{}
		var alias sql.NullString
		err := rows.Scan(&l.ID, &l.OriginalURL, &l.ShortCode, &alias, &l.CreatedByID, &l.ClickCount, &l.MaxClicks, &l.CreatedAt)
		if err != nil {
			return nil, err
		}
		if alias.Valid {
			l.CustomAlias = alias.String
		}
		links = append(links, l)
	}
	return links, nil
}

// GetAll sistemdeki tüm linkleri listeler (Superadmin için).
func (r *LinkRepository) GetAll() ([]*model.Link, error) {
	query := `SELECT id, original_url, short_code, custom_alias, created_by_id, click_count, max_clicks, created_at 
	          FROM links 
	          ORDER BY created_at DESC`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []*model.Link
	for rows.Next() {
		l := &model.Link{}
		var alias sql.NullString
		err := rows.Scan(&l.ID, &l.OriginalURL, &l.ShortCode, &alias, &l.CreatedByID, &l.ClickCount, &l.MaxClicks, &l.CreatedAt)
		if err != nil {
			return nil, err
		}
		if alias.Valid {
			l.CustomAlias = alias.String
		}
		links = append(links, l)
	}
	return links, nil
}

// Update link bilgilerini günceller (Dışarıdan veya panelden düzenleme için).
func (r *LinkRepository) Update(l *model.Link) error {
	query := `UPDATE links SET original_url = ?, short_code = ?, custom_alias = ?, max_clicks = ? WHERE id = ?`
	var alias interface{}
	if l.CustomAlias != "" {
		alias = l.CustomAlias
	}
	_, err := r.db.Exec(query, l.OriginalURL, l.ShortCode, alias, l.MaxClicks, l.ID)
	return err
}

// Delete linki siler. Güvenlik için silen kişinin sahibi veya admin olması kontrolü üst katmanda yapılır.
func (r *LinkRepository) Delete(id int64) error {
	query := `DELETE FROM links WHERE id = ?`
	_, err := r.db.Exec(query, id)
	return err
}
