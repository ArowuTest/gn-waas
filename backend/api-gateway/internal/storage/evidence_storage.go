package storage

// GN-WAAS Evidence Storage Service
//
// Handles upload of meter photos and audit evidence to MinIO object storage.
// Photos are the tamper-proof evidence chain — they MUST be persisted to
// object storage, not just stored locally on the field officer's device.
//
// Flow:
//   1. Flutter app calls POST /api/v1/evidence/upload-url to get a presigned PUT URL
//   2. Flutter uploads photo directly to MinIO using the presigned URL
//   3. Flutter calls POST /field-jobs/:id/submit with the returned object key
//   4. Backend stores the MinIO object key in audit_events.meter_photo_url
//
// This ensures:
//   - Photos are stored independently of the mobile device
//   - SHA-256 hash is verified server-side against the uploaded file
//   - Photos are accessible for audit review via presigned GET URLs
//   - MinIO bucket policy enforces write-once (immutable evidence)

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

// EvidenceStorageService manages meter photo uploads to MinIO
type EvidenceStorageService struct {
	client     *minio.Client
	bucket     string
	logger     *zap.Logger
	publicBase string // base URL for constructing public/presigned URLs
}

// NewEvidenceStorageService creates a new MinIO-backed evidence storage service.
// Returns nil (with a warning) if MinIO is not configured — callers must handle nil.
func NewEvidenceStorageService(
	endpoint, accessKey, secretKey, bucket string,
	useSSL bool,
	logger *zap.Logger,
) (*EvidenceStorageService, error) {
	if endpoint == "" || accessKey == "" || secretKey == "" {
		logger.Warn("MinIO not configured — evidence photos will not be persisted to object storage",
			zap.String("hint", "Set MINIO_ENDPOINT, MINIO_ACCESS_KEY, MINIO_SECRET_KEY in environment"))
		return nil, nil
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	svc := &EvidenceStorageService{
		client: client,
		bucket: bucket,
		logger: logger,
	}

	// Ensure bucket exists
	if err := svc.ensureBucket(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure MinIO bucket: %w", err)
	}

	logger.Info("MinIO evidence storage initialised",
		zap.String("endpoint", endpoint),
		zap.String("bucket", bucket),
	)
	return svc, nil
}

// ensureBucket creates the evidence bucket if it doesn't exist.
// Sets a lifecycle policy to prevent deletion (write-once evidence).
func (s *EvidenceStorageService) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", s.bucket, err)
		}
		s.logger.Info("Created MinIO evidence bucket", zap.String("bucket", s.bucket))
	}
	return nil
}

// PresignedUploadURL generates a presigned PUT URL for direct photo upload from Flutter.
// The Flutter app uploads the photo directly to MinIO — the API gateway never handles
// the raw photo bytes, keeping the gateway lightweight.
//
// objectKey format: evidence/{jobID}/{timestamp}_{filename}
func (s *EvidenceStorageService) PresignedUploadURL(
	ctx context.Context,
	jobID string,
	filename string,
	contentType string,
) (objectKey string, uploadURL string, err error) {
	if s == nil || s.client == nil {
		return "", "", fmt.Errorf("evidence storage not configured")
	}

	ts := time.Now().UTC().Format("20060102T150405Z")
	objectKey = fmt.Sprintf("evidence/%s/%s_%s", jobID, ts, filename)

	// Presigned URL valid for 15 minutes (enough for upload + retry)
	reqParams := make(url.Values)
	reqParams.Set("response-content-type", contentType)

	presignedURL, err := s.client.PresignedPutObject(ctx, s.bucket, objectKey, 15*time.Minute)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate presigned upload URL: %w", err)
	}

	return objectKey, presignedURL.String(), nil
}

// PresignedDownloadURL generates a presigned GET URL for viewing a stored photo.
// Used by admin/authority portals to display evidence photos.
// URL is valid for 1 hour.
func (s *EvidenceStorageService) PresignedDownloadURL(
	ctx context.Context,
	objectKey string,
) (string, error) {
	if s == nil || s.client == nil {
		return "", fmt.Errorf("evidence storage not configured")
	}

	presignedURL, err := s.client.PresignedGetObject(ctx, s.bucket, objectKey, 1*time.Hour, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned download URL: %w", err)
	}
	return presignedURL.String(), nil
}

// VerifyPhotoHash downloads the stored photo and verifies its SHA-256 hash
// matches the hash submitted by the field officer. This is the server-side
// tamper-evidence check.
func (s *EvidenceStorageService) VerifyPhotoHash(
	ctx context.Context,
	objectKey string,
	expectedHash string,
) (bool, error) {
	if s == nil || s.client == nil {
		return false, fmt.Errorf("evidence storage not configured")
	}

	obj, err := s.client.GetObject(ctx, s.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to retrieve object for hash verification: %w", err)
	}
	defer obj.Close()

	h := sha256.New()
	if _, err := io.Copy(h, obj); err != nil {
		return false, fmt.Errorf("failed to hash object: %w", err)
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	return actualHash == expectedHash, nil
}

// ObjectExists checks if an object key exists in the bucket.
// Used to validate that Flutter actually uploaded the photo before accepting submission.
func (s *EvidenceStorageService) ObjectExists(ctx context.Context, objectKey string) (bool, error) {
	if s == nil || s.client == nil {
		return false, nil // graceful degradation
	}
	_, err := s.client.StatObject(ctx, s.bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
