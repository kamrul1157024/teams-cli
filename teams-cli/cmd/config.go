package cmd

import (
	"fmt"

	"github.com/kamrul1157024/teams-cli/teams-cli/api"
	"github.com/kamrul1157024/teams-cli/teams-cli/output"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := api.LoadConfig()
		switch outputFormat {
		case "text":
			fmt.Printf("signature: %s\n", cfg.Signature)
			fmt.Printf("signature_enabled: %v\n", cfg.SignatureEnabled)
		default:
			output.JSON(cfg, prettyPrint)
		}
		return nil
	},
}

var configSignatureCmd = &cobra.Command{
	Use:   "signature",
	Short: "Manage message signature",
}

var configSignatureOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Enable message signature",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := api.LoadConfig()
		cfg.SignatureEnabled = true
		if err := api.SaveConfig(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		fmt.Printf("Signature enabled: %q\n", cfg.Signature)
		return nil
	},
}

var configSignatureOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Disable message signature",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := api.LoadConfig()
		cfg.SignatureEnabled = false
		if err := api.SaveConfig(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		fmt.Println("Signature disabled")
		return nil
	},
}

var configSignatureSetCmd = &cobra.Command{
	Use:   "set <text>",
	Short: "Set custom signature text",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := api.LoadConfig()
		cfg.Signature = args[0]
		cfg.SignatureEnabled = true
		if err := api.SaveConfig(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		fmt.Printf("Signature set to: %q\n", cfg.Signature)
		return nil
	},
}

func init() {
	configSignatureCmd.AddCommand(configSignatureOnCmd)
	configSignatureCmd.AddCommand(configSignatureOffCmd)
	configSignatureCmd.AddCommand(configSignatureSetCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSignatureCmd)
	rootCmd.AddCommand(configCmd)
}
