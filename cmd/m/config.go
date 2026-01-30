package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage M configuration",
	Long:  `View and modify M configuration stored in ~/.m/config.yaml.`,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value using dot-separated or slash-separated keys.

Examples:
  m config set server/api_key sk-ant-xxx
  m config set server/port 8080
  m config set storage/path /var/data/m.db`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current configuration from ~/.m/config.yaml.`,
	RunE:  runConfigShow,
}

func init() {
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configShowCmd)
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(home, ".m", "config.yaml")
}

func ensureConfigDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(home, ".m"), 0755)
}

func loadConfig() (map[string]any, error) {
	path := defaultConfigPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, err
	}

	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = make(map[string]any)
	}
	return cfg, nil
}

func saveConfig(cfg map[string]any) error {
	if err := ensureConfigDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(defaultConfigPath(), data, 0600)
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	// Normalize key: support both server/api_key and server.api_key
	key = strings.ReplaceAll(key, "/", ".")
	parts := strings.Split(key, ".")

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Navigate/create nested structure
	current := cfg
	for i, part := range parts[:len(parts)-1] {
		if _, exists := current[part]; !exists {
			current[part] = make(map[string]any)
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			return fmt.Errorf("cannot set nested key: %s is not a map", strings.Join(parts[:i+1], "."))
		}
		current = next
	}

	// Set the value (try to preserve types for common values)
	finalKey := parts[len(parts)-1]
	current[finalKey] = parseValue(value)

	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}

func parseValue(s string) any {
	// Try int
	var i int
	if n, _ := fmt.Sscanf(s, "%d", &i); n == 1 && fmt.Sprintf("%d", i) == s {
		return i
	}
	// Try bool
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	// Default to string
	return s
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	path := defaultConfigPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		fmt.Printf("No config file found at %s\n", path)
		fmt.Println("\nDefault values:")
		fmt.Println("  server.port: 8080")
		fmt.Println("  storage.path: ./data/m.db")
		fmt.Println("  workspaces.path: ./workspaces")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	fmt.Printf("Config file: %s\n\n", path)
	fmt.Print(string(data))
	return nil
}
