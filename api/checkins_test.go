package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// TestMain wires up an in-memory SQLite DB for all tests in this package.
func TestMain(m *testing.M) {
	var err error
	db, err = sql.Open("sqlite", "file::memory:?cache=shared&_fk=true")
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(1)
	createSchema()
	seedConfig()
	os.Setenv("CHECKIN_PASSWORD", "testpass")
	os.Exit(m.Run())
}

// resetDB wipes check-in data between tests.
func resetDB(t *testing.T) {
	t.Helper()
	for _, tbl := range []string{"day_stats", "checkins", "days"} {
		if _, err := db.Exec("DELETE FROM " + tbl); err != nil {
			t.Fatalf("resetDB: %v", err)
		}
	}
}

func authedRequest(method, path string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, path, &buf)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Checkin-Password", "testpass")
	return r
}

// ── ptr helpers ───────────────────────────────────────────────────────────────

func TestNullPtr(t *testing.T) {
	if nullPtr(false, "") != nil {
		t.Error("invalid string should return nil")
	}
	s := nullPtr(true, "hello")
	if s == nil || *s != "hello" {
		t.Errorf("expected %q, got %v", "hello", s)
	}

	if nullPtr(false, 0.0) != nil {
		t.Error("invalid float64 should return nil")
	}
	f := nullPtr(true, 3.14)
	if f == nil || *f != 3.14 {
		t.Errorf("expected 3.14, got %v", f)
	}

	if nullPtr(false, int64(0)) != nil {
		t.Error("invalid int64 should return nil")
	}
	i := nullPtr(true, int64(42))
	if i == nil || *i != 42 {
		t.Errorf("expected 42, got %v", i)
	}
}

func TestPtrBool(t *testing.T) {
	if ptrBool(sql.NullInt64{}) != nil {
		t.Error("invalid NullInt64 should return nil")
	}
	b := ptrBool(sql.NullInt64{Int64: 1, Valid: true})
	if b == nil || !*b {
		t.Errorf("expected true, got %v", b)
	}
	b = ptrBool(sql.NullInt64{Int64: 0, Valid: true})
	if b == nil || *b {
		t.Errorf("expected false, got %v", b)
	}
}

// ── handler integration tests ─────────────────────────────────────────────────

