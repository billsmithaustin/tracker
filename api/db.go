package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func initDB() {
	dataDir := "/app/data"
	if d := os.Getenv("DATA_DIR"); d != "" {
		dataDir = d
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	dbPath := filepath.Join(dataDir, "tracker.db")
	var err error
	db, err = sql.Open("sqlite", dbPath+"?_journal=WAL&_timeout=5000&_fk=true")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer

	createSchema()
	seedConfig()
}

func mustExec(query string, args ...any) {
	if _, err := db.Exec(query, args...); err != nil {
		log.Fatalf("db exec: %v\nquery: %s", err, query)
	}
}

func createSchema() {
	mustExec(`CREATE TABLE IF NOT EXISTS trip_config (
		key   TEXT PRIMARY KEY,
		value TEXT
	)`)
	mustExec(`CREATE TABLE IF NOT EXISTS days (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		date        TEXT NOT NULL UNIQUE,
		is_rest_day INTEGER DEFAULT 0
	)`)
	mustExec(`CREATE TABLE IF NOT EXISTS checkins (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		day_id            INTEGER NOT NULL REFERENCES days(id),
		created_at        TEXT NOT NULL,
		lat               REAL,
		lng               REAL,
		town              TEXT,
		state             TEXT,
		name              TEXT,
		elevation_ft      INTEGER,
		mile_marker       REAL,
		photo_url         TEXT,
		weather_temp_f    REAL,
		weather_condition TEXT,
		weather_wind_mph  REAL,
		weather_wind_dir  TEXT
	)`)
	mustExec(`CREATE TABLE IF NOT EXISTS day_stats (
		day_id               INTEGER PRIMARY KEY REFERENCES days(id),
		checkin_id           INTEGER REFERENCES checkins(id),
		miles                REAL,
		elevation_gain       INTEGER,
		elevation_loss       INTEGER,
		moving_time_minutes  INTEGER,
		avg_speed            REAL,
		lodging_type         TEXT,
		total_miles_override REAL,
		note                 TEXT
	)`)
}

func seedConfig() {
	defaults := map[string]string{
		"rider_name":        "Bill",
		"start_date":        "2026-04-16",
		"route_total_miles": "4205",
		"target_days":       "90",
	}
	for k, v := range defaults {
		db.Exec("INSERT OR IGNORE INTO trip_config (key, value) VALUES (?, ?)", k, v)
	}
}
