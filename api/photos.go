package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const uploadsDir = "/app/data/uploads"

var allowedExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
	".heic": true,
}

func handleUploadPhoto(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 15<<20) // 15 MB
	if err := r.ParseMultipartForm(15 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file too large (max 15 MB)"})
		return
	}

	file, header, err := r.FormFile("photo")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing photo field"})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExts[ext] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported file type; use jpg, png, gif, webp, or heic"})
		return
	}

	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	filename := hex.EncodeToString(b[:]) + ext

	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "storage error"})
		return
	}

	dst, err := os.Create(filepath.Join(uploadsDir, filename))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "storage error"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "write error"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"url": fmt.Sprintf("/api/photos/%s", filename),
	})
}

func handleGetPhoto(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	// Reject any path traversal attempts
	if strings.ContainsAny(filename, "/\\") || strings.Contains(filename, "..") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, filepath.Join(uploadsDir, filename))
}