func TestCreateAndListCheckins(t *testing.T) {
	resetDB(t)

	body := map[string]any{
		"lat":        38.9,
		"lng":        -77.0,
		"town":       "Yorktown",
		"state":      "VA",
		"created_at": "2026-04-16T10:00:00Z",
		"miles_today": 62.5,
		"is_rest_day": false,
	}
	w := httptest.NewRecorder()
	handleCreateCheckin(w, authedRequest("POST", "/checkins", body))
	if w.Code != http.StatusCreated {
		t.Fatalf("create: got %d, want 201; body: %s", w.Code, w.Body)
	}

	var created map[string]int64
	json.NewDecoder(w.Body).Decode(&created)
	if created["id"] == 0 {
		t.Fatal("expected non-zero id in response")
	}

	w2 := httptest.NewRecorder()
	handleListCheckins(w2, httptest.NewRequest("GET", "/checkins", nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("list: got %d", w2.Code)
	}
	var checkins []Checkin
	json.NewDecoder(w2.Body).Decode(&checkins)
	if len(checkins) != 1 {
		t.Fatalf("expected 1 checkin, got %d", len(checkins))
	}
	if checkins[0].Town == nil || *checkins[0].Town != "Yorktown" {
		t.Errorf("unexpected town: %v", checkins[0].Town)
	}
}

func TestCreateCheckinAuth(t *testing.T) {
	resetDB(t)

	r := httptest.NewRequest("POST", "/checkins", bytes.NewBufferString(`{}`))
	r.Header.Set("Content-Type", "application/json")
	// no auth header
	w := httptest.NewRecorder()
	handleCreateCheckin(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestDeleteCheckinCleansUpDay(t *testing.T) {
	resetDB(t)

	// Create a check-in
	w := httptest.NewRecorder()
	handleCreateCheckin(w, authedRequest("POST", "/checkins", map[string]any{
		"town":       "Mineral",
		"state":      "VA",
		"created_at": "2026-04-17T12:00:00Z",
		"miles_today": 55.0,
	}))
	var created map[string]int64
	json.NewDecoder(w.Body).Decode(&created)
	id := created["id"]

	// Confirm the day exists
	var dayCount int
	db.QueryRow("SELECT COUNT(*) FROM days").Scan(&dayCount)
	if dayCount != 1 {
		t.Fatalf("expected 1 day, got %d", dayCount)
	}

	// Delete the check-in
	req := authedRequest("DELETE", "/checkins/"+itoa(id), nil)
	req.SetPathValue("id", itoa(id))
	w2 := httptest.NewRecorder()
	handleDeleteCheckin(w2, req)
	if w2.Code != http.StatusOK {
		t.Fatalf("delete: got %d; body: %s", w2.Code, w2.Body)
	}

	// Day and day_stats should be gone
	db.QueryRow("SELECT COUNT(*) FROM days").Scan(&dayCount)
	if dayCount != 0 {
		t.Errorf("expected day to be cleaned up, got %d days", dayCount)
	}
	var statsCount int
	db.QueryRow("SELECT COUNT(*) FROM day_stats").Scan(&statsCount)
	if statsCount != 0 {
		t.Errorf("expected day_stats to be cleaned up, got %d rows", statsCount)
	}
}

func TestDeleteCheckinNonExistent(t *testing.T) {
	resetDB(t)

	req := authedRequest("DELETE", "/checkins/9999", nil)
	req.SetPathValue("id", "9999")
	w := httptest.NewRecorder()
	handleDeleteCheckin(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for missing id, got %d", w.Code)
	}
}

func TestDeleteCheckinInvalidID(t *testing.T) {
	resetDB(t)

	req := authedRequest("DELETE", "/checkins/abc", nil)
	req.SetPathValue("id", "abc")
	w := httptest.NewRecorder()
	handleDeleteCheckin(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid id, got %d", w.Code)
	}
}

func TestComputeStats(t *testing.T) {
	resetDB(t)

	// Two riding days + one rest day
	days := []struct {
		date      string
		isRest    int
		miles     float64
		gain      int64
		lodging   string
	}{
		{"2026-04-16", 0, 62.5, 2100, "camping"},
		{"2026-04-17", 0, 78.0, 3400, "hotel"},
		{"2026-04-18", 1, 0, 0, ""},
	}

	for _, d := range days {
		db.Exec("INSERT INTO days (date, is_rest_day) VALUES (?, ?)", d.date, d.isRest)
		var dayID int64
		db.QueryRow("SELECT id FROM days WHERE date = ?", d.date).Scan(&dayID)
		db.Exec("INSERT INTO checkins (day_id, created_at) VALUES (?, ?)", dayID, d.date+"T10:00:00Z")
		var cid int64
		db.QueryRow("SELECT id FROM checkins WHERE day_id = ?", dayID).Scan(&cid)
		if d.miles > 0 {
			db.Exec(`INSERT INTO day_stats (day_id, checkin_id, miles, elevation_gain, lodging_type)
				VALUES (?, ?, ?, ?, ?)`, dayID, cid, d.miles, d.gain, d.lodging)
		}
	}

	s := computeStats(4205.0)

	if s.RidingDays != 2 {
		t.Errorf("RidingDays: got %d, want 2", s.RidingDays)
	}
	if s.RestDays != 1 {
		t.Errorf("RestDays: got %d, want 1", s.RestDays)
	}
	if s.TotalMiles != 140.5 {
		t.Errorf("TotalMiles: got %v, want 140.5", s.TotalMiles)
	}
	if s.LongestDay != 78.0 {
		t.Errorf("LongestDay: got %v, want 78.0", s.LongestDay)
	}
	if s.TotalClimbing != 5500 {
		t.Errorf("TotalClimbing: got %d, want 5500", s.TotalClimbing)
	}
	if s.NightsCamped != 1 {
		t.Errorf("NightsCamped: got %d, want 1", s.NightsCamped)
	}
	if s.NightsIndoor != 1 {
		t.Errorf("NightsIndoor: got %d, want 1", s.NightsIndoor)
	}
	wantAvg := 70.3 // (62.5 + 78.0) / 2 = 70.25 → rounded to 70.3
	if s.AvgMilesPerRidingDay != wantAvg {
		t.Errorf("AvgMilesPerRidingDay: got %v, want %v", s.AvgMilesPerRidingDay, wantAvg)
	}
}

func TestLatestCheckinEmptyDB(t *testing.T) {
	resetDB(t)

	w := httptest.NewRecorder()
	handleLatestCheckin(w, httptest.NewRequest("GET", "/checkins/latest", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("got %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["latest"] != nil {
		t.Errorf("expected null latest on empty DB, got %v", resp["latest"])
	}
	if resp["stats"] == nil {
		t.Error("expected stats in response")
	}
}

func TestNotePreservedAfterSecondCheckinSameDay(t *testing.T) {
	resetDB(t)

	note := "great campsite"
	photoURL := "/api/photos/day2.jpg"

	// First check-in: has a note, no photo
	w := httptest.NewRecorder()
	handleCreateCheckin(w, authedRequest("POST", "/checkins", map[string]any{
		"town":        "Afton",
		"state":       "VA",
		"created_at":  "2026-04-20T08:00:00Z",
		"miles_today": 55.0,
		"note":        note,
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("first checkin: got %d; body: %s", w.Code, w.Body)
	}

	// Second check-in: same day, has a photo, no note
	w2 := httptest.NewRecorder()
	handleCreateCheckin(w2, authedRequest("POST", "/checkins", map[string]any{
		"town":       "Waynesboro",
		"state":      "VA",
		"created_at": "2026-04-20T18:00:00Z",
		"photo_url":  photoURL,
	}))
	if w2.Code != http.StatusCreated {
		t.Fatalf("second checkin: got %d; body: %s", w2.Code, w2.Body)
	}

	// List check-ins and verify the first one still has its note
	w3 := httptest.NewRecorder()
	handleListCheckins(w3, httptest.NewRequest("GET", "/checkins", nil))
	var checkins []Checkin
	json.NewDecoder(w3.Body).Decode(&checkins)
	if len(checkins) != 2 {
		t.Fatalf("expected 2 checkins, got %d", len(checkins))
	}

	// Results are ordered by created_at DESC, so checkins[0] is the second, checkins[1] is the first
	first := checkins[1]
	second := checkins[0]

	if first.Note == nil || *first.Note != note {
		t.Errorf("first checkin note: got %v, want %q", first.Note, note)
	}
	if second.PhotoURL == nil || *second.PhotoURL != photoURL {
		t.Errorf("second checkin photo_url: got %v, want %q", second.PhotoURL, photoURL)
	}
	if second.Note != nil {
		t.Errorf("second checkin note: expected nil, got %q", *second.Note)
	}
}

// itoa converts int64 to string without importing strconv in test helpers.
func itoa(n int64) string {
	return string(bytes.TrimSpace([]byte(fmt.Sprint(n))))
}
