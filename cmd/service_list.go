package cmd

import (
	"fmt"

	"github.com/juststeveking/scout/internal/config"
	"github.com/spf13/cobra"
)

var serviceListCmd = &cobra.Command{
	Use:   "service:list",
	Short: "List all configured services",
	Long:  `Display all services currently configured in scout.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if len(cfg.Services) == 0 {
			fmt.Println("No services configured yet.")
			fmt.Println("\nAdd a service with:")
			fmt.Println("  scout service:add --name <name> --url <url>")
			return nil
		}

		fmt.Printf("Configured services (%d):\n\n", len(cfg.Services))

		for _, service := range cfg.Services {
			fmt.Printf("  • %s\n", service.Name)
			fmt.Printf("    URL: %s", service.URL)

			if service.HealthEndpoint != "" {
				fmt.Printf("%s", service.HealthEndpoint)
			}
			fmt.Println()

			if service.Type != "" {
				fmt.Printf("    Type: %s\n", service.Type)
			}

			if service.Method != "" {
				fmt.Printf("    Method: %s (expects %d)\n", service.Method, service.ExpectedStatus)
			}

			fmt.Println()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(serviceListCmd)
}
