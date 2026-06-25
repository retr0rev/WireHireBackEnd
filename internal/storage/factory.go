package storage

import (
	"context"
	"os"
)

// StorageClient interface abstracts the storage backend.
type StorageClient interface {
	GeneratePresignedURL(ctx context.Context, key, contentType string, maxBytes int64) (string, error)
	PublicURL(key string) string
}

// NewStorageClient creates a storage client based on available configuration.
// Priority: Cloudinary > R2 > Local
func NewStorageClient() (StorageClient, error) {
	// Check if Cloudinary is fully configured
	cloudName := os.Getenv("CLOUDINARY_CLOUD_NAME")
	apiKey := os.Getenv("CLOUDINARY_API_KEY")
	apiSecret := os.Getenv("CLOUDINARY_API_SECRET")

	if cloudName != "" && apiKey != "" && apiSecret != "" {
		return NewCloudinaryClient()
	}

	// Check if R2 is fully configured
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKey := os.Getenv("R2_ACCESS_KEY_ID")
	secretKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	bucket := os.Getenv("R2_BUCKET_NAME")
	publicURL := os.Getenv("R2_PUBLIC_URL")

	if accountID != "" && accessKey != "" && secretKey != "" && bucket != "" && publicURL != "" {
		return NewR2Client()
	}

	// Fall back to local storage
	return NewLocalClient()
}