package api

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Fileri/share/server/internal/storage"
	"golang.org/x/net/webdav"
)

// WebDAVHandler wraps the storage backend for WebDAV access
type WebDAVHandler struct {
	storage     storage.Storage
	tokens      []string
	handler     *webdav.Handler
	maxFileSize int64
}

// NewWebDAV creates a new WebDAV handler
func NewWebDAV(store storage.Storage, tokens []string, maxFileSize int64) *WebDAVHandler {
	w := &WebDAVHandler{
		storage:     store,
		tokens:      tokens,
		maxFileSize: maxFileSize,
	}

	w.handler = &webdav.Handler{
		Prefix:     "/webdav",
		FileSystem: w,
		LockSystem: webdav.NewMemLS(),
	}

	return w
}

// ServeHTTP handles WebDAV requests with Basic authentication
func (w *WebDAVHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	// Extract token from Basic auth (username is ignored, password is the token)
	_, token, ok := r.BasicAuth()
	if !ok || !w.isValidToken(token) {
		rw.Header().Set("WWW-Authenticate", `Basic realm="share"`)
		http.Error(rw, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Store token in context for file operations
	ctx := context.WithValue(r.Context(), tokenContextKey, token)
	w.handler.ServeHTTP(rw, r.WithContext(ctx))
}

type contextKey string

const tokenContextKey contextKey = "owner_token"

func (w *WebDAVHandler) isValidToken(token string) bool {
	for _, t := range w.tokens {
		if subtle.ConstantTimeCompare([]byte(t), []byte(token)) == 1 {
			return true
		}
	}
	return false
}

// --- webdav.FileSystem implementation ---

func (w *WebDAVHandler) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	// Flat filesystem, no directories supported
	return os.ErrPermission
}

func (w *WebDAVHandler) RemoveAll(ctx context.Context, name string) error {
	name = cleanPath(name)
	if name == "" || name == "/" {
		return os.ErrPermission
	}

	token, _ := ctx.Value(tokenContextKey).(string)

	// Find item by filename
	item, err := w.findByFilename(ctx, token, name)
	if err != nil {
		return err
	}

	// Check ownership
	if item.OwnerToken != token {
		return os.ErrPermission
	}

	return w.storage.Delete(ctx, item.ID)
}

func (w *WebDAVHandler) Rename(ctx context.Context, oldName, newName string) error {
	// Renaming not supported in current storage model
	return os.ErrPermission
}

func (w *WebDAVHandler) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	name = cleanPath(name)

	// Root directory
	if name == "" || name == "/" {
		return &davFileInfo{
			name:    "/",
			size:    0,
			mode:    os.ModeDir | 0755,
			modTime: time.Now(),
			isDir:   true,
		}, nil
	}

	token, _ := ctx.Value(tokenContextKey).(string)
	item, err := w.findByFilename(ctx, token, name)
	if err != nil {
		return nil, err
	}

	return &davFileInfo{
		name:    item.Filename,
		size:    item.Size,
		mode:    0644,
		modTime: item.CreatedAt,
		isDir:   false,
	}, nil
}

func (w *WebDAVHandler) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	name = cleanPath(name)
	token, _ := ctx.Value(tokenContextKey).(string)

	// Root directory listing
	if name == "" || name == "/" {
		return w.openRootDir(ctx, token)
	}

	// Write mode - create new file
	if flag&os.O_CREATE != 0 || flag&os.O_WRONLY != 0 || flag&os.O_RDWR != 0 {
		return w.createFile(ctx, token, name)
	}

	// Read mode - open existing file
	return w.openFile(ctx, token, name)
}

func (w *WebDAVHandler) openRootDir(ctx context.Context, token string) (webdav.File, error) {
	items, err := w.storage.List(ctx, token)
	if err != nil {
		return nil, err
	}

	var children []os.FileInfo
	for _, item := range items {
		name := item.Filename
		if name == "" {
			name = item.ID // Use ID if no filename
		}
		children = append(children, &davFileInfo{
			name:    name,
			size:    item.Size,
			mode:    0644,
			modTime: item.CreatedAt,
			isDir:   false,
		})
	}

	return &davDir{
		info:     &davFileInfo{name: "/", mode: os.ModeDir | 0755, modTime: time.Now(), isDir: true},
		children: children,
	}, nil
}

func (w *WebDAVHandler) openFile(ctx context.Context, token, name string) (webdav.File, error) {
	item, err := w.findByFilename(ctx, token, name)
	if err != nil {
		return nil, err
	}

	content, _, err := w.storage.Get(ctx, item.ID)
	if err != nil {
		return nil, os.ErrNotExist
	}

	// Read all content into memory (WebDAV needs seeking)
	data, err := io.ReadAll(content)
	content.Close()
	if err != nil {
		return nil, err
	}

	return &davFile{
		info:   &davFileInfo{name: item.Filename, size: item.Size, mode: 0644, modTime: item.CreatedAt},
		reader: bytes.NewReader(data),
	}, nil
}

