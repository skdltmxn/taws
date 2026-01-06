package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// UserConfig holds user preferences that persist across sessions
type UserConfig struct {
	LastProfile  string `json:"last_profile,omitempty"`
	LastRegion   string `json:"last_region,omitempty"`
	LastResource string `json:"last_resource,omitempty"`
	Theme        string `json:"theme,omitempty"`
}

// getConfigDir returns the platform-specific config directory for taws
func getConfigDir() (string, error) {
	var baseDir string

	switch runtime.GOOS {
	case "windows":
		// Windows: %APPDATA%\taws (e.g., C:\Users\<user>\AppData\Roaming\taws)
		baseDir = os.Getenv("APPDATA")
		if baseDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			baseDir = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(baseDir, "taws"), nil
	default:
		// Unix-like (Linux, macOS): ~/.config/taws
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			configHome = filepath.Join(home, ".config")
		}
		return filepath.Join(configHome, "taws"), nil
	}
}

// getConfigPath returns the full path to the config file
func getConfigPath() (string, error) {
	dir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// LoadUserConfig loads user configuration from the config file
func LoadUserConfig() (*UserConfig, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return &UserConfig{}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &UserConfig{}, nil
		}
		return nil, err
	}

	var cfg UserConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		// If config is corrupted, return empty config
		return &UserConfig{}, nil
	}

	return &cfg, nil
}

// Save saves user configuration to the config file
func (c *UserConfig) Save() error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	// Ensure config directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// Update updates specific fields and saves
func (c *UserConfig) Update(profile, region, resource string) error {
	if profile != "" {
		c.LastProfile = profile
	}
	if region != "" {
		c.LastRegion = region
	}
	if resource != "" {
		c.LastResource = resource
	}
	return c.Save()
}
