package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/juststeveking/scout/internal/config"
	"github.com/spf13/cobra"
)

var (
	forceRemove bool
)

var serviceRemoveCmd = &cobra.Command{
	Use:   "service:remove <name>",
	Short: "Remove a service from configuration",
	Long: `Remove a service by name from your scout configuration.

Example:
  scout service:remove api-prod
  scout service:remove redis --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		// Load existing config
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Confirm removal unless --force is used
		if !forceRemove {
			fmt.Printf("Remove service '%s'? (y/N): ", serviceName)
			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				return err
			}

			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		// Remove service
		if err := cfg.RemoveService(serviceName); err != nil {
			return err
		}

		// Save config
		if err := config.SaveConfig(cfg); err != nil {
			return err
		}

		configPath, _ := config.GetConfigPath()
		fmt.Printf("✓ Removed service '%s' from %s\n", serviceName, configPath)

		return nil
	},
}

func init() {
	serviceRemoveCmd.Flags().BoolVarP(&forceRemove, "force", "f", false, "skip confirmation prompt")
	rootCmd.AddCommand(serviceRemoveCmd)
}
