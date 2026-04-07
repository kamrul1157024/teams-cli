package api

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AppConfig holds persistent CLI configuration
type AppConfig struct {
	Signature        string `json:"signature"`         // Appended to sent messages (e.g. "sent via claude")
	SignatureEnabled bool   `json:"signature_enabled"` // Whether to append signature
}

// DefaultConfig returns the default configuration
func DefaultConfig() AppConfig {
	return AppConfig{
		Signature:        "sent via claude 🤖",
		SignatureEnabled: true,
	}
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "teams-cli")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// LoadConfig reads the config file, returning defaults if it doesn't exist
func LoadConfig() AppConfig {
	path, err := configPath()
	if err != nil {
		return DefaultConfig()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultConfig()
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig()
	}

	// Ensure signature has a default if empty
	if cfg.Signature == "" {
		cfg.Signature = DefaultConfig().Signature
	}

	return cfg
}

// SaveConfig writes the config to disk
func SaveConfig(cfg AppConfig) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}
