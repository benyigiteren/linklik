package repository

import (
	"database/sql"
	"errors"
	"linklik/internal/db"
	"linklik/internal/model"
	"time"
)

// UserRepository kullanıcı işlemlerini gerçekleştiren veritabanı katmanıdır.
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository yeni bir UserRepository örneği döndürür.
func NewUserRepository() *UserRepository {
	return &UserRepository{db: db.Db}
}

// Create yeni bir kullanıcıyı veritabanına kaydeder.
func (r *UserRepository) Create(u *model.User) error {
	query := `INSERT INTO users (username, password_hash, role, api_key, created_at)
	          VALUES (?, ?, ?, ?, ?)`

	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")
	res, err := r.db.Exec(query, u.Username, u.PasswordHash, u.Role, u.APIKey, nowStr)
	if err != nil {
		return err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	u.ID = id
	u.CreatedAt = now
	return nil
}

// CreateIfNone kullanıcıyı yalnızca tabloda hiç kayıt yoksa ekler (atomik).
// Güvenlik: kurulum (setup) TOCTOU yarış durumunu önler; iki eşzamanlı istekten
// yalnızca biri başarılı olur. Başarılıysa true, aksi halde false döner.
func (r *UserRepository) CreateIfNone(u *model.User) (bool, error) {
	query := `INSERT INTO users (username, password_hash, role, api_key, created_at)
	          SELECT ?, ?, ?, ?, ?
	          WHERE NOT EXISTS (SELECT 1 FROM users)`

	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")
	res, err := r.db.Exec(query, u.Username, u.PasswordHash, u.Role, u.APIKey, nowStr)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil
	}
	id, err := res.LastInsertId()
	if err != nil {
		return false, err
	}
	u.ID = id
	u.CreatedAt = now
	return true, nil
}

// GetByID ID'ye göre kullanıcıyı getirir.
func (r *UserRepository) GetByID(id int64) (*model.User, error) {
	query := `SELECT id, username, password_hash, role, api_key, created_at FROM users WHERE id = ?`
	u := &model.User{}
	err := r.db.QueryRow(query, id).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.APIKey, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

// GetByUsername kullanıcı adına göre kullanıcıyı getirir.
func (r *UserRepository) GetByUsername(username string) (*model.User, error) {
	query := `SELECT id, username, password_hash, role, api_key, created_at FROM users WHERE username = ?`
	u := &model.User{}
	err := r.db.QueryRow(query, username).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.APIKey, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

// GetByAPIKey API anahtarına göre kullanıcıyı getirir.
func (r *UserRepository) GetByAPIKey(apiKey string) (*model.User, error) {
	query := `SELECT id, username, password_hash, role, api_key, created_at FROM users WHERE api_key = ?`
	u := &model.User{}
	err := r.db.QueryRow(query, apiKey).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.APIKey, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

// GetCount sistemdeki toplam kullanıcı sayısını döner (Kurulum kontrolü için).
func (r *UserRepository) GetCount() (int64, error) {
	query := `SELECT COUNT(*) FROM users`
	var count int64
	err := r.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetAll sistemdeki tüm kullanıcıları listeler (Yönetici paneli için).
// Güvenlik: API anahtarları admin listesinde döndürülmez; her sızıntıda tüm kullanıcılar
// taklit edilebileceği için gereksiz yüzeyi azaltır.
func (r *UserRepository) GetAll() ([]*model.User, error) {
	query := `SELECT id, username, role, created_at FROM users ORDER BY created_at DESC`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		u := &model.User{}
		err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// UpdateAPIKey kullanıcının API anahtarını günceller.
func (r *UserRepository) UpdateAPIKey(userID int64, newAPIKey string) error {
	query := `UPDATE users SET api_key = ? WHERE id = ?`
	_, err := r.db.Exec(query, newAPIKey, userID)
	return err
}

// Delete kullanıcıyı siler.
func (r *UserRepository) Delete(id int64) error {
	query := `DELETE FROM users WHERE id = ?`
	_, err := r.db.Exec(query, id)
	return err
}

// Update kullanıcının bilgilerini günceller (kullanıcı adı ve şifre hash'i).
func (r *UserRepository) Update(u *model.User) error {
	query := `UPDATE users SET username = ?, password_hash = ? WHERE id = ?`
	_, err := r.db.Exec(query, u.Username, u.PasswordHash, u.ID)
	return err
}
