package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	// SQLite sürücüsü pure Go (CGO gerektirmez)
	_ "modernc.org/sqlite"
)

// Db global veritabanı bağlantı nesnesidir.
var Db *sql.DB

// InitDB veritabanı bağlantısını açar, WAL ve diğer optimize PRAGMA ayarlarını yapar, tabloları oluşturur.
func InitDB(dbPath string) error {
	// Veritabanı klasörü yoksa oluştur
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("veritabanı klasörü oluşturulamadı: %v", err)
		}
	}

	// SQLite bağlantısını aç
	var err error
	Db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("veritabanı açılamadı: %v", err)
	}

	// Bağlantı havuzu ayarları (SQLite tek bir dosya olduğu için aşırı eşzamanlı yazmalarda kilitlenmeyi önlemek amacıyla sınırlandırılır)
	Db.SetMaxOpenConns(1) // modernc.org/sqlite ve WAL modu için tek yazıcı bağlantısı önerilir

	// WAL modu ve performans iyileştirmelerini uygula
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=ON;",
	}

	for _, pragma := range pragmas {
		if _, err := Db.Exec(pragma); err != nil {
			return fmt.Errorf("pragma uygulanamadı (%s): %v", pragma, err)
		}
	}

	// Tabloları oluştur
	if err := createTables(); err != nil {
		return fmt.Errorf("tablolar oluşturulamadı: %v", err)
	}

	// Göç İşlemi: max_clicks kolonu yoksa ekle
	_, _ = Db.Exec("ALTER TABLE links ADD COLUMN max_clicks INTEGER DEFAULT 0;")

	log.Printf("SQLite veritabanı başarıyla başlatıldı: %s (WAL Modu Aktif)", dbPath)
	return nil
}

// createTables veritabanı şemasını oluşturur.
func createTables() error {
	queries := []string{
		// Kullanıcılar tablosu
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL,
			api_key TEXT UNIQUE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		// Linkler tablosu (max_clicks kolonu eklendi)
		`CREATE TABLE IF NOT EXISTS links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			original_url TEXT NOT NULL,
			short_code TEXT UNIQUE NOT NULL,
			custom_alias TEXT UNIQUE,
			created_by_id INTEGER NOT NULL,
			click_count INTEGER DEFAULT 0,
			max_clicks INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (created_by_id) REFERENCES users (id) ON DELETE CASCADE
		);`,

		// Analitik tablosu
		`CREATE TABLE IF NOT EXISTS analytics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			link_id INTEGER NOT NULL,
			ip_address TEXT,
			referrer TEXT,
			user_agent TEXT,
			browser TEXT,
			os TEXT,
			country TEXT,
			clicked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (link_id) REFERENCES links (id) ON DELETE CASCADE
		);`,

		// İndeksler (Sorgu performansları için)
		`CREATE INDEX IF NOT EXISTS idx_users_api_key ON users(api_key);`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);`,
		`CREATE INDEX IF NOT EXISTS idx_links_short_code ON links(short_code);`,
		`CREATE INDEX IF NOT EXISTS idx_links_custom_alias ON links(custom_alias);`,
		`CREATE INDEX IF NOT EXISTS idx_analytics_link_id ON analytics(link_id);`,
	}

	for _, query := range queries {
		if _, err := Db.Exec(query); err != nil {
			return err
		}
	}

	return nil
}

// CloseDB veritabanı bağlantısını güvenli bir şekilde kapatır.
func CloseDB() {
	if Db != nil {
		_ = Db.Close()
		log.Println("Veritabanı bağlantısı kapatıldı.")
	}
}
