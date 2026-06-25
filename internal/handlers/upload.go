package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"jobapps/internal/middleware"
	"jobapps/internal/storage"

	"github.com/google/uuid"
)

// Allowed image upload content types.
var allowedContentTypes = map[string]string{
	"image/png":     "png",
	"image/jpeg":    "jpeg",
	"image/jpg":     "jpg",
	"image/webp":    "webp",
	"image/svg+xml": "svg",
}

const maxUploadBytes int64 = 5 * 1024 * 1024 // 5 MB

type cloudinaryUploadResponse struct {
	PublicURL string `json:"public_url"`
	Key       string `json:"key"`
	Error     string `json:"error,omitempty"`
}

func (h *UploadHandler) CloudinaryUpload(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	var fileData []byte
	var err error

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, `{"error":"failed to parse multipart form"}`, http.StatusBadRequest)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, `{"error":"no file uploaded"}`, http.StatusBadRequest)
			return
		}
		defer file.Close()

		fileData, err = io.ReadAll(file)
		if err != nil {
			http.Error(w, `{"error":"failed to read file"}`, http.StatusInternalServerError)
			return
		}
		if int64(len(fileData)) > maxUploadBytes {
			http.Error(w, `{"error":"file too large (max 5MB)"}`, http.StatusBadRequest)
			return
		}
		ext := strings.TrimPrefix(header.Filename, "admin/")
		if ext == "" {
			hext = ".png"
		}
		key := fmt.Sprintf("admin/%s%s", uuid.New().String(), ext)
		resp := h.uploadToCloudinary(fileData, key, contentType, r)
		if resp.Error != "" {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, resp.Error), http.StatusBadRequest)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	} else {
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
		fileData, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":"file too large (max 5MB)"}`, http.StatusRequestEntityTooLarge)
			return
		}
		if len(fileData) == 0 {
			http.Error(w, `{"error":"empty file"}`, http.StatusBadRequest)
			return
		}
		key := fmt.Sprintf("admin/%s.%s", uuid.New().String(), "upload")
		tp := contentType
		if p := strings.SplitN(contentType, "/", 2); len(p) == 2 {
			p[1] = strings.ReplaceAll(p[1], "+", ".")
			tp = p[0] + "/" + p[1]
		}
		resp := h.uploadToCloudinary(fileData, key, tp, r)
		if resp.Error != "" {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, resp.Error), http.StatusBadRequest)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}
}

func (h *UploadHandler) uploadToCloudinary(fileData []byte, key, contentType string, r *http.Request) cloudinaryUploadResponse {
	// The handler's store could be Cloudinary if configured
	cloudinaryStore, ok := h.store.(*storage.CloudinaryClient)
	if !ok {
		return cloudinaryUploadResponse{Error: "Storage is not Cloudinary"}
	}
	
	// Validate content type
	if err := cloudinaryStore.ValidateContentType(contentType); err != nil {
		return cloudinaryUploadResponse{Error: err.Error()}
	}
	
	// Upload to Cloudinary
	publicURL, err := cloudinaryStore.UploadDirect(r.Context(), key, contentType, fileData)
	if err != nil {
		return cloudinaryUploadResponse{Error: fmt.Sprintf("Cloudinary upload failed: %v", err)}
	}
	
	return cloudinaryUploadResponse{
		PublicURL: publicURL,
		Key:       key,
	}
}

const maxUploadBytes int64 = 5 * 1024 * 1024 // 5 MB

type cloudinaryUploadResponse struct {
	PublicURL string `json:"public_url"`
	Key       string `json:"key"`
	Error     string `json:"error,omitempty"`
}

type uploadURLRequest struct {
	Type        string `json:"type"`         // "logo" or "banner"
	ContentType string `json:"content_type"` // MIME type
	Ext         string `json:"ext"`          // file extension (png, jpg, etc.)
}

type uploadURLResponse struct {
	UploadURL       string `json:"upload_url"`
	PublicURL       string `json:"public_url"`
	CloudinaryPreset string `json:"cloudinary_preset,omitempty"`
}

// UploadHandler provides the upload-url endpoint.
type UploadHandler struct {
	store storage.StorageClient
}

func NewUploadHandler(store storage.StorageClient) *UploadHandler {
	return &UploadHandler{store: store}
}

// GetUploadURL generates a presigned upload URL for an authenticated client.
func (h *UploadHandler) GetUploadURL(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)

	var req uploadURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.Type = strings.TrimSpace(strings.ToLower(req.Type))
	req.ContentType = strings.TrimSpace(strings.ToLower(req.ContentType))
	req.Ext = strings.TrimSpace(strings.ToLower(req.Ext))

	// Validate type.
	if req.Type != "logo" && req.Type != "banner" {
		http.Error(w, `{"error":"type must be 'logo' or 'banner'"}`, http.StatusBadRequest)
		return
	}

	// Validate content type against allowed list.
	expectedExt, ok := allowedContentTypes[req.ContentType]
	if !ok {
		http.Error(w, `{"error":"unsupported content type; allowed: image/png, image/jpeg, image/webp, image/svg+xml"}`, http.StatusBadRequest)
		return
	}

	// Validate extension matches content type.
	if req.Ext != expectedExt && !(req.ContentType == "image/jpeg" && (req.Ext == "jpeg" || req.Ext == "jpg")) {
		http.Error(w, fmt.Sprintf(`{"error":"extension '%s' does not match content type '%s'"}`, req.Ext, req.ContentType), http.StatusBadRequest)
		return
	}

	clientID := middleware.GetClientID(r)
	if clientID == 0 {
		http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
		return
	}

	key := fmt.Sprintf("%d/%s/%s.%s", clientID, req.Type, uuid.New().String(), req.Ext)
	uploadURL, err := h.store.GeneratePresignedURL(r.Context(), key, req.ContentType, maxUploadBytes)
	if err != nil {
		http.Error(w, `{"error":"failed to generate upload url"}`, http.StatusInternalServerError)
		return
	}

	publicURL := h.store.PublicURL(key)

	// Check if using Cloudinary (unsigned preset flow)
	preset := os.Getenv("CLOUDINARY_UPLOAD_PRESET")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(uploadURLResponse{
		UploadURL:        uploadURL,
		PublicURL:        publicURL,
		CloudinaryPreset: preset,
	})
}