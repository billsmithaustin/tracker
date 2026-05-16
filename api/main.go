package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func main() {
	initDB()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	mux.HandleFunc("GET /auth", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	// Checkins — /checkins/latest must be registered before /checkins/{id}
	// but Go 1.22 mux prefers literal segments over wildcards regardless of order.
	mux.HandleFunc("GET /checkins/latest", handleLatestCheckin)
	mux.HandleFunc("GET /checkins", handleListCheckins)
	mux.HandleFunc("POST /checkins", handleCreateCheckin)
	mux.HandleFunc("DELETE /checkins/{id}", handleDeleteCheckin)

	// Photo upload
	mux.HandleFunc("POST /photos", handleUploadPhoto)
	mux.HandleFunc("GET /photos/{filename}", handleGetPhoto)

	// Trip config
	mux.HandleFunc("GET /trip", handleGetTrip)
	mux.HandleFunc("PUT /trip", handleUpdateTrip)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("API listening on :%s", port)
	if err := http.ListenAndServe(":"+port, corsMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Checkin-Password")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func internalError(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

func checkAuth(r *http.Request) bool {
	return r.Header.Get("X-Checkin-Password") == os.Getenv("CHECKIN_PASSWORD")
}
