package storage

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2Client wraps an S3 client configured for Cloudflare R2.
type R2Client struct {
	client    *s3.Client
	bucket    string
	publicURL string
	accountID string
}

// NewR2Client initialises an S3 client pointed at Cloudflare R2.
// Required env vars: R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY,
// R2_BUCKET_NAME, R2_PUBLIC_URL.
func NewR2Client() (*R2Client, error) {
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKey := os.Getenv("R2_ACCESS_KEY_ID")
	secretKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	bucket := os.Getenv("R2_BUCKET_NAME")
	publicURL := os.Getenv("R2_PUBLIC_URL")

	if accountID == "" || accessKey == "" || secretKey == "" || bucket == "" || publicURL == "" {
		return nil, fmt.Errorf("R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_BUCKET_NAME, and R2_PUBLIC_URL must be set")
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("auto"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = &endpoint
		o.UsePathStyle = true
	})

	return &R2Client{client: client, bucket: bucket, publicURL: publicURL, accountID: accountID}, nil
}

// GeneratePresignedURL creates a presigned PUT URL that constrains uploads to
// the given content type and maximum size (in bytes). The URL expires in 15
// minutes.
func (r *R2Client) GeneratePresignedURL(ctx context.Context, key, contentType string, maxBytes int64) (string, error) {
	presignClient := s3.NewPresignClient(r.client)

	presignParams := &s3.PutObjectInput{
		Bucket:      &r.bucket,
		Key:         &key,
		ContentType: &contentType,
	}

	req, err := presignClient.PresignPutObject(ctx, presignParams,
		func(opts *s3.PresignOptions) {
			opts.Expires = 15 * time.Minute
		},
	)
	if err != nil {
		return "", fmt.Errorf("presign put object: %w", err)
	}

	return req.URL, nil
}

// PublicURL returns the full public URL for a given object key.
func (r *R2Client) PublicURL(key string) string {
	return fmt.Sprintf("%s/%s", r.publicURL, key)
}
