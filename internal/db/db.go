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

	// Şema göçlerini çalıştır
	if err := RunMigrations(Db); err != nil {
		return fmt.Errorf("veritabanı şema göçleri başarısız: %v", err)
	}

	log.Printf("SQLite veritabanı başarıyla başlatıldı: %s (WAL Modu Aktif)", dbPath)
	return nil
}


// CloseDB veritabanı bağlantısını güvenli bir şekilde kapatır.
func CloseDB() {
	if Db != nil {
		_ = Db.Close()
		log.Println("Veritabanı bağlantısı kapatıldı.")
	}
}
