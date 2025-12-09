package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/juststeveking/scout/internal/config"
	"github.com/juststeveking/scout/internal/monitor"
	"github.com/juststeveking/scout/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "scout",
	Short: "Monitor the health of your services from the terminal",
	Long: `Scout is a terminal-based dashboard for monitoring multiple services and APIs.
Configure your endpoints, authentication, and health checks in a single config file,
then launch Scout to see real-time status across all your services.

Perfect for keeping an eye on staging environments, microservices, or any APIs
you depend on during development.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Println("Config not found, creating default config...")
				if initErr := config.InitConfig(false); initErr != nil {
					return fmt.Errorf("failed to create default config: %w", initErr)
				}
				// Try loading again
				cfg, err = config.LoadConfig()
				if err != nil {
					return fmt.Errorf("failed to load config after creation: %w", err)
				}
			} else {
				return fmt.Errorf("failed to load config: %w (run 'scout init' to create one)", err)
			}
		}

		if len(cfg.Services) == 0 {
			return fmt.Errorf("no services configured (run 'scout service:add' to add one)")
		}

		// Create monitor
		mon, err := monitor.NewMonitor(cfg)
		if err != nil {
			return fmt.Errorf("failed to create monitor: %w", err)
		}

		// Setup context with cancellation
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle OS signals
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			cancel()
		}()

		// Start monitoring in background
		go mon.Start(ctx)

		// Start TUI
		model := tui.NewModel(mon, cancel)
		p := tea.NewProgram(model, tea.WithAltScreen())

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("failed to start TUI: %w", err)
		}

		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
