package main

import (
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"time"
)

// Checkin is the denormalized view returned by the API, matching the
// shape the frontend expects. Pointer fields serialize as null in JSON.
type Checkin struct {
	ID                 int64    `json:"id"`
	DayID              int64    `json:"day_id"`
	CreatedAt          string   `json:"created_at"`
	Lat                *float64 `json:"lat"`
	Lng                *float64 `json:"lng"`
	Town               *string  `json:"town"`
	State              *string  `json:"state"`
	Name               *string  `json:"name"`
	ElevationFt        *int64   `json:"elevation_ft"`
	MileMarker         *float64 `json:"mile_marker"`
	PhotoURL           *string  `json:"photo_url"`
	WeatherTempF       *float64 `json:"weather_temp_f"`
	WeatherCondition   *string  `json:"weather_condition"`
	WeatherWindMph     *float64 `json:"weather_wind_mph"`
	WeatherWindDir     *string  `json:"weather_wind_dir"`
	Date               string   `json:"date"`
	IsRestDay          *bool    `json:"is_rest_day"`          // null on non-primary check-ins
	Note               *string  `json:"note"`
	MilesToday         *float64 `json:"miles_today"`
	ElevGainToday      *int64   `json:"elevation_gain_today"`
	ElevLossToday      *int64   `json:"elevation_loss_today"`
	MovingTimeMinutes  *int64   `json:"moving_time_minutes"`
	AvgSpeedToday      *float64 `json:"avg_speed_today"`
	LodgingType        *string  `json:"lodging_type"`
	TotalMilesOverride *float64 `json:"total_miles_override"`
}

// checkinSQL is the denormalized JOIN used for all checkin reads.
// day_stats columns are null when this check-in does not own the day's stats.
const checkinSQL = `
	SELECT
		c.id, c.day_id, c.created_at, c.lat, c.lng, c.town, c.state, c.name,
		c.elevation_ft, c.mile_marker, c.photo_url,
		c.weather_temp_f, c.weather_condition, c.weather_wind_mph, c.weather_wind_dir,
		d.date,
		CASE WHEN ds.day_id IS NOT NULL THEN d.is_rest_day ELSE NULL END AS is_rest_day,
		ds.note,
		ds.miles           AS miles_today,
		ds.elevation_gain  AS elevation_gain_today,
		ds.elevation_loss  AS elevation_loss_today,
		ds.moving_time_minutes,
		ds.avg_speed       AS avg_speed_today,
		ds.lodging_type,
		ds.total_miles_override
	FROM checkins c
	JOIN days d ON d.id = c.day_id
	LEFT JOIN day_stats ds ON ds.day_id = d.id AND ds.checkin_id = c.id
`

// ── Nullable scanning helpers ─────────────────────────────────────────────────

func ptrStr(s sql.NullString) *string {
	if s.Valid {
		return &s.String
	}
	return nil
}

func ptrF64(f sql.NullFloat64) *float64 {
	if f.Valid {
		return &f.Float64
	}
	return nil
}

func ptrI64(i sql.NullInt64) *int64 {
	if i.Valid {
		return &i.Int64
	}
	return nil
}

func ptrBool(i sql.NullInt64) *bool {
	if !i.Valid {
		return nil
	}
	b := i.Int64 != 0
	return &b
}

// scanCheckin scans one row from checkinSQL into a Checkin.
func scanCheckin(rows *sql.Rows) (Checkin, error) {
	var c Checkin
	var (
		lat, lng, mileMarker, weatherTempF, weatherWindMph sql.NullFloat64
		town, state, name, photoURL                        sql.NullString
		weatherCondition, weatherWindDir                   sql.NullString
		elevFt, isRestDayRaw                               sql.NullInt64
		dsNote, dsLodging                                  sql.NullString
		dsMiles, dsAvgSpeed, dsOverride                    sql.NullFloat64
		dsGain, dsLoss, dsMoving                           sql.NullInt64
	)
	err := rows.Scan(
		&c.ID, &c.DayID, &c.CreatedAt,
		&lat, &lng, &town, &state, &name,
		&elevFt, &mileMarker, &photoURL,
		&weatherTempF, &weatherCondition, &weatherWindMph, &weatherWindDir,
		&c.Date,
		&isRestDayRaw,
		&dsNote,
		&dsMiles,
		&dsGain, &dsLoss, &dsMoving,
		&dsAvgSpeed,
		&dsLodging,
		&dsOverride,
	)
	if err != nil {
		return c, err
	}
	c.Lat = ptrF64(lat)
	c.Lng = ptrF64(lng)
	c.Town = ptrStr(town)
	c.State = ptrStr(state)
	c.Name = ptrStr(name)
	c.ElevationFt = ptrI64(elevFt)
	c.MileMarker = ptrF64(mileMarker)
	c.PhotoURL = ptrStr(photoURL)
	c.WeatherTempF = ptrF64(weatherTempF)
	c.WeatherCondition = ptrStr(weatherCondition)
	c.WeatherWindMph = ptrF64(weatherWindMph)
	c.WeatherWindDir = ptrStr(weatherWindDir)
	c.IsRestDay = ptrBool(isRestDayRaw)
	c.Note = ptrStr(dsNote)
	c.MilesToday = ptrF64(dsMiles)
	c.ElevGainToday = ptrI64(dsGain)
	c.ElevLossToday = ptrI64(dsLoss)
	c.MovingTimeMinutes = ptrI64(dsMoving)
	c.AvgSpeedToday = ptrF64(dsAvgSpeed)
	c.LodgingType = ptrStr(dsLodging)
	c.TotalMilesOverride = ptrF64(dsOverride)
	return c, nil
}

