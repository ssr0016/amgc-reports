// Package storage puts public files (profile pictures) in Cloudflare R2
// through its S3-compatible API.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/ssr0016/pastoral-reports/backend/internal/config"
)

// R2 writes objects to one bucket and returns their public URLs.
type R2 struct {
	client    *s3.Client
	bucket    string
	publicURL string
}

// NewR2 returns a client for cfg; cfg must be Enabled.
func NewR2(cfg config.StorageConfig) *R2 {
	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)),
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
	})
	return &R2{client: client, bucket: cfg.Bucket, publicURL: cfg.PublicURL}
}

// Put stores data under key and returns its public URL.
func (r *R2) Put(ctx context.Context, key, contentType string, data []byte) (string, error) {
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("put %s: %w", key, err)
	}
	return r.publicURL + "/" + key, nil
}

// Delete removes an avatar served by this bucket, including URLs with old cache tokens.
func (r *R2) Delete(ctx context.Context, url string) error {
	objectURL, _, _ := strings.Cut(url, "?")
	key, ok := strings.CutPrefix(objectURL, r.publicURL+"/")
	if !ok || !strings.HasPrefix(key, "avatars/") {
		return fmt.Errorf("avatar URL does not belong to this bucket")
	}
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete avatar: %w", err)
	}
	return nil
}
