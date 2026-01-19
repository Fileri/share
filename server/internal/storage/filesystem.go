package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Filesystem implements Storage using the local filesystem
type Filesystem struct {
	basePath string
}

// NewFilesystem creates a new filesystem storage backend
func NewFilesystem(basePath string) (*Filesystem, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Create subdirectories for data and metadata
	for _, sub := range []string{"files", "meta"} {
		if err := os.MkdirAll(filepath.Join(basePath, sub), 0755); err != nil {
			return nil, fmt.Errorf("failed to create %s directory: %w", sub, err)
		}
	}

	return &Filesystem{basePath: basePath}, nil
}

func (f *Filesystem) filePath(id string) string {
	return filepath.Join(f.basePath, "files", id)
}

func (f *Filesystem) metaPath(id string) string {
	return filepath.Join(f.basePath, "meta", id+".json")
}

// Put stores a file and its metadata
func (f *Filesystem) Put(ctx context.Context, id string, content io.Reader, item *Item) error {
	// Write file content
	file, err := os.Create(f.filePath(id))
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	size, err := io.Copy(file, content)
	if err != nil {
		os.Remove(f.filePath(id))
		return fmt.Errorf("failed to write file: %w", err)
	}
	item.Size = size

	// Write metadata
	metaFile, err := os.Create(f.metaPath(id))
	if err != nil {
		os.Remove(f.filePath(id))
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer metaFile.Close()

	if err := json.NewEncoder(metaFile).Encode(item); err != nil {
		os.Remove(f.filePath(id))
		os.Remove(f.metaPath(id))
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// Get retrieves a file and its metadata
func (f *Filesystem) Get(ctx context.Context, id string) (io.ReadCloser, *Item, error) {
	item, err := f.GetMeta(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	file, err := os.Open(f.filePath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("file not found")
		}
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, item, nil
}

// GetMeta retrieves only the metadata
func (f *Filesystem) GetMeta(ctx context.Context, id string) (*Item, error) {
	metaFile, err := os.Open(f.metaPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("item not found")
		}
		return nil, fmt.Errorf("failed to open metadata: %w", err)
	}
	defer metaFile.Close()

	var item Item
	if err := json.NewDecoder(metaFile).Decode(&item); err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	return &item, nil
}

// Delete removes a file and its metadata
func (f *Filesystem) Delete(ctx context.Context, id string) error {
	// Remove both files, ignore errors if they don't exist
	os.Remove(f.filePath(id))
	os.Remove(f.metaPath(id))
	return nil
}

// List returns all items for a given owner token
func (f *Filesystem) List(ctx context.Context, ownerToken string) ([]*Item, error) {
	metaDir := filepath.Join(f.basePath, "meta")
	entries, err := os.ReadDir(metaDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata directory: %w", err)
	}

	var items []*Item
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-5] // remove .json
		item, err := f.GetMeta(ctx, id)
		if err != nil {
			continue
		}

		// Filter by owner token
		if item.OwnerToken == ownerToken {
			items = append(items, item)
		}
	}

	// Sort by creation time, newest first
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	return items, nil
}
