// Command m-server runs the M HTTP server.
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/anthropics/m/internal/api"
	"github.com/anthropics/m/internal/config"
	"github.com/anthropics/m/internal/store"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Validate required fields
	if cfg.Server.APIKey == "" || cfg.Server.APIKey == "your-api-key-here" {
		log.Printf("warning: API key not configured, set M_API_KEY environment variable")
	}

	// Log claude binary location for debugging
	claudeBin := cfg.Claude.FindClaudeBinary()
	log.Printf("using claude binary: %s", claudeBin)

	// Ensure data directory exists
	dbDir := filepath.Dir(cfg.Storage.Path)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("failed to create data directory: %v", err)
	}

	// Initialize store
	s, err := store.New(cfg.Storage.Path)
	if err != nil {
		log.Fatalf("failed to initialize store: %v", err)
	}
	defer s.Close()

	// Ensure workspaces directory exists
	if err := os.MkdirAll(cfg.Workspaces.Path, 0755); err != nil {
		log.Fatalf("failed to create workspaces directory: %v", err)
	}

	// Create and run server
	srv := api.New(api.Config{
		Port:           cfg.Server.Port,
		APIKey:         cfg.Server.APIKey,
		WorkspacesPath: cfg.Workspaces.Path,
		DemoMode:       cfg.Server.DemoMode,
	}, s)

	if err := srv.Run(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
