package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/anthropics/m/internal/api"
	"github.com/anthropics/m/internal/config"
	"github.com/anthropics/m/internal/store"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the M server",
	Long:  `Start the M HTTP server with the configured settings.`,
	RunE:  runServe,
}

var serveConfigPath string

func init() {
	serveCmd.Flags().StringVarP(&serveConfigPath, "config", "c", "", "path to config file (default: ~/.m/config.yaml)")
}

func runServe(cmd *cobra.Command, args []string) error {
	// Determine config path
	cfgPath := serveConfigPath
	if cfgPath == "" {
		cfgPath = defaultConfigPath()
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	// Validate required fields
	if cfg.Server.APIKey == "" || cfg.Server.APIKey == "your-api-key-here" {
		log.Printf("warning: API key not configured, set M_API_KEY or use 'm config set server/api_key <key>'")
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

	log.Printf("Starting server on port %d", cfg.Server.Port)
	return srv.Run()
}
