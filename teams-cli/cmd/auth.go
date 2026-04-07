package cmd

import (
	"fmt"
	"os"

	"github.com/kamrul1157024/teams-cli/teams-cli/auth"
	"github.com/spf13/cobra"
)

var forceAuth bool

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with Microsoft Teams",
	Long:  "Opens a login window to authenticate with your Microsoft Teams account. Saves tokens to ~/.config/teams-cli/",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !forceAuth {
			// Check if tokens already exist and are valid
			info, err := auth.GetTokenInfo(auth.TokenTeams)
			if err == nil && info.Valid {
				fmt.Fprintf(os.Stderr, "Already authenticated as %s (tokens valid for %s)\n", info.Email, info.ExpiresIn)
				fmt.Fprintf(os.Stderr, "Use --force to re-authenticate\n")
				return nil
			}
		}

		fmt.Fprintln(os.Stderr, "Opening login window...")
		result, err := auth.RunOAuth()
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		dir, _ := auth.ConfigDir()
		fmt.Fprintf(os.Stderr, "Authenticated as %s\n", result.Email)
		fmt.Fprintf(os.Stderr, "Tenant: %s\n", result.TenantID)
		fmt.Fprintf(os.Stderr, "Tokens saved to %s\n", dir)
		return nil
	},
}

func init() {
	authCmd.Flags().BoolVar(&forceAuth, "force", false, "Re-authenticate even if tokens are valid")
	rootCmd.AddCommand(authCmd)
}
