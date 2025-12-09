package cmd

import (
	"fmt"

	"github.com/juststeveking/scout/internal/config"
	"github.com/spf13/cobra"
)

var (
	serviceName           string
	serviceURL            string
	serviceHealthEndpoint string
	serviceMethod         string
	serviceExpectedStatus int
	serviceType           string
	serviceHeaders        map[string]string
)

var serviceAddCmd = &cobra.Command{
	Use:   "service:add",
	Short: "Add a new service to monitor",
	Long: `Add a new service to your scout configuration.

Examples:
  scout service:add --name api-prod --url https://api.example.com --health-endpoint /health
  scout service:add --name redis --url redis://localhost:6379 --type redis
  scout service:add --name db --url db.example.com:5432 --type tcp`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate required fields
		if serviceName == "" {
			return fmt.Errorf("service name is required (--name)")
		}
		if serviceURL == "" {
			return fmt.Errorf("service URL is required (--url)")
		}

		// Load existing config
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Create new service
		service := config.Service{
			Name:           serviceName,
			URL:            serviceURL,
			HealthEndpoint: serviceHealthEndpoint,
			Method:         serviceMethod,
			ExpectedStatus: serviceExpectedStatus,
			Type:           serviceType,
			Headers:        serviceHeaders,
		}

		// Add service to config
		if err := cfg.AddService(service); err != nil {
			return err
		}

		// Save config
		if err := config.SaveConfig(cfg); err != nil {
			return err
		}

		configPath, _ := config.GetConfigPath()
		fmt.Printf("✓ Added service '%s' to %s\n", serviceName, configPath)

		return nil
	},
}

func init() {
	serviceAddCmd.Flags().StringVarP(&serviceName, "name", "n", "", "service name (required)")
	serviceAddCmd.Flags().StringVarP(&serviceURL, "url", "u", "", "service URL (required)")
	serviceAddCmd.Flags().StringVar(&serviceHealthEndpoint, "health-endpoint", "", "health check endpoint path")
	serviceAddCmd.Flags().StringVar(&serviceMethod, "method", "GET", "HTTP method for health check")
	serviceAddCmd.Flags().IntVar(&serviceExpectedStatus, "expected-status", 200, "expected HTTP status code")
	serviceAddCmd.Flags().StringVar(&serviceType, "type", "", "service type (http, tcp, redis)")
	serviceAddCmd.Flags().StringToStringVar(&serviceHeaders, "headers", nil, "HTTP headers (key=value)")

	serviceAddCmd.MarkFlagRequired("name")
	serviceAddCmd.MarkFlagRequired("url")

	rootCmd.AddCommand(serviceAddCmd)
}
