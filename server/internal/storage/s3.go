package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Fileri/share/server/internal/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Storage implements Storage using S3-compatible backends
type S3Storage struct {
	client *s3.Client
	bucket string
}

// NewS3 creates a new S3 storage backend
func NewS3(cfg config.StorageConfig) (*S3Storage, error) {
	ctx := context.Background()

	// Build AWS config
	var opts []func(*awsconfig.LoadOptions) error

	// Custom endpoint for non-AWS S3 (GCS, MinIO, etc.)
	if cfg.Endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               cfg.Endpoint,
				HostnameImmutable: true,
			}, nil
		})
		opts = append(opts, awsconfig.WithEndpointResolverWithOptions(customResolver))
	}

	// Static credentials if provided
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	// Set region
	if cfg.Region != "" {
		opts = append(opts, awsconfig.WithRegion(cfg.Region))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true // Required for MinIO and some S3-compatible services
	})

	return &S3Storage{
		client: client,
		bucket: cfg.Bucket,
	}, nil
}

func (s *S3Storage) fileKey(id string) string {
	return "files/" + id
}

func (s *S3Storage) metaKey(id string) string {
	return "meta/" + id + ".json"
}

// Put stores a file and its metadata
func (s *S3Storage) Put(ctx context.Context, id string, content io.Reader, item *Item) error {
	// Read content into buffer to get size
	var buf bytes.Buffer
	size, err := io.Copy(&buf, content)
	if err != nil {
		return fmt.Errorf("failed to read content: %w", err)
	}
	item.Size = size

	// Upload file
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(s.fileKey(id)),
		Body:        bytes.NewReader(buf.Bytes()),
		ContentType: aws.String(item.ContentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	// Upload metadata
	metaBytes, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(s.metaKey(id)),
		Body:        bytes.NewReader(metaBytes),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		// Try to clean up the file
		s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(s.fileKey(id)),
		})
		return fmt.Errorf("failed to upload metadata: %w", err)
	}

	return nil
}

// Get retrieves a file and its metadata
func (s *S3Storage) Get(ctx context.Context, id string) (io.ReadCloser, *Item, error) {
	item, err := s.GetMeta(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.fileKey(id)),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get file: %w", err)
	}

	return result.Body, item, nil
}

// GetMeta retrieves only the metadata
func (s *S3Storage) GetMeta(ctx context.Context, id string) (*Item, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.metaKey(id)),
	})
	if err != nil {
		return nil, fmt.Errorf("item not found")
	}
	defer result.Body.Close()

	var item Item
	if err := json.NewDecoder(result.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("failed to decode metadata: %w", err)
	}

	return &item, nil
}

// Delete removes a file and its metadata
func (s *S3Storage) Delete(ctx context.Context, id string) error {
	// Delete both objects
	s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.fileKey(id)),
	})
	s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.metaKey(id)),
	})
	return nil
}

// List returns all items for a given owner token
func (s *S3Storage) List(ctx context.Context, ownerToken string) ([]*Item, error) {
	var items []*Item

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String("meta/"),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			if len(key) <= 10 { // "meta/" + ".json"
				continue
			}
			id := key[5 : len(key)-5] // remove "meta/" and ".json"

			item, err := s.GetMeta(ctx, id)
			if err != nil {
				continue
			}

			if item.OwnerToken == ownerToken {
				items = append(items, item)
			}
		}
	}

	return items, nil
}
