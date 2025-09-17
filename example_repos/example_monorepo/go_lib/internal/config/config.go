package config

import (
	"encoding/json"
	"os"
)

// Config represents application configuration
type Config struct {
	AppName string `json:"app_name"`
	Version string `json:"version"`
	Debug   bool   `json:"debug"`
}

// Load reads configuration from a JSON file
func Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// Save writes configuration to a JSON file
func (c *Config) Save(filename string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}