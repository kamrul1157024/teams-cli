package cmd

import (
	"fmt"
	"os"

	"github.com/kamrul1157024/teams-cli/teams-cli/auth"
	"github.com/kamrul1157024/teams-cli/teams-cli/output"
	"github.com/spf13/cobra"
)

var checkAPI bool

type StatusOutput struct {
	User     string                  `json:"user"`
	TenantID string                  `json:"tenant_id"`
	Tokens   map[string]*TokenStatus `json:"tokens"`
}

type TokenStatus struct {
	Valid     bool   `json:"valid"`
	ExpiresAt string `json:"expires_at,omitempty"`
	ExpiresIn string `json:"expires_in,omitempty"`
	Error     string `json:"error,omitempty"`
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check token health and authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		status := StatusOutput{
			Tokens: make(map[string]*TokenStatus),
		}

		tokenTypes := []auth.TokenType{auth.TokenTeams, auth.TokenSkype, auth.TokenChatSvcAgg}

		for _, t := range tokenTypes {
			info, err := auth.GetTokenInfo(t)
			ts := &TokenStatus{}
			if err != nil {
				ts.Error = err.Error()
			} else {
				ts.Valid = info.Valid
				if !info.ExpiresAt.IsZero() {
					ts.ExpiresAt = info.ExpiresAt.Format("2006-01-02T15:04:05Z")
					ts.ExpiresIn = info.ExpiresIn
				}
				if t == auth.TokenTeams {
					status.User = info.Email
					status.TenantID = info.TenantID
				}
			}
			status.Tokens[string(t)] = ts
		}

		switch outputFormat {
		case "text":
			fmt.Printf("User: %s\n", status.User)
			fmt.Printf("Tenant: %s\n", status.TenantID)
			for name, ts := range status.Tokens {
				if ts.Error != "" {
					fmt.Printf("  %s: ERROR - %s\n", name, ts.Error)
				} else if ts.Valid {
					fmt.Printf("  %s: valid (expires in %s)\n", name, ts.ExpiresIn)
				} else {
					fmt.Printf("  %s: EXPIRED\n", name)
				}
			}
		case "table":
			headers := []string{"TOKEN", "STATUS", "EXPIRES IN"}
			var rows [][]string
			for name, ts := range status.Tokens {
				s := "EXPIRED"
				exp := "-"
				if ts.Error != "" {
					s = "ERROR"
				} else if ts.Valid {
					s = "valid"
					exp = ts.ExpiresIn
				}
				rows = append(rows, []string{name, s, exp})
			}
			output.Table(headers, rows)
		default:
			output.JSON(status, prettyPrint)
		}

		// Exit with error if any token is invalid
		for _, ts := range status.Tokens {
			if !ts.Valid {
				fmt.Fprintln(os.Stderr, "Run 'teams-cli auth' to re-authenticate")
				os.Exit(1)
			}
		}

		return nil
	},
}

func init() {
	statusCmd.Flags().BoolVar(&checkAPI, "check", false, "Also verify tokens against API")
	rootCmd.AddCommand(statusCmd)
}
