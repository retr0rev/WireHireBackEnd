package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// LocalClient stores uploaded files on the local filesystem.
// Used for development when R2 is not configured.
type LocalClient struct {
	baseDir  string
	publicURL string
}

func NewLocalClient() (*LocalClient, error) {
	baseDir := os.Getenv("LOCAL_UPLOAD_DIR")
	if baseDir == "" {
		baseDir = "./uploads"
	}
	publicURL := os.Getenv("LOCAL_PUBLIC_URL")
	if publicURL == "" {
		// Use Vite proxy URL in dev (both apps proxy /api/* to backend)
		publicURL = "http://localhost:5174"
	}

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}

	return &LocalClient{baseDir: baseDir, publicURL: publicURL}, nil
}

// GeneratePresignedURL returns a local upload endpoint URL.
// In local mode, this is a POST endpoint that accepts multipart form data.
// The "presigned URL" is just the local upload endpoint with the target key.
func (l *LocalClient) GeneratePresignedURL(ctx context.Context, key, contentType string, maxBytes int64) (string, error) {
	// Return a local upload endpoint that the frontend can POST to directly
	// The frontend will POST multipart/form-data to this URL with the file
	return fmt.Sprintf("%s/api/auth/local-upload?key=%s&content_type=%s", l.publicURL, key, contentType), nil
}

func (l *LocalClient) PublicURL(key string) string {
	return fmt.Sprintf("%s/uploads/%s", l.publicURL, key)
}

// ServeFile handles GET requests for uploaded files.
func (l *LocalClient) ServeFile(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/uploads/")
	if key == "" {
		http.NotFound(w, r)
		return
	}

	// Security: prevent directory traversal
	if strings.Contains(key, "..") {
		http.NotFound(w, r)
		return
	}

	filePath := filepath.Join(l.baseDir, key)
	f, err := os.Open(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Set content type based on extension
	ext := strings.ToLower(filepath.Ext(key))
	var contentType string
	switch ext {
	case ".png":
		contentType = "image/png"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".webp":
		contentType = "image/webp"
	case ".svg":
		contentType = "image/svg+xml"
	default:
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	http.ServeContent(w, r, key, stat.ModTime(), f)
}

// HandleLocalUpload handles POST requests for direct file uploads.
func (l *LocalClient) HandleLocalUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, `{"error":"method not allowed - FIXED"}`, http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form with max memory
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB
		http.Error(w, `{"error":"file too large"}`, http.StatusBadRequest)
		return
	}

	key := r.URL.Query().Get("key")
	contentType := r.URL.Query().Get("content_type")

	if key == "" || contentType == "" {
		http.Error(w, `{"error":"missing key or content_type"}`, http.StatusBadRequest)
		return
	}

	// Security: validate content type
	allowedTypes := map[string]bool{
		"image/png":  true,
		"image/jpeg": true,
		"image/webp": true,
		"image/svg+xml": true,
	}
	if !allowedTypes[contentType] {
		http.Error(w, `{"error":"unsupported content type"}`, http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"no file uploaded"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file size (5 MB)
	if header.Size > 5*1024*1024 {
		http.Error(w, `{"error":"file too large (max 5MB)"}`, http.StatusBadRequest)
		return
	}

	// Create directory
	dir := filepath.Join(l.baseDir, filepath.Dir(key))
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, `{"error":"failed to create directory"}`, http.StatusInternalServerError)
		return
	}

	// Save file
	filePath := filepath.Join(l.baseDir, key)
	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(w, `{"error":"failed to save file"}`, http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, `{"error":"failed to write file"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"public_url":"%s"}`, l.PublicURL(key))
}