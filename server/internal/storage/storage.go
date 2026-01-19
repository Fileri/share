package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Fileri/share/server/internal/config"
)

// Item represents a stored file
type Item struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename,omitempty"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	RenderMode  string    `json:"render_mode"` // "auto", "raw", "render"
	CreatedAt   time.Time `json:"created_at"`
	OwnerToken  string    `json:"owner_token,omitempty"` // stored but not exposed in API responses
}

// Storage defines the interface for file storage backends
type Storage interface {
	// Put stores a file and returns its metadata
	Put(ctx context.Context, id string, content io.Reader, item *Item) error

	// Get retrieves a file's content
	Get(ctx context.Context, id string) (io.ReadCloser, *Item, error)

	// GetMeta retrieves only metadata
	GetMeta(ctx context.Context, id string) (*Item, error)

	// Delete removes a file
	Delete(ctx context.Context, id string) error

	// List returns all items for a given owner token
	List(ctx context.Context, ownerToken string) ([]*Item, error)
}

// New creates a new storage backend based on configuration
func New(cfg config.StorageConfig) (Storage, error) {
	switch cfg.Type {
	case "filesystem", "":
		return NewFilesystem(cfg.Path)
	case "s3":
		return NewS3(cfg)
	default:
		return nil, fmt.Errorf("unknown storage type: %s", cfg.Type)
	}
}
