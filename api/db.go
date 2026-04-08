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

	applyMigrations()
	seedConfig()
}

func hasTable(name string) bool {
	var n int
	db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&n)
	return n > 0
}

func hasColumn(table, col string) bool {
	var n int
	db.QueryRow("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?", table, col).Scan(&n)
	return n > 0
}

func mustExec(query string, args ...any) {
	if _, err := db.Exec(query, args...); err != nil {
		log.Fatalf("db exec: %v\nquery: %s", err, query)
	}
}

func applyMigrations() {
	// Migration 1: old flat checkins table (has miles_today column) → normalized schema
	checkinExists := hasTable("checkins")
	needsMigration1 := checkinExists && hasColumn("checkins", "miles_today")
	if needsMigration1 {
		mustExec("ALTER TABLE checkins RENAME TO _old_checkins")
	}

	// Migration 2: intermediate day_stats missing checkin_id/note → current schema
	needsMigration2 := hasTable("day_stats") && !hasColumn("day_stats", "checkin_id")

	// Create current schema (IF NOT EXISTS is safe to run every time)
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

	if needsMigration1 {
		mustExec(`INSERT INTO days (date, is_rest_day)
			SELECT date(created_at), is_rest_day FROM _old_checkins ORDER BY created_at ASC`)
		mustExec(`INSERT INTO checkins (day_id, created_at, lat, lng, town, state, name,
			elevation_ft, mile_marker, photo_url,
			weather_temp_f, weather_condition, weather_wind_mph, weather_wind_dir)
			SELECT d.id, o.created_at, o.lat, o.lng, o.town, o.state, o.name,
			  o.elevation_ft, o.mile_marker, o.photo_url,
			  o.weather_temp_f, o.weather_condition, o.weather_wind_mph, o.weather_wind_dir
			FROM _old_checkins o
			JOIN days d ON d.date = date(o.created_at)`)
		mustExec(`INSERT INTO day_stats (day_id, checkin_id, miles, elevation_gain, elevation_loss,
			moving_time_minutes, avg_speed, lodging_type, total_miles_override, note)
			SELECT d.id,
			  (SELECT c.id FROM checkins c WHERE c.day_id = d.id ORDER BY c.created_at DESC LIMIT 1),
			  o.miles_today, o.elevation_gain_today, o.elevation_loss_today,
			  o.moving_time_minutes, o.avg_speed_today, o.lodging_type, o.total_miles_override, o.note
			FROM _old_checkins o
			JOIN days d ON d.date = date(o.created_at)`)
	}

	if needsMigration2 {
		mustExec("ALTER TABLE day_stats ADD COLUMN checkin_id INTEGER REFERENCES checkins(id)")
		mustExec("ALTER TABLE day_stats ADD COLUMN note TEXT")
		mustExec(`UPDATE day_stats SET
			checkin_id = (
				SELECT c.id FROM checkins c
				WHERE c.day_id = day_stats.day_id
				ORDER BY c.created_at DESC LIMIT 1
			),
			note = (
				SELECT d.note FROM days d WHERE d.id = day_stats.day_id
			)`)
	}
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
