package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
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

type uploadURLRequest struct {
	Type        string `json:"type"`         // "logo" or "banner"
	ContentType string `json:"content_type"` // MIME type
	Ext         string `json:"ext"`          // file extension (png, jpg, etc.)
}

type uploadURLResponse struct {
	UploadURL string `json:"upload_url"`
	PublicURL string `json:"public_url"`
}

// UploadHandler provides the upload-url endpoint.
type UploadHandler struct {
	r2 *storage.R2Client
}

func NewUploadHandler(r2 *storage.R2Client) *UploadHandler {
	return &UploadHandler{r2: r2}
}

// GetUploadURL generates a presigned R2 upload URL for an authenticated client.
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
	uploadURL, err := h.r2.GeneratePresignedURL(r.Context(), key, req.ContentType, maxUploadBytes)
	if err != nil {
		http.Error(w, `{"error":"failed to generate upload url"}`, http.StatusInternalServerError)
		return
	}

	publicURL := h.r2.PublicURL(key)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(uploadURLResponse{
		UploadURL: uploadURL,
		PublicURL: publicURL,
	})
}
