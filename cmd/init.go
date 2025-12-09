package cmd

import (
	"fmt"

	"github.com/juststeveking/scout/internal/config"
	"github.com/spf13/cobra"
)

var (
	forceInit bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize scout configuration",
	Long: `Create a new scout configuration file at ~/.config/scout/config.yml
with sensible defaults. Edit this file to add your services.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.InitConfig(forceInit); err != nil {
			return err
		}

		configPath, _ := config.GetConfigPath()

		if forceInit {
			fmt.Printf("✓ Configuration reset at %s\n", configPath)
		} else {
			fmt.Printf("✓ Configuration initialized at %s\n", configPath)
		}

		fmt.Println("\nEdit the config file to add your services, then run:")
		fmt.Println("  scout")

		return nil
	},
}

func init() {
	initCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "overwrite existing configuration")
	rootCmd.AddCommand(initCmd)
}
