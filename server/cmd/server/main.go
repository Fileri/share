package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Fileri/share/server/internal/api"
	"github.com/Fileri/share/server/internal/config"
	"github.com/Fileri/share/server/internal/storage"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize storage
	store, err := storage.New(cfg.Storage)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create API handler
	handler := api.New(cfg, store)

	// Start server
	addr := cfg.ListenAddr
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("Starting share server on %s", addr)
	log.Printf("Base URL: %s", cfg.BaseURL)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func init() {
	// Set default log format
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Check for config file path from env
	if os.Getenv("SHARE_CONFIG") == "" {
		// Default config locations
		for _, path := range []string{"/etc/share/config.yaml", "./config.yaml"} {
			if _, err := os.Stat(path); err == nil {
				os.Setenv("SHARE_CONFIG", path)
				break
			}
		}
	}
}
