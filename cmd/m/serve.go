package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/anthropics/m/internal/api"
	"github.com/anthropics/m/internal/store"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the M server",
	Long:  `Start the M HTTP server with the configured settings.`,
	RunE:  runServe,
}

var configPath string

func init() {
	serveCmd.Flags().StringVarP(&configPath, "config", "c", "", "path to config file (default: ~/.m/config.yaml)")
}

// ServerConfig represents the server configuration file.
type ServerConfig struct {
	Server struct {
		Port     int    `yaml:"port"`
		APIKey   string `yaml:"api_key"`
		DemoMode bool   `yaml:"demo_mode"`
	} `yaml:"server"`
	Storage struct {
		Path string `yaml:"path"`
	} `yaml:"storage"`
	Workspaces struct {
		Path string `yaml:"path"`
	} `yaml:"workspaces"`
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg, err := loadServerConfig()
	if err != nil {
		return err
	}

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

func loadServerConfig() (*ServerConfig, error) {
	cfg := &ServerConfig{}

	// Set defaults
	cfg.Server.Port = 8080
	cfg.Storage.Path = "./data/m.db"
	cfg.Workspaces.Path = "./workspaces"

	// Determine config path
	path := configPath
	if path == "" {
		path = defaultConfigPath()
	}

	// Load config file if it exists
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	// Environment variable overrides
	if v := os.Getenv("M_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}
	if v := os.Getenv("M_API_KEY"); v != "" {
		cfg.Server.APIKey = v
	}
	if v := os.Getenv("M_DB_PATH"); v != "" {
		cfg.Storage.Path = v
	}
	if v := os.Getenv("M_WORKSPACES_PATH"); v != "" {
		cfg.Workspaces.Path = v
	}
	if v := os.Getenv("M_DEMO_MODE"); v != "" {
		cfg.Server.DemoMode = v == "true" || v == "1"
	}

	// Validate required fields
	if cfg.Server.APIKey == "" || cfg.Server.APIKey == "your-api-key-here" {
		log.Printf("warning: API key not configured, set M_API_KEY or use 'm config set server/api_key <key>'")
	}

	return cfg, nil
}