func queryCheckins(orderClause string) ([]Checkin, error) {
	rows, err := db.Query(checkinSQL + orderClause)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Checkin
	for rows.Next() {
		c, err := scanCheckin(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

// ── Stats ─────────────────────────────────────────────────────────────────────

type stats struct {
	TotalMiles           float64 `json:"totalMiles"`
	MilesRemaining       float64 `json:"milesRemaining"`
	PctComplete          float64 `json:"pctComplete"`
	TotalClimbing        int64   `json:"totalClimbing"`
	RidingDays           int     `json:"ridingDays"`
	RestDays             int     `json:"restDays"`
	AvgMilesPerRidingDay float64 `json:"avgMilesPerRidingDay"`
	LongestDay           float64 `json:"longestDay"`
	NightsCamped         int     `json:"nightsCamped"`
	NightsIndoor         int     `json:"nightsIndoor"`
}

type dayRow struct {
	id        int64
	isRestDay int
}

type dayStatRow struct {
	dayID       int64
	miles       sql.NullFloat64
	elevGain    sql.NullInt64
	lodgingType sql.NullString
}

func computeStats(totalMilesRoute float64) stats {
	// Close rows explicitly before opening the next query — with MaxOpenConns(1)
	// a defer would hold the connection open and deadlock the second Query call.
	dayRows, _ := db.Query("SELECT id, is_rest_day FROM days")
	var days []dayRow
	for dayRows.Next() {
		var d dayRow
		dayRows.Scan(&d.id, &d.isRestDay)
		days = append(days, d)
	}
	dayRows.Close()

	dsRows, _ := db.Query("SELECT day_id, miles, elevation_gain, lodging_type FROM day_stats")
	var dayStats []dayStatRow
	for dsRows.Next() {
		var ds dayStatRow
		dsRows.Scan(&ds.dayID, &ds.miles, &ds.elevGain, &ds.lodgingType)
		dayStats = append(dayStats, ds)
	}
	dsRows.Close()

	ridingDayIDs := make(map[int64]bool)
	var ridingDays, restDays int
	for _, d := range days {
		if d.isRestDay != 0 {
			restDays++
		} else {
			ridingDays++
			ridingDayIDs[d.id] = true
		}
	}

	var totalMiles, longestDay float64
	var totalClimbing int64
	var nightsCamped, nightsIndoor int

	for _, ds := range dayStats {
		if ds.miles.Valid {
			totalMiles += ds.miles.Float64
			if ridingDayIDs[ds.dayID] && ds.miles.Float64 > longestDay {
				longestDay = ds.miles.Float64
			}
		}
		if ds.elevGain.Valid {
			totalClimbing += ds.elevGain.Int64
		}
		if ds.lodgingType.Valid {
			switch ds.lodgingType.String {
			case "camping":
				nightsCamped++
			case "hotel", "motel", "warmshowers", "bnb":
				nightsIndoor++
			}
		}
	}

	var avgMiles float64
	if ridingDays > 0 {
		avgMiles = math.Round(totalMiles/float64(ridingDays)*10) / 10
	}

	pct := 0.0
	if totalMilesRoute > 0 {
		pct = math.Round(totalMiles/totalMilesRoute*1000) / 10
	}

	return stats{
		TotalMiles:           math.Round(totalMiles*10) / 10,
		MilesRemaining:       math.Round((totalMilesRoute-totalMiles)*10) / 10,
		PctComplete:          pct,
		TotalClimbing:        totalClimbing,
		RidingDays:           ridingDays,
		RestDays:             restDays,
		AvgMilesPerRidingDay: avgMiles,
		LongestDay:           math.Round(longestDay*10) / 10,
		NightsCamped:         nightsCamped,
		NightsIndoor:         nightsIndoor,
	}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func handleListCheckins(w http.ResponseWriter, r *http.Request) {
	checkins, err := queryCheckins("ORDER BY c.created_at DESC")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if checkins == nil {
		checkins = []Checkin{} // return [] not null
	}
	writeJSON(w, http.StatusOK, checkins)
}

func handleLatestCheckin(w http.ResponseWriter, r *http.Request) {
	checkins, err := queryCheckins("ORDER BY c.created_at DESC LIMIT 1")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var latest any
	if len(checkins) > 0 {
		latest = checkins[0]
	}

	// Close before computeStats opens its own queries on the same connection.
	configRows, _ := db.Query("SELECT key, value FROM trip_config")
	config := make(map[string]string)
	for configRows.Next() {
		var k, v string
		configRows.Scan(&k, &v)
		config[k] = v
	}
	configRows.Close()

	totalRoute := 4205.0
	if v, ok := config["route_total_miles"]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			totalRoute = f
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"latest": latest,
		"stats":  computeStats(totalRoute),
		"config": config,
	})
}

// checkinRequest is the POST /checkins request body.
type checkinRequest struct {
	Lat                *float64 `json:"lat"`
	Lng                *float64 `json:"lng"`
	Name               *string  `json:"name"`
	Town               *string  `json:"town"`
	State              *string  `json:"state"`
	ElevationFt        *int64   `json:"elevation_ft"`
	MileMarker         *float64 `json:"mile_marker"`
	PhotoURL           *string  `json:"photo_url"`
	IsRestDay          bool     `json:"is_rest_day"`
	CreatedAt          string   `json:"created_at"`
	MilesToday         *float64 `json:"miles_today"`
	AvgSpeedToday      *float64 `json:"avg_speed_today"`
	MovingTimeMinutes  *int64   `json:"moving_time_minutes"`
	ElevGainToday      *int64   `json:"elevation_gain_today"`
	ElevLossToday      *int64   `json:"elevation_loss_today"`
	LodgingType        *string  `json:"lodging_type"`
	TotalMilesOverride *float64 `json:"total_miles_override"`
	Note               *string  `json:"note"`
	WeatherTempF       *float64 `json:"weather_temp_f"`
	WeatherWindMph     *float64 `json:"weather_wind_mph"`
	WeatherCondition   *string  `json:"weather_condition"`
	WeatherWindDir     *string  `json:"weather_wind_dir"`
}

func handleCreateCheckin(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var b checkinRequest
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Auto-fetch weather if coords present and weather not supplied
	if b.Lat != nil && b.Lng != nil && b.WeatherTempF == nil {
		if wx := fetchWeather(*b.Lat, *b.Lng); wx != nil {
			b.WeatherTempF = &wx.TempF
			b.WeatherWindMph = &wx.WindMph
			b.WeatherCondition = &wx.Condition
			b.WeatherWindDir = &wx.WindDir
		}
	}

	ts := b.CreatedAt
	if ts == "" {
		ts = time.Now().UTC().Format(time.RFC3339)
	}
	date := ts[:10] // YYYY-MM-DD

	restDayInt := 0
	if b.IsRestDay {
		restDayInt = 1
	}

	// Upsert day
	_, err := db.Exec(`INSERT INTO days (date, is_rest_day) VALUES (?, ?)
		ON CONFLICT(date) DO UPDATE SET is_rest_day = excluded.is_rest_day`,
		date, restDayInt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var dayID int64
	db.QueryRow("SELECT id FROM days WHERE date = ?", date).Scan(&dayID)

	// Insert checkin
	result, err := db.Exec(`INSERT INTO checkins (
		day_id, created_at, lat, lng, name, town, state, mile_marker, elevation_ft,
		photo_url, weather_temp_f, weather_condition, weather_wind_mph, weather_wind_dir
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dayID, ts,
		b.Lat, b.Lng, b.Name, b.Town, b.State, b.MileMarker, b.ElevationFt,
		b.PhotoURL, b.WeatherTempF, b.WeatherCondition, b.WeatherWindMph, b.WeatherWindDir,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	checkinID, _ := result.LastInsertId()

	// Upsert day_stats
	_, err = db.Exec(`INSERT INTO day_stats (
		day_id, checkin_id, miles, elevation_gain, elevation_loss,
		moving_time_minutes, avg_speed, lodging_type, total_miles_override, note
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(day_id) DO UPDATE SET
		checkin_id           = excluded.checkin_id,
		miles                = excluded.miles,
		elevation_gain       = excluded.elevation_gain,
		elevation_loss       = excluded.elevation_loss,
		moving_time_minutes  = excluded.moving_time_minutes,
		avg_speed            = excluded.avg_speed,
		lodging_type         = excluded.lodging_type,
		total_miles_override = excluded.total_miles_override,
		note                 = excluded.note`,
		dayID, checkinID,
		b.MilesToday, b.ElevGainToday, b.ElevLossToday,
		b.MovingTimeMinutes, b.AvgSpeedToday, b.LodgingType,
		b.TotalMilesOverride, b.Note,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]int64{"id": checkinID})
}

func handleDeleteCheckin(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	id := r.PathValue("id")
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var dayID int64
	err := db.QueryRow("SELECT day_id FROM checkins WHERE id = ?", id).Scan(&dayID)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	db.Exec("DELETE FROM checkins WHERE id = ?", id)

	// Clean up the day if it has no remaining check-ins
	var remaining int
	db.QueryRow("SELECT COUNT(*) FROM checkins WHERE day_id = ?", dayID).Scan(&remaining)
	if remaining == 0 {
		db.Exec("DELETE FROM day_stats WHERE day_id = ?", dayID)
		db.Exec("DELETE FROM days WHERE id = ?", dayID)
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
