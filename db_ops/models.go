package db_ops

import "fmt"

const (
	OriginUnknown = iota
	OriginHuman
	OriginBot
)

const AdminUserID = 1

var sqlCreateTables = []string{
	`CREATE TABLE IF NOT EXISTS users (
	id					INTEGER PRIMARY KEY AUTOINCREMENT,
	email       TEXT UNIQUE NOT NULL,
	password		TEXT NOT NULL,
	created_at  TEXT DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS urls (
	id						INTEGER PRIMARY KEY AUTOINCREMENT,
	name  				TEXT UNIQUE NOT NULL,
	url						TEXT NOT NULL,
	redirect_url 	TEXT UNIQUE NOT NULL,
	created_at		TEXT DEFAULT CURRENT_TIMESTAMP,
	user_id       INTEGER NOT NULL,

	CONSTRAINT		fk_user FOREIGN KEY (user_id) references users(id) ON DELETE CASCADE
	)`,
	`CREATE TABLE IF NOT EXISTS clicks (
	id 						INTEGER PRIMARY KEY AUTOINCREMENT,
	ip						TEXT NOT NULL,
	origin				INTEGER NOT NULL,
	location			TEXT NOT NULL,
	clicked_at 		TEXT DEFAULT CURRENT_TIMESTAMP,
	url_id        INTEGER NOT NULL,
	user_agent    TEXT NOT NULL DEFAULT '',
	browser       TEXT NOT NULL DEFAULT '',
	os            TEXT NOT NULL DEFAULT '',
	device        TEXT NOT NULL DEFAULT '',
	referrer      TEXT NOT NULL DEFAULT '',

	CONSTRAINT fk_url FOREIGN KEY (url_id) references urls(id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_clicks_url_time ON clicks (url_id, clicked_at)`,
}

var addedClickColumns = []string{"user_agent", "browser", "os", "device", "referrer"}

func InitModels() error {
	db, err := OpenBD()
	if err != nil {
		return err
	}
	defer db.Close()
	for _, query := range sqlCreateTables {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("create table: %w", err)
		}
	}
	for _, column := range addedClickColumns {
		var exists bool
		if err := db.QueryRow(`SELECT COUNT(*) > 0 FROM pragma_table_info('clicks') WHERE name = ?`, column).Scan(&exists); err != nil {
			return fmt.Errorf("check column %s: %w", column, err)
		}
		if !exists {
			if _, err := db.Exec(`ALTER TABLE clicks ADD COLUMN ` + column + ` TEXT NOT NULL DEFAULT ''`); err != nil {
				return fmt.Errorf("add column %s: %w", column, err)
			}
		}
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO users (id, email, password) VALUES (?, 'admin@localhost', '')`, AdminUserID); err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}
	fmt.Println("DATABASE MODELS INITIALIZED SUCCESSFULLY %%%%")
	return nil
}