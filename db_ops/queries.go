package db_ops

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mattn/go-sqlite3"
)

var ErrNameTaken = errors.New("name already taken")

type Link struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	RedirectURL string `json:"redirect_url"`
	CreatedAt   string `json:"created_at"`
	Clicks      int    `json:"clicks"`
	BotClicks   int    `json:"bot_clicks"`
}

type Count struct {
	Key    string `json:"key"`
	Clicks int    `json:"clicks"`
	Bots   int    `json:"bots"`
}

type Stats struct {
	Link      Link    `json:"link"`
	ByDay     []Count `json:"by_day"`
	ByCountry []Count `json:"by_country"`
}

const selectLinks = `
	SELECT u.id, u.name, u.url, u.redirect_url, u.created_at,
	       COUNT(c.id), COALESCE(SUM(c.origin = ?), 0)
	FROM urls u LEFT JOIN clicks c ON c.url_id = u.id`

func scanLink(row interface{ Scan(...any) error }) (Link, error) {
	var l Link
	err := row.Scan(&l.ID, &l.Name, &l.URL, &l.RedirectURL, &l.CreatedAt, &l.Clicks, &l.BotClicks)
	return l, err
}

func GetURL(db *sql.DB, name string) (int64, string, error) {
	var id int64
	var target string
	err := db.QueryRow(`SELECT id, url FROM urls WHERE name = ?`, name).Scan(&id, &target)
	if err != nil {
		return 0, "", fmt.Errorf("get url %q: %w", name, err)
	}
	return id, target, nil
}

func CreateURL(db *sql.DB, name, target, redirectURL string) error {
	_, err := db.Exec(`INSERT INTO urls (name, url, redirect_url, user_id) VALUES (?, ?, ?, ?)`,
		name, target, redirectURL, AdminUserID)
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
		return ErrNameTaken
	}
	if err != nil {
		return fmt.Errorf("create url: %w", err)
	}
	return nil
}

func DeleteURL(db *sql.DB, name string) error {
	res, err := db.Exec(`DELETE FROM urls WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete url: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func ListURLs(db *sql.DB) ([]Link, error) {
	rows, err := db.Query(selectLinks+` GROUP BY u.id ORDER BY u.id DESC`, OriginBot)
	if err != nil {
		return nil, fmt.Errorf("list urls: %w", err)
	}
	defer rows.Close()
	links := []Link{}
	for rows.Next() {
		l, err := scanLink(rows)
		if err != nil {
			return nil, fmt.Errorf("list urls: %w", err)
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func countClicks(db *sql.DB, groupBy string, orderBy string, urlID int64) ([]Count, error) {
	rows, err := db.Query(`SELECT `+groupBy+`, COUNT(*), SUM(origin = ?) FROM clicks
		WHERE url_id = ? GROUP BY 1 ORDER BY `+orderBy, OriginBot, urlID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := []Count{}
	for rows.Next() {
		var c Count
		if err := rows.Scan(&c.Key, &c.Clicks, &c.Bots); err != nil {
			return nil, err
		}
		counts = append(counts, c)
	}
	return counts, rows.Err()
}

func GetStats(db *sql.DB, name string) (Stats, error) {
	var s Stats
	l, err := scanLink(db.QueryRow(selectLinks+` WHERE u.name = ? GROUP BY u.id`, OriginBot, name))
	if err != nil {
		return s, fmt.Errorf("get stats %q: %w", name, err)
	}
	s.Link = l
	if s.ByDay, err = countClicks(db, "date(clicked_at)", "1", l.ID); err != nil {
		return s, fmt.Errorf("stats by day: %w", err)
	}
	if s.ByCountry, err = countClicks(db, "location", "2 DESC", l.ID); err != nil {
		return s, fmt.Errorf("stats by country: %w", err)
	}
	return s, nil
}

type Click struct {
	Time      time.Time
	IP        string
	Origin    int
	Location  string
	UserAgent string
	Browser   string
	OS        string
	Device    string
	Referrer  string
}

func InsertClick(db *sql.DB, urlID int64, c Click) error {
	_, err := db.Exec(`INSERT INTO clicks (ip, origin, location, user_agent, browser, os, device, referrer, url_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.IP, c.Origin, c.Location, c.UserAgent, c.Browser, c.OS, c.Device, c.Referrer, urlID)
	if err != nil {
		return fmt.Errorf("insert click: %w", err)
	}
	return nil
}

func ListClicks(db *sql.DB, urlID int64, since time.Time) ([]Click, error) {
	rows, err := db.Query(`SELECT clicked_at, ip, origin, location, user_agent, browser, os, device, referrer
		FROM clicks WHERE url_id = ? AND clicked_at >= ? ORDER BY clicked_at, id`,
		urlID, since.UTC().Format(time.DateTime))
	if err != nil {
		return nil, fmt.Errorf("list clicks: %w", err)
	}
	defer rows.Close()
	clicks := []Click{}
	for rows.Next() {
		var c Click
		var at string
		if err := rows.Scan(&at, &c.IP, &c.Origin, &c.Location, &c.UserAgent, &c.Browser, &c.OS, &c.Device, &c.Referrer); err != nil {
			return nil, fmt.Errorf("list clicks: %w", err)
		}
		if c.Time, err = time.ParseInLocation(time.DateTime, at, time.UTC); err != nil {
			return nil, fmt.Errorf("list clicks: bad time %q: %w", at, err)
		}
		clicks = append(clicks, c)
	}
	return clicks, rows.Err()
}
