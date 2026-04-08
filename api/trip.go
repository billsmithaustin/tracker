package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func handleGetTrip(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT key, value FROM trip_config")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	config := make(map[string]string)
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		config[k] = v
	}
	writeJSON(w, http.StatusOK, config)
}

func handleUpdateTrip(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	allowed := []string{"rider_name", "start_date", "route_total_miles", "target_days"}
	for _, key := range allowed {
		if val, ok := body[key]; ok {
			db.Exec("INSERT OR REPLACE INTO trip_config (key, value) VALUES (?, ?)", key, fmt.Sprint(val))
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
