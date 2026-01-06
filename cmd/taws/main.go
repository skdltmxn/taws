package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/skdltmxn/taws/internal/app"
	"github.com/skdltmxn/taws/internal/buildinfo"
	"github.com/skdltmxn/taws/internal/config"
	"github.com/skdltmxn/taws/internal/ui"
	"github.com/skdltmxn/taws/internal/ui/common"
)

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			fmt.Println(buildinfo.FullString())
			return
		}
	}

	ctx := context.Background()

	application := app.New()
	if err := application.Init(ctx); err != nil {
		// We don't exit here, because we want to show the UI even if auth fails
		// The UI will handle the error state
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize app: %v\n", err)
	}

	// Load and apply theme
	themeName := ""
	if application.Config != nil && application.Config.UserConfig != nil {
		themeName = application.Config.UserConfig.Theme
	}
	theme, _ := config.LoadTheme(themeName)
	common.ApplyTheme(theme)

	p := tea.NewProgram(ui.NewModel(application), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
