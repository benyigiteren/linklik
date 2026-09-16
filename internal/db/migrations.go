package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

type migration struct {
	version int
	name    string
	run     func(db *sql.DB) error
}

// RunMigrations veritabanı şema göçlerini sırasıyla çalıştırır.
func RunMigrations(database *sql.DB) error {
	// 1. Göç geçmişi tablosunu oluştur
	_, err := database.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`)
	if err != nil {
		return fmt.Errorf("schema_migrations tablosu oluşturulamadı: %w", err)
	}

	migrations := []migration{
		{
			version: 1,
			name:    "create_initial_tables",
			run:     migrationInitialTables,
		},
		{
			version: 2,
			name:    "add_max_clicks_column",
			run:     func(db *sql.DB) error { return addColumnIfNotExists(db, "links", "max_clicks", "INTEGER DEFAULT 0") },
		},
		{
			version: 3,
			name:    "add_is_active_column",
			run:     func(db *sql.DB) error { return addColumnIfNotExists(db, "links", "is_active", "INTEGER DEFAULT 1") },
		},
		{
			version: 4,
			name:    "add_expires_at_column",
			run:     func(db *sql.DB) error { return addColumnIfNotExists(db, "links", "expires_at", "DATETIME DEFAULT NULL") },
		},
		{
			version: 5,
			name:    "add_password_hash_column",
			run:     func(db *sql.DB) error { return addColumnIfNotExists(db, "links", "password_hash", "TEXT DEFAULT NULL") },
		},
		{
			version: 6,
			name:    "create_optimized_indexes",
			run:     migrationOptimizedIndexes,
		},
	}

	for _, m := range migrations {
		var exists int
		err := database.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", m.version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("göç kontrol hatası (v%d): %w", m.version, err)
		}

		if exists == 0 {
			log.Printf("[Migration] Çalıştırılıyor: v%d - %s", m.version, m.name)
			if err := m.run(database); err != nil {
				return fmt.Errorf("göç başarısız (v%d - %s): %w", m.version, m.name, err)
			}
			_, err = database.Exec("INSERT INTO schema_migrations (version, name) VALUES (?, ?)", m.version, m.name)
			if err != nil {
				return fmt.Errorf("göç geçmişi kaydedilemedi (v%d): %w", m.version, err)
			}
		}
	}

	return nil
}

func migrationInitialTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL,
			api_key TEXT UNIQUE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
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
		`CREATE INDEX IF NOT EXISTS idx_users_api_key ON users(api_key);`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);`,
		`CREATE INDEX IF NOT EXISTS idx_links_short_code ON links(short_code);`,
		`CREATE INDEX IF NOT EXISTS idx_links_custom_alias ON links(custom_alias);`,
		`CREATE INDEX IF NOT EXISTS idx_analytics_link_id ON analytics(link_id);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func migrationOptimizedIndexes(db *sql.DB) error {
	queries := []string{
		`CREATE INDEX IF NOT EXISTS idx_links_created_by ON links(created_by_id);`,
		`CREATE INDEX IF NOT EXISTS idx_links_created_at ON links(created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_analytics_clicked_at ON analytics(clicked_at DESC);`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// addColumnIfNotExists tablonun ilgili kolonunu yoksa ALTER TABLE ile ekler (idempotent).
func addColumnIfNotExists(db *sql.DB, tableName, colName, colDef string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return err
	}
	defer rows.Close()

	colFound := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, colName) {
			colFound = true
			break
		}
	}

	if !colFound {
		query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", tableName, colName, colDef)
		if _, err := db.Exec(query); err != nil {
			return err
		}
		log.Printf("[Migration] Tabloya kolon eklendi: %s.%s %s", tableName, colName, colDef)
	}
	return nil
}