func (w *WebDAVHandler) createFile(ctx context.Context, token, name string) (webdav.File, error) {
	return &davWriteFile{
		name:        name,
		token:       token,
		storage:     w.storage,
		buffer:      &bytes.Buffer{},
		maxFileSize: w.maxFileSize,
	}, nil
}

func (w *WebDAVHandler) findByFilename(ctx context.Context, token, name string) (*storage.Item, error) {
	items, err := w.storage.List(ctx, token)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		itemName := item.Filename
		if itemName == "" {
			itemName = item.ID
		}
		if itemName == name {
			return item, nil
		}
	}

	return nil, os.ErrNotExist
}

func cleanPath(name string) string {
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimSuffix(name, "/")
	return name
}

// --- FileInfo implementation ---

type davFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (fi *davFileInfo) Name() string       { return fi.name }
func (fi *davFileInfo) Size() int64        { return fi.size }
func (fi *davFileInfo) Mode() os.FileMode  { return fi.mode }
func (fi *davFileInfo) ModTime() time.Time { return fi.modTime }
func (fi *davFileInfo) IsDir() bool        { return fi.isDir }
func (fi *davFileInfo) Sys() interface{}   { return nil }

// --- Directory implementation ---

type davDir struct {
	info     *davFileInfo
	children []os.FileInfo
	pos      int
}

func (d *davDir) Close() error                             { return nil }
func (d *davDir) Read(p []byte) (int, error)               { return 0, os.ErrInvalid }
func (d *davDir) Write(p []byte) (int, error)              { return 0, os.ErrInvalid }
func (d *davDir) Seek(offset int64, whence int) (int64, error) { return 0, os.ErrInvalid }
func (d *davDir) Stat() (os.FileInfo, error)               { return d.info, nil }

func (d *davDir) Readdir(count int) ([]os.FileInfo, error) {
	if d.pos >= len(d.children) {
		if count <= 0 {
			return nil, nil
		}
		return nil, io.EOF
	}

	if count <= 0 || count > len(d.children)-d.pos {
		count = len(d.children) - d.pos
	}

	result := d.children[d.pos : d.pos+count]
	d.pos += count
	return result, nil
}

// --- Read-only file implementation ---

type davFile struct {
	info   *davFileInfo
	reader *bytes.Reader
}

func (f *davFile) Close() error                                  { return nil }
func (f *davFile) Read(p []byte) (int, error)                    { return f.reader.Read(p) }
func (f *davFile) Write(p []byte) (int, error)                   { return 0, os.ErrInvalid }
func (f *davFile) Seek(offset int64, whence int) (int64, error)  { return f.reader.Seek(offset, whence) }
func (f *davFile) Stat() (os.FileInfo, error)                    { return f.info, nil }
func (f *davFile) Readdir(count int) ([]os.FileInfo, error)      { return nil, os.ErrInvalid }

// --- Write file implementation ---

type davWriteFile struct {
	name        string
	token       string
	storage     storage.Storage
	buffer      *bytes.Buffer
	maxFileSize int64
	closed      bool
}

func (f *davWriteFile) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	// Generate ID and save file
	id := generateID()
	item := &storage.Item{
		ID:          id,
		Filename:    f.name,
		ContentType: detectContentType(f.name, f.buffer.Bytes()),
		RenderMode:  "auto",
		CreatedAt:   time.Now().UTC(),
		OwnerToken:  f.token,
	}

	return f.storage.Put(context.Background(), id, f.buffer, item)
}

func (f *davWriteFile) Read(p []byte) (int, error) { return 0, os.ErrInvalid }

func (f *davWriteFile) Write(p []byte) (int, error) {
	if f.maxFileSize > 0 && int64(f.buffer.Len()+len(p)) > f.maxFileSize {
		return 0, errors.New("file too large")
	}
	return f.buffer.Write(p)
}

func (f *davWriteFile) Seek(offset int64, whence int) (int64, error) { return 0, os.ErrInvalid }

func (f *davWriteFile) Stat() (os.FileInfo, error) {
	return &davFileInfo{
		name:    f.name,
		size:    int64(f.buffer.Len()),
		mode:    0644,
		modTime: time.Now(),
	}, nil
}

func (f *davWriteFile) Readdir(count int) ([]fs.FileInfo, error) { return nil, os.ErrInvalid }
