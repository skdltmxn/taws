package config

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/skdltmxn/taws/internal/domain"
)

// ListProfiles reads AWS config and credentials files to list available profiles
func ListProfiles() ([]domain.AWSProfile, error) {
	profiles := make(map[string]*domain.AWSProfile)

	// Read ~/.aws/config
	configPath := filepath.Join(getAWSDir(), "config")
	if err := parseConfigFile(configPath, profiles, true); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Read ~/.aws/credentials
	credentialsPath := filepath.Join(getAWSDir(), "credentials")
	if err := parseConfigFile(credentialsPath, profiles, false); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Convert map to slice
	result := make([]domain.AWSProfile, 0, len(profiles))
	for _, p := range profiles {
		result = append(result, *p)
	}

	// Sort: default first, then alphabetically
	sortProfiles(result)

	return result, nil
}

func getAWSDir() string {
	if dir := os.Getenv("AWS_CONFIG_FILE"); dir != "" {
		return filepath.Dir(dir)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".aws")
}

func parseConfigFile(path string, profiles map[string]*domain.AWSProfile, isConfig bool) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Regex to match profile headers
	// In config file: [profile name] or [default]
	// In credentials file: [name] or [default]
	profileRegex := regexp.MustCompile(`^\[(profile\s+)?([^\]]+)\]$`)

	scanner := bufio.NewScanner(file)
	var currentProfile string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Check for profile header
		if matches := profileRegex.FindStringSubmatch(line); matches != nil {
			profileName := matches[2]
			currentProfile = profileName

			if _, exists := profiles[profileName]; !exists {
				profiles[profileName] = &domain.AWSProfile{
					Name:      profileName,
					IsDefault: profileName == "default",
				}
			}
			continue
		}

		// Parse key-value pairs
		if currentProfile != "" && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			profile := profiles[currentProfile]
			switch key {
			case "region":
				profile.Region = value
			case "sso_start_url":
				profile.SSOStart = value
				profile.IsSSO = true
			}
		}
	}

	return scanner.Err()
}

func sortProfiles(profiles []domain.AWSProfile) {
	// Simple bubble sort for small lists
	n := len(profiles)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			swap := false
			if profiles[j+1].IsDefault {
				swap = true
			} else if !profiles[j].IsDefault && profiles[j].Name > profiles[j+1].Name {
				swap = true
			}
			if swap {
				profiles[j], profiles[j+1] = profiles[j+1], profiles[j]
			}
		}
	}
}
