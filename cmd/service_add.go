package cmd

import (
	"fmt"
	"strconv"

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
	authType              string
	authToken             string
	authUsername          string
	authPassword          string
	jsonAssertions        []string // Format: "path=value=operator" (e.g., "status=ok===")
)

var serviceAddCmd = &cobra.Command{
	Use:   "service:add",
	Short: "Add a new service to monitor",
	Long: `Add a new service to your scout configuration.

Examples:
  # Basic HTTP health check
  scout service:add --name api-prod --url https://api.example.com --health-endpoint /health
  
  # With custom headers
  scout service:add --name api --url https://api.example.com --headers X-API-Key:secret --headers User-Agent:Scout
  
  # With Bearer token auth
  scout service:add --name api --url https://api.example.com --auth-type bearer --auth-token mytoken123
  
  # With JSON assertions
  scout service:add --name api --url https://api.example.com --json-assertion status=ok===  --json-assertion uptime=0=>
  
  # TCP port check
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

		// Parse JSON assertions
		var assertions []config.JSONAssertion
		for _, assertion := range jsonAssertions {
			// Parse format: "path=value=operator"
			parts := splitAssertionString(assertion)
			if len(parts) >= 3 {
				jsonAssert := config.JSONAssertion{
					Path:     parts[0],
					Value:    parseJSONValue(parts[1]),
					Operator: parts[2],
				}
				assertions = append(assertions, jsonAssert)
			}
		}

		// Create auth if specified
		var auth *config.Auth
		if authType != "" {
			auth = &config.Auth{
				Type:     authType,
				Token:    authToken,
				Username: authUsername,
				Password: authPassword,
			}
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
			Auth:           auth,
			JSONAssertions: assertions,
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
	serviceAddCmd.Flags().StringVar(&serviceType, "type", "", "service type (http, tcp)")
	serviceAddCmd.Flags().StringToStringVar(&serviceHeaders, "headers", nil, "HTTP headers (key=value)")
	serviceAddCmd.Flags().StringVar(&authType, "auth-type", "", "authentication type (bearer, basic)")
	serviceAddCmd.Flags().StringVar(&authToken, "auth-token", "", "bearer token for authentication")
	serviceAddCmd.Flags().StringVar(&authUsername, "auth-username", "", "username for basic authentication")
	serviceAddCmd.Flags().StringVar(&authPassword, "auth-password", "", "password for basic authentication")
	serviceAddCmd.Flags().StringSliceVar(&jsonAssertions, "json-assertion", nil, "JSON path assertion (format: path=value=operator, e.g., status=ok===)")

	serviceAddCmd.MarkFlagRequired("name")
	serviceAddCmd.MarkFlagRequired("url")

	rootCmd.AddCommand(serviceAddCmd)
}

// Helper functions
func splitAssertionString(s string) []string {
	parts := make([]string, 0)
	current := ""
	for i := 0; i < len(s); i++ {
		if s[i] == '=' && i+1 < len(s) && s[i+1] != '=' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else if s[i] == '=' && i+1 < len(s) && s[i+1] == '=' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
			parts = append(parts, "==")
			i++ // Skip next =
		} else {
			current += string(s[i])
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// parseJSONValue attempts to parse a string into a JSON-compatible value
func parseJSONValue(s string) interface{} {
	// Try to parse as boolean
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	// Try to parse as number
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	// Return as string
	return s
}
