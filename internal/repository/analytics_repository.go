package repository

import (
	"database/sql"
	"linklik/internal/db"
	"linklik/internal/model"
	"time"
)

// AnalyticsRepository analitik verilerini veritabanına kaydeden ve istatistikleri çeken katmandır.
type AnalyticsRepository struct {
	db *sql.DB
}

// NewAnalyticsRepository yeni bir AnalyticsRepository örneği döndürür.
func NewAnalyticsRepository() *AnalyticsRepository {
	return &AnalyticsRepository{db: db.Db}
}

// Create yeni bir analitik satırını veritabanına ekler.
func (r *AnalyticsRepository) Create(a *model.Analytics) error {
	query := `INSERT INTO analytics (link_id, ip_address, referrer, user_agent, browser, os, country, clicked_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	
	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")
	res, err := r.db.Exec(query, a.LinkID, a.IPAddress, a.Referrer, a.UserAgent, a.Browser, a.OS, a.Country, nowStr)
	if err != nil {
		return err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	a.ID = id
	a.ClickedAt = now
	return nil
}

// GetStatsByLinkID belirli bir linke ait tüm analitik özetlerini döner.
func (r *AnalyticsRepository) GetStatsByLinkID(linkID int64) (*model.LinkStats, error) {
	stats := &model.LinkStats{
		Countries:   make(map[string]int),
		Browsers:    make(map[string]int),
		OS:          make(map[string]int),
		Referrers:   make(map[string]int),
		DailyClicks: make(map[string]int),
	}

	// 1. Toplam tıklanma sayısını al
	totalQuery := `SELECT COUNT(*) FROM analytics WHERE link_id = ?`
	err := r.db.QueryRow(totalQuery, linkID).Scan(&stats.TotalClicks)
	if err != nil {
		return nil, err
	}

	if stats.TotalClicks == 0 {
		return stats, nil
	}

	// 2. Ülkeleri al
	countryQuery := `SELECT COALESCE(country, 'Bilinmiyor'), COUNT(*) FROM analytics WHERE link_id = ? GROUP BY country`
	rows, err := r.db.Query(countryQuery, linkID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var count int
			if errScan := rows.Scan(&name, &count); errScan == nil {
				stats.Countries[name] = count
			}
		}
	}

	// 3. Tarayıcıları al
	browserQuery := `SELECT COALESCE(browser, 'Bilinmiyor'), COUNT(*) FROM analytics WHERE link_id = ? GROUP BY browser`
	rowsBr, err := r.db.Query(browserQuery, linkID)
	if err == nil {
		defer rowsBr.Close()
		for rowsBr.Next() {
			var name string
			var count int
			if errScan := rowsBr.Scan(&name, &count); errScan == nil {
				stats.Browsers[name] = count
			}
		}
	}

	// 4. İşletim sistemlerini al
	osQuery := `SELECT COALESCE(os, 'Bilinmiyor'), COUNT(*) FROM analytics WHERE link_id = ? GROUP BY os`
	rowsOS, err := r.db.Query(osQuery, linkID)
	if err == nil {
		defer rowsOS.Close()
		for rowsOS.Next() {
			var name string
			var count int
			if errScan := rowsOS.Scan(&name, &count); errScan == nil {
				stats.OS[name] = count
			}
		}
	}

	// 5. Yönlendiricileri al
	refQuery := `SELECT COALESCE(referrer, 'Direkt İrtibat/Bilinmiyor'), COUNT(*) FROM analytics WHERE link_id = ? GROUP BY referrer`
	rowsRef, err := r.db.Query(refQuery, linkID)
	if err == nil {
		defer rowsRef.Close()
		for rowsRef.Next() {
			var name string
			var count int
			if errScan := rowsRef.Scan(&name, &count); errScan == nil {
				if name == "" {
					name = "Direkt İrtibat/Bilinmiyor"
				}
				stats.Referrers[name] = count
			}
		}
	}

	// 6. Günlük tıklanma istatistiğini al (Son 30 gün)
	dailyQuery := `SELECT strftime('%Y-%m-%d', clicked_at) as day, COUNT(*) 
	               FROM analytics 
	               WHERE link_id = ? 
	               GROUP BY day 
	               ORDER BY day ASC 
	               LIMIT 30`
	rowsDaily, err := r.db.Query(dailyQuery, linkID)
	if err == nil {
		defer rowsDaily.Close()
		for rowsDaily.Next() {
			var day string
			var count int
			if errScan := rowsDaily.Scan(&day, &count); errScan == nil {
				stats.DailyClicks[day] = count
			}
		}
	}

	return stats, nil
}
