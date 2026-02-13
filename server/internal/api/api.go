package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Fileri/share/server/internal/config"
	"github.com/Fileri/share/server/internal/render"
	"github.com/Fileri/share/server/internal/storage"
)

// Handler is the main API handler
type Handler struct {
	config      *config.Config
	storage     storage.Storage
	mux         *http.ServeMux
	webdav      *WebDAVHandler
	maxFileSize int64 // 0 means unlimited
}

// New creates a new API handler
func New(cfg *config.Config, store storage.Storage) *Handler {
	maxFileSize := parseSize(cfg.Limits.MaxFileSize)

	h := &Handler{
		config:      cfg,
		storage:     store,
		mux:         http.NewServeMux(),
		webdav:      NewWebDAV(store, cfg.Auth.Tokens, maxFileSize),
		maxFileSize: maxFileSize,
	}

	h.setupRoutes()
	return h
}

// parseSize converts size strings like "100MB", "1GB" to bytes
func parseSize(s string) int64 {
	if s == "" || s == "0" {
		return 0 // unlimited
	}

	s = strings.ToUpper(strings.TrimSpace(s))
	multiplier := int64(1)

	switch {
	case strings.HasSuffix(s, "GB"):
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GB")
	case strings.HasSuffix(s, "MB"):
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	case strings.HasSuffix(s, "KB"):
		multiplier = 1024
		s = strings.TrimSuffix(s, "KB")
	case strings.HasSuffix(s, "B"):
		s = strings.TrimSuffix(s, "B")
	}

	var size int64
	fmt.Sscanf(strings.TrimSpace(s), "%d", &size)
	return size * multiplier
}

func (h *Handler) setupRoutes() {
	h.mux.HandleFunc("/", h.handleRoot)
	h.mux.HandleFunc("/api/upload", h.handleUpload)
	h.mux.HandleFunc("/api/list", h.handleList)
	h.mux.HandleFunc("/api/delete/", h.handleDelete)
	h.mux.HandleFunc("/robots.txt", h.handleRobots)
	h.mux.Handle("/webdav/", h.webdav)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add security headers
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	// CSP: Allow scripts from CDN for highlight.js, marked, DOMPurify
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'; style-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none';")

	h.mux.ServeHTTP(w, r)
}

func (h *Handler) handleRoot(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Root path - show simple info
	if path == "/" {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("share - CLI-first file sharing\nhttps://github.com/Fileri/share\n"))
		return
	}

	// Parse path: /<id> or /<id>/raw or /<id>/render
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || len(parts) > 2 {
		http.NotFound(w, r)
		return
	}

	id := parts[0]
	viewMode := ""
	if len(parts) == 2 {
		viewMode = parts[1]
	}

	h.serveFile(w, r, id, viewMode)
}

