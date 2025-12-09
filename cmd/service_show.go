package cmd

import (
	"fmt"

	"github.com/juststeveking/scout/internal/config"
	"github.com/spf13/cobra"
)

var serviceShowCmd = &cobra.Command{
	Use:   "service:show <name>",
	Short: "Show details of a specific service",
	Long: `Display detailed configuration for a specific service.

Example:
  scout service:show api-prod`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		// Load existing config
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Find the service
		var found *config.Service
		for _, s := range cfg.Services {
			if s.Name == serviceName {
				found = &s
				break
			}
		}

		if found == nil {
			return fmt.Errorf("service '%s' not found", serviceName)
		}

		// Display service details
		fmt.Printf("Service: %s\n", found.Name)
		fmt.Println("─────────────────────────────────────")
		fmt.Printf("URL:              %s\n", found.URL)

		if found.HealthEndpoint != "" {
			fmt.Printf("Health Endpoint:  %s\n", found.HealthEndpoint)
		}

		if found.Type != "" {
			fmt.Printf("Type:             %s\n", found.Type)
		}

		if found.Method != "" {
			fmt.Printf("Method:           %s\n", found.Method)
		}

		if found.ExpectedStatus > 0 {
			fmt.Printf("Expected Status:  %d\n", found.ExpectedStatus)
		}

		if len(found.Headers) > 0 {
			fmt.Println("\nHeaders:")
			for key, value := range found.Headers {
				fmt.Printf("  %s: %s\n", key, value)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(serviceShowCmd)
}
