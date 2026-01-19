package api

import (
	"crypto/rand"
	"encoding/hex"
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
	config  *config.Config
	storage storage.Storage
	mux     *http.ServeMux
}

// New creates a new API handler
func New(cfg *config.Config, store storage.Storage) *Handler {
	h := &Handler{
		config:  cfg,
		storage: store,
		mux:     http.NewServeMux(),
	}

	h.setupRoutes()
	return h
}

func (h *Handler) setupRoutes() {
	h.mux.HandleFunc("/", h.handleRoot)
	h.mux.HandleFunc("/api/upload", h.handleUpload)
	h.mux.HandleFunc("/api/list", h.handleList)
	h.mux.HandleFunc("/api/delete/", h.handleDelete)
	h.mux.HandleFunc("/robots.txt", h.handleRobots)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add security headers
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

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
		w.Header().Set("Content-Disposition", "inline; filename=\""+item.Filename+"\"")
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

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("[\n"))
	for i, item := range items {
		if i > 0 {
			w.Write([]byte(",\n"))
		}
		url := h.config.BaseURL + "/" + item.ID
		w.Write([]byte(`  {"id":"` + item.ID + `","url":"` + url + `","filename":"` + item.Filename + `","size":` + formatSize(item.Size) + `,"created":"` + item.CreatedAt.Format(time.RFC3339) + `"}`))
	}
	w.Write([]byte("\n]\n"))
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
	for _, t := range h.config.Auth.Tokens {
		if t == token {
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
