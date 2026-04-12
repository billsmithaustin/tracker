package main

import (
	"database/sql"
	"encoding/json"
	"log"
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

// nullPtr returns a pointer to val when valid is true, otherwise nil.
func nullPtr[T any](valid bool, val T) *T {
	if valid {
		return &val
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
	c.Lat = nullPtr(lat.Valid, lat.Float64)
	c.Lng = nullPtr(lng.Valid, lng.Float64)
	c.Town = nullPtr(town.Valid, town.String)
	c.State = nullPtr(state.Valid, state.String)
	c.Name = nullPtr(name.Valid, name.String)
	c.ElevationFt = nullPtr(elevFt.Valid, elevFt.Int64)
	c.MileMarker = nullPtr(mileMarker.Valid, mileMarker.Float64)
	c.PhotoURL = nullPtr(photoURL.Valid, photoURL.String)
	c.WeatherTempF = nullPtr(weatherTempF.Valid, weatherTempF.Float64)
	c.WeatherCondition = nullPtr(weatherCondition.Valid, weatherCondition.String)
	c.WeatherWindMph = nullPtr(weatherWindMph.Valid, weatherWindMph.Float64)
	c.WeatherWindDir = nullPtr(weatherWindDir.Valid, weatherWindDir.String)
	c.IsRestDay = ptrBool(isRestDayRaw)
	c.Note = nullPtr(dsNote.Valid, dsNote.String)
	c.MilesToday = nullPtr(dsMiles.Valid, dsMiles.Float64)
	c.ElevGainToday = nullPtr(dsGain.Valid, dsGain.Int64)
	c.ElevLossToday = nullPtr(dsLoss.Valid, dsLoss.Int64)
	c.MovingTimeMinutes = nullPtr(dsMoving.Valid, dsMoving.Int64)
	c.AvgSpeedToday = nullPtr(dsAvgSpeed.Valid, dsAvgSpeed.Float64)
	c.LodgingType = nullPtr(dsLodging.Valid, dsLodging.String)
	c.TotalMilesOverride = nullPtr(dsOverride.Valid, dsOverride.Float64)
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
	dayRows, err := db.Query("SELECT id, is_rest_day FROM days")
	if err != nil {
		log.Printf("computeStats days: %v", err)
		return stats{}
	}
	var days []dayRow
	for dayRows.Next() {
		var d dayRow
		dayRows.Scan(&d.id, &d.isRestDay)
		days = append(days, d)
	}
	dayRows.Close()

	dsRows, err := db.Query("SELECT day_id, miles, elevation_gain, lodging_type FROM day_stats")
	if err != nil {
		log.Printf("computeStats day_stats: %v", err)
		return stats{}
	}
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
		internalError(w, err)
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
		internalError(w, err)
		return
	}

	var latest any
	if len(checkins) > 0 {
		latest = checkins[0]
	}

	config, err := getConfig()
	if err != nil {
		internalError(w, err)
		return
	}

	totalRoute := defaultRouteMiles
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

func upsertDay(date string, isRestDay bool) (int64, error) {
	restDayInt := 0
	if isRestDay {
		restDayInt = 1
	}
	_, err := db.Exec(`INSERT INTO days (date, is_rest_day) VALUES (?, ?)
		ON CONFLICT(date) DO UPDATE SET is_rest_day = excluded.is_rest_day`,
		date, restDayInt)
	if err != nil {
		return 0, err
	}
	var dayID int64
	err = db.QueryRow("SELECT id FROM days WHERE date = ?", date).Scan(&dayID)
	return dayID, err
}

func insertCheckin(dayID int64, ts string, b checkinRequest) (int64, error) {
	result, err := db.Exec(`INSERT INTO checkins (
		day_id, created_at, lat, lng, name, town, state, mile_marker, elevation_ft,
		photo_url, weather_temp_f, weather_condition, weather_wind_mph, weather_wind_dir
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dayID, ts,
		b.Lat, b.Lng, b.Name, b.Town, b.State, b.MileMarker, b.ElevationFt,
		b.PhotoURL, b.WeatherTempF, b.WeatherCondition, b.WeatherWindMph, b.WeatherWindDir,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func upsertDayStats(dayID, checkinID int64, b checkinRequest) error {
	_, err := db.Exec(`INSERT INTO day_stats (
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
	return err
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

	dayID, err := upsertDay(ts[:10], b.IsRestDay)
	if err != nil {
		internalError(w, err)
		return
	}

	checkinID, err := insertCheckin(dayID, ts, b)
	if err != nil {
		internalError(w, err)
		return
	}

	if err := upsertDayStats(dayID, checkinID, b); err != nil {
		internalError(w, err)
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
