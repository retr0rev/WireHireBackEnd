package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
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

// HandleLocalUpload handles direct file uploads in local dev mode.
// Supports both:
//   - Raw PUT/POST with Content-Type image/* and file bytes as body
//   - Multipart form POST with a "file" field (legacy)
func (l *LocalClient) HandleLocalUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	contentType := r.URL.Query().Get("content_type")

	if key == "" || contentType == "" {
		http.Error(w, `{"error":"missing key or content_type"}`, http.StatusBadRequest)
		return
	}

	if strings.Contains(key, "..") {
		http.Error(w, `{"error":"invalid key"}`, http.StatusBadRequest)
		return
	}

	// Security: validate content type
	allowedTypes := map[string]bool{
		"image/png":       true,
		"image/jpeg":      true,
		"image/webp":      true,
		"image/svg+xml":   true,
	}
	if !allowedTypes[contentType] {
		http.Error(w, `{"error":"unsupported content type"}`, http.StatusBadRequest)
		return
	}

	var file io.ReadCloser
	var fileSize int64

	// Check if this is a multipart upload or raw body upload.
	contentTypeHeader := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentTypeHeader, "multipart/form-data") {
		// Multipart form upload (legacy path).
		if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB
			http.Error(w, `{"error":"failed to parse multipart form"}`, http.StatusBadRequest)
			return
		}
		f, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, `{"error":"no file uploaded"}`, http.StatusBadRequest)
			return
		}
		defer f.Close()
		file = f
		fileSize = header.Size
	} else {
		// Raw body upload — the file bytes are the entire request body.
		// Limit to 5 MB.
		r.Body = http.MaxBytesReader(w, r.Body, 5*1024*1024)
		file = r.Body
		fileSize = -1 // unknown, rely on MaxBytesReader
	}

	// Validate file size (only for multipart where we know it upfront).
	if fileSize > 5*1024*1024 {
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

// HandleAdminUpload handles file uploads from the admin dashboard.
// Accepts PUT with raw file bytes and Content-Type header.
// Uses admin/ prefix for the stored key to separate from employer uploads.
func (l *LocalClient) HandleAdminUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		http.Error(w, `{"error":"missing Content-Type header"}`, http.StatusBadRequest)
		return
	}

	allowedTypes := map[string]bool{
		"image/png":     true,
		"image/jpeg":    true,
		"image/webp":    true,
		"image/svg+xml": true,
	}
	if !allowedTypes[contentType] {
		http.Error(w, `{"error":"unsupported content type; allowed: image/png, image/jpeg, image/webp, image/svg+xml"}`, http.StatusBadRequest)
		return
	}

	extMap := map[string]string{
		"image/png":     "png",
		"image/jpeg":    "jpeg",
		"image/webp":    "webp",
		"image/svg+xml": "svg",
	}
	ext := extMap[contentType]

	r.Body = http.MaxBytesReader(w, r.Body, 5*1024*1024)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":"file too large (max 5MB)"}`, http.StatusRequestEntityTooLarge)
		return
	}
	if len(body) == 0 {
		http.Error(w, `{"error":"empty file"}`, http.StatusBadRequest)
		return
	}

	key := fmt.Sprintf("admin/%s.%s", uuid.New().String(), ext)

	dir := filepath.Join(l.baseDir, filepath.Dir(key))
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, `{"error":"failed to create directory"}`, http.StatusInternalServerError)
		return
	}

	filePath := filepath.Join(l.baseDir, key)
	if err := os.WriteFile(filePath, body, 0644); err != nil {
		http.Error(w, `{"error":"failed to save file"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"public_url": l.PublicURL(key),
	})
}