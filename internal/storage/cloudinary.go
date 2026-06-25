package storage

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// CloudinaryClient wraps Cloudinary SDK for image uploads.
type CloudinaryClient struct {
	cld *cloudinary.Cloudinary
}

// NewCloudinaryClient initializes Cloudinary client from env vars.
// Required: CLOUDINARY_CLOUD_NAME, CLOUDINARY_API_KEY, CLOUDINARY_API_SECRET.
func NewCloudinaryClient() (*CloudinaryClient, error) {
	cloudName := os.Getenv("CLOUDINARY_CLOUD_NAME")
	apiKey := os.Getenv("CLOUDINARY_API_KEY")
	apiSecret := os.Getenv("CLOUDINARY_API_SECRET")

	if cloudName == "" || apiKey == "" || apiSecret == "" {
		return nil, fmt.Errorf("CLOUDINARY_CLOUD_NAME, CLOUDINARY_API_KEY, CLOUDINARY_API_SECRET must be set")
	}

	cld, err := cloudinary.NewFromParams(cloudName, apiKey, apiSecret)
	if err != nil {
		return nil, fmt.Errorf("cloudinary init: %w", err)
	}

	return &CloudinaryClient{cld: cld}, nil
}

// GeneratePresignedURL creates a signed upload URL for direct browser upload.
// Uses Cloudinary's unsigned upload preset (create one in dashboard: Settings → Upload → Upload presets).
// If no preset configured, falls back to signed upload (requires backend proxy).
func (c *CloudinaryClient) GeneratePresignedURL(ctx context.Context, key, contentType string, maxBytes int64) (string, error) {
	// Option 1: Use unsigned upload preset (recommended for direct browser uploads)
	// Create preset in Cloudinary dashboard: Settings → Upload → Upload presets → "unsigned"
	preset := os.Getenv("CLOUDINARY_UPLOAD_PRESET")
	if preset != "" {
		// Return the Cloudinary upload endpoint + preset for direct browser POST
		return fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/upload", c.cld.Config.Cloud.CloudName), nil
	}

	// Option 2: Signed upload (backend must proxy) - return a token endpoint
	// For now, return a backend endpoint that will handle signed upload
	return "/api/auth/cloudinary-upload", nil
}

// PublicURL returns the Cloudinary delivery URL for a given public ID.
func (c *CloudinaryClient) PublicURL(key string) string {
	asset, _ := c.cld.Image(key)
	return asset.String()
}

// UploadDirect handles signed upload via backend (for admin or when preset not used).
func (c *CloudinaryClient) UploadDirect(ctx context.Context, key, contentType string, file []byte) (string, error) {
	res, err := c.cld.Upload.Upload(ctx, file, uploader.UploadParams{
		PublicID:     key,
		ResourceType: "image",
		Folder:       "wirehire",
		// Auto-format, quality optimization
		Transformation: "f_auto,q_auto",
	})
	if err != nil {
		return "", fmt.Errorf("cloudinary upload: %w", err)
	}
	return res.SecureURL, nil
}

// ValidateContentType checks if content type is allowed.
func (c *CloudinaryClient) ValidateContentType(contentType string) error {
	allowed := map[string]bool{
		"image/png":     true,
		"image/jpeg":    true,
		"image/webp":    true,
		"image/svg+xml": true,
	}
	if !allowed[contentType] {
		return fmt.Errorf("unsupported content type: %s", contentType)
	}
	return nil
}

// ExtractPublicIDFromURL extracts the public_id from a Cloudinary URL.
func (c *CloudinaryClient) ExtractPublicIDFromURL(url string) string {
	// URL format: https://res.cloudinary.com/<cloud>/image/upload/v123456/folder/file.jpg
	parts := strings.Split(url, "/upload/")
	if len(parts) != 2 {
		return ""
	}
	// Remove version prefix (v123456/) and extension
	path := parts[1]
	if idx := strings.Index(path, "/"); idx != -1 {
		path = path[idx+1:]
	}
	// Remove extension
	if idx := strings.LastIndex(path, "."); idx != -1 {
		path = path[:idx]
	}
	return path
}