func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, id string, viewMode string) {
	ctx := r.Context()

	content, item, err := h.storage.Get(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer content.Close()

	// Determine if we should render
	shouldRender := false
	switch viewMode {
	case "raw":
		shouldRender = false
	case "render":
		shouldRender = true
	default:
		// Use item's render mode preference
		shouldRender = item.RenderMode != "raw" && render.CanRender(item.ContentType)
	}

	if shouldRender {
		// Read content for rendering
		data, err := io.ReadAll(content)
		if err != nil {
			http.Error(w, "Failed to read content", http.StatusInternalServerError)
			return
		}

		rendered, err := render.Render(item.ContentType, data, item.Filename, id)
		if err != nil {
			// Fall back to raw
			w.Header().Set("Content-Type", item.ContentType)
			w.Write(data)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(rendered)
		return
	}

	// Serve raw file
	w.Header().Set("Content-Type", item.ContentType)
	if item.Filename != "" {
		// Safely format Content-Disposition to prevent header injection
		disposition := mime.FormatMediaType("inline", map[string]string{"filename": item.Filename})
		w.Header().Set("Content-Disposition", disposition)
	}
	io.Copy(w, content)
}

func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check authentication
	token := r.Header.Get("Authorization")
	if token == "" {
		token = r.Header.Get("X-Share-Token")
	}
	token = strings.TrimPrefix(token, "Bearer ")

	if !h.isValidToken(token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Enforce file size limit
	if h.maxFileSize > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, h.maxFileSize)
	}

	// Parse multipart form or read raw body
	var content io.Reader
	var filename string
	var contentType string

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "No file provided", http.StatusBadRequest)
			return
		}
		defer file.Close()
		content = file
		filename = header.Filename
		contentType = header.Header.Get("Content-Type")
	} else {
		content = r.Body
		filename = r.URL.Query().Get("filename")
		contentType = r.Header.Get("Content-Type")
	}

	// Detect content type if not provided
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = detectContentType(filename, nil)
	}

	// Get render mode from query param
	renderMode := r.URL.Query().Get("render")
	if renderMode == "" {
		renderMode = "auto"
	}

	// Generate ID
	id := generateID()

	// Create item
	item := &storage.Item{
		ID:          id,
		Filename:    filename,
		ContentType: contentType,
		RenderMode:  renderMode,
		CreatedAt:   time.Now().UTC(),
		OwnerToken:  token,
	}

	// Store
	if err := h.storage.Put(r.Context(), id, content, item); err != nil {
		log.Printf("Failed to store file: %v", err)
		http.Error(w, "Failed to store file", http.StatusInternalServerError)
		return
	}

	// Return URL
	url := h.config.BaseURL + "/" + id
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(url + "\n"))
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.Header.Get("Authorization")
	if token == "" {
		token = r.Header.Get("X-Share-Token")
	}
	token = strings.TrimPrefix(token, "Bearer ")

	if !h.isValidToken(token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	items, err := h.storage.List(r.Context(), token)
	if err != nil {
		log.Printf("Failed to list items: %v", err)
		http.Error(w, "Failed to list items", http.StatusInternalServerError)
		return
	}

	// Build response with proper JSON encoding
	type listItem struct {
		ID       string `json:"id"`
		URL      string `json:"url"`
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
		Created  string `json:"created"`
	}

	response := make([]listItem, len(items))
	for i, item := range items {
		response[i] = listItem{
			ID:       item.ID,
			URL:      h.config.BaseURL + "/" + item.ID,
			Filename: item.Filename,
			Size:     item.Size,
			Created:  item.CreatedAt.Format(time.RFC3339),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.Header.Get("Authorization")
	if token == "" {
		token = r.Header.Get("X-Share-Token")
	}
	token = strings.TrimPrefix(token, "Bearer ")

	if !h.isValidToken(token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get ID from path
	id := strings.TrimPrefix(r.URL.Path, "/api/delete/")
	if id == "" {
		http.Error(w, "No ID provided", http.StatusBadRequest)
		return
	}

	// Check ownership
	item, err := h.storage.GetMeta(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if item.OwnerToken != token {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := h.storage.Delete(r.Context(), id); err != nil {
		log.Printf("Failed to delete item: %v", err)
		http.Error(w, "Failed to delete", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("User-agent: *\nDisallow: /\n"))
}

func (h *Handler) isValidToken(token string) bool {
	if token == "" {
		return false
	}
	// Use constant-time comparison to prevent timing attacks
	for _, t := range h.config.Auth.Tokens {
		if subtle.ConstantTimeCompare([]byte(t), []byte(token)) == 1 {
			return true
		}
	}
	return false
}

func generateID() string {
	bytes := make([]byte, 12) // 24 hex chars
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func detectContentType(filename string, content []byte) string {
	// Try by extension first
	if filename != "" {
		ext := filepath.Ext(filename)
		if mimeType := mime.TypeByExtension(ext); mimeType != "" {
			return mimeType
		}

		// Handle common extensions not in mime database
		switch strings.ToLower(ext) {
		case ".md", ".markdown":
			return "text/markdown"
		case ".ts":
			return "text/typescript"
		case ".tsx":
			return "text/tsx"
		case ".jsx":
			return "text/jsx"
		case ".yaml", ".yml":
			return "text/yaml"
		case ".toml":
			return "text/toml"
		case ".go":
			return "text/x-go"
		case ".rs":
			return "text/x-rust"
		case ".py":
			return "text/x-python"
		case ".rb":
			return "text/x-ruby"
		case ".sh", ".bash":
			return "text/x-shellscript"
		case ".sql":
			return "text/x-sql"
		case ".dockerfile":
			return "text/x-dockerfile"
		}
	}

	// Try detecting from content
	if len(content) > 0 {
		return http.DetectContentType(content)
	}

	return "text/plain"
}

func formatSize(size int64) string {
	return fmt.Sprintf("%d", size)
}
