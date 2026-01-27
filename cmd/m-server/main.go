// Command m-server runs the M HTTP server.
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/anthropics/m/internal/api"
	"github.com/anthropics/m/internal/store"
	"gopkg.in/yaml.v3"
)

// Config represents the server configuration file.
type Config struct {
	Server struct {
		Port   int    `yaml:"port"`
		APIKey string `yaml:"api_key"`
	} `yaml:"server"`
	Storage struct {
		Path string `yaml:"path"`
	} `yaml:"storage"`
	Workspaces struct {
		Path string `yaml:"path"`
	} `yaml:"workspaces"`
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
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

	// Create and run server
	srv := api.New(api.Config{
		Port:   cfg.Server.Port,
		APIKey: cfg.Server.APIKey,
	}, s)

	if err := srv.Run(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func loadConfig(path string) (*Config, error) {
	cfg := &Config{} // defaults

	// Set defaults
	cfg.Server.Port = 8080
	cfg.Storage.Path = "./data/m.db"
	cfg.Workspaces.Path = "./workspaces"

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

	// Validate required fields
	if cfg.Server.APIKey == "" || cfg.Server.APIKey == "your-api-key-here" {
		log.Printf("warning: API key not configured, set M_API_KEY environment variable")
	}

	return cfg, nil
}
