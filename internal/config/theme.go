package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Theme defines the color scheme for the application
type Theme struct {
	Name   string      `json:"name"`
	Colors ThemeColors `json:"colors"`
}

// ThemeColors contains all customizable colors
type ThemeColors struct {
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
	Error     string `json:"error"`
	Success   string `json:"success"`
	Warning   string `json:"warning"`
	Info      string `json:"info"`
	Text      string `json:"text"`
	SubText   string `json:"sub_text"`
	Border    string `json:"border"`
	HeaderBg  string `json:"header_bg"`
}

// DefaultTheme returns the built-in default theme
func DefaultTheme() *Theme {
	return &Theme{
		Name: "default",
		Colors: ThemeColors{
			Primary:   "#7D56F4",
			Secondary: "#6244C5",
			Error:     "#FF0000",
			Success:   "#3FB950",
			Warning:   "#F4BD2D",
			Info:      "#3794FF",
			Text:      "#FFFFFF",
			SubText:   "#888888",
			Border:    "#888888",
			HeaderBg:  "#1a1a2e",
		},
	}
}

// getThemesDir returns the themes directory path
func getThemesDir() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "themes"), nil
}

// LoadTheme loads a theme by name from the themes directory
func LoadTheme(name string) (*Theme, error) {
	if name == "" || name == "default" {
		return DefaultTheme(), nil
	}

	themesDir, err := getThemesDir()
	if err != nil {
		return DefaultTheme(), nil
	}

	themePath := filepath.Join(themesDir, name+".json")
	data, err := os.ReadFile(themePath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultTheme(), nil
		}
		return nil, err
	}

	var theme Theme
	if err := json.Unmarshal(data, &theme); err != nil {
		return DefaultTheme(), nil
	}

	// Fill in missing colors with defaults
	defaults := DefaultTheme()
	if theme.Colors.Primary == "" {
		theme.Colors.Primary = defaults.Colors.Primary
	}
	if theme.Colors.Secondary == "" {
		theme.Colors.Secondary = defaults.Colors.Secondary
	}
	if theme.Colors.Error == "" {
		theme.Colors.Error = defaults.Colors.Error
	}
	if theme.Colors.Success == "" {
		theme.Colors.Success = defaults.Colors.Success
	}
	if theme.Colors.Warning == "" {
		theme.Colors.Warning = defaults.Colors.Warning
	}
	if theme.Colors.Info == "" {
		theme.Colors.Info = defaults.Colors.Info
	}
	if theme.Colors.Text == "" {
		theme.Colors.Text = defaults.Colors.Text
	}
	if theme.Colors.SubText == "" {
		theme.Colors.SubText = defaults.Colors.SubText
	}
	if theme.Colors.Border == "" {
		theme.Colors.Border = defaults.Colors.Border
	}
	if theme.Colors.HeaderBg == "" {
		theme.Colors.HeaderBg = defaults.Colors.HeaderBg
	}

	return &theme, nil
}

// SaveTheme saves a theme to the themes directory
func SaveTheme(theme *Theme) error {
	themesDir, err := getThemesDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(themesDir, 0755); err != nil {
		return err
	}

	themePath := filepath.Join(themesDir, theme.Name+".json")
	data, err := json.MarshalIndent(theme, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(themePath, data, 0644)
}

// ListThemes returns available theme names
func ListThemes() ([]string, error) {
	themes := []string{"default"}

	themesDir, err := getThemesDir()
	if err != nil {
		return themes, nil
	}

	entries, err := os.ReadDir(themesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return themes, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) == ".json" {
			themeName := name[:len(name)-5]
			if themeName != "default" {
				themes = append(themes, themeName)
			}
		}
	}

	return themes, nil
}
