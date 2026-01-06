package config

import (
	"flag"
	"os"
)

// Config holds the application configuration
type Config struct {
	Region       string
	Profile      string
	LastResource string
	UserConfig   *UserConfig
}

// Load loads the configuration from environment variables, command-line flags, and user config
func Load() (*Config, error) {
	cfg := &Config{}

	// Load user config first (lowest priority)
	userCfg, err := LoadUserConfig()
	if err != nil {
		userCfg = &UserConfig{}
	}
	cfg.UserConfig = userCfg

	// Apply user config defaults
	cfg.Region = userCfg.LastRegion
	cfg.Profile = userCfg.LastProfile
	cfg.LastResource = userCfg.LastResource

	// Environment variables (medium priority, overrides user config)
	if envRegion := os.Getenv("AWS_REGION"); envRegion != "" {
		cfg.Region = envRegion
	}
	if cfg.Region == "" {
		if envRegion := os.Getenv("AWS_DEFAULT_REGION"); envRegion != "" {
			cfg.Region = envRegion
		}
	}
	if envProfile := os.Getenv("AWS_PROFILE"); envProfile != "" {
		cfg.Profile = envProfile
	}

	// Command-line flags (highest priority)
	flag.StringVar(&cfg.Region, "region", cfg.Region, "AWS region")
	flag.StringVar(&cfg.Region, "r", cfg.Region, "AWS region (shorthand)")
	flag.StringVar(&cfg.Profile, "profile", cfg.Profile, "AWS profile name (supports SSO profiles)")
	flag.StringVar(&cfg.Profile, "p", cfg.Profile, "AWS profile name (shorthand)")

	flag.Parse()

	return cfg, nil
}

// SaveUserState saves the current state to user config
func (c *Config) SaveUserState() error {
	if c.UserConfig == nil {
		c.UserConfig = &UserConfig{}
	}
	c.UserConfig.LastProfile = c.Profile
	c.UserConfig.LastRegion = c.Region
	c.UserConfig.LastResource = c.LastResource
	return c.UserConfig.Save()
}
