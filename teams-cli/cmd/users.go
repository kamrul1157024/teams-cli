package cmd

import (
	"fmt"
	"strings"

	"github.com/kamrul1157024/teams-cli/teams-cli/output"
	"github.com/spf13/cobra"
)

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Look up users",
}

var usersSearchCmd = &cobra.Command{
	Use:   "search <email-or-name>",
	Short: "Look up a user by email or display name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		users, err := client.SearchUsers(args[0])
		if err != nil {
			return fmt.Errorf("failed to find user: %w", err)
		}
		if len(users) == 0 {
			return fmt.Errorf("no users found matching %q", args[0])
		}

		switch outputFormat {
		case "text":
			for _, user := range users {
				fmt.Printf("%s (%s)\n", user.DisplayName, user.Email)
				if user.JobTitle != "" {
					fmt.Printf("  Title: %s\n", user.JobTitle)
				}
				if user.Department != "" {
					fmt.Printf("  Department: %s\n", user.Department)
				}
			}
		case "table":
			headers := []string{"NAME", "EMAIL", "TITLE", "DEPARTMENT", "MRI"}
			var rows [][]string
			for _, user := range users {
				rows = append(rows, []string{
					user.DisplayName,
					user.Email,
					user.JobTitle,
					user.Department,
					user.Mri,
				})
			}
			output.Table(headers, rows)
		default:
			if len(users) == 1 {
				output.JSON(users[0], prettyPrint)
			} else {
				output.JSON(users, prettyPrint)
			}
		}
		return nil
	},
}

var usersResolveCmd = &cobra.Command{
	Use:   "resolve <mri1,mri2,...>",
	Short: "Resolve MRI strings to user names (comma-separated)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		mris := strings.Split(args[0], ",")
		for i := range mris {
			mris[i] = strings.TrimSpace(mris[i])
		}

		users, err := client.ResolveMRIs(mris)
		if err != nil {
			return fmt.Errorf("failed to resolve MRIs: %w", err)
		}

		switch outputFormat {
		case "text":
			for _, u := range users {
				fmt.Printf("%s → %s\n", u.Mri, u.DisplayName)
			}
		case "table":
			headers := []string{"MRI", "NAME"}
			var rows [][]string
			for _, u := range users {
				rows = append(rows, []string{u.Mri, u.DisplayName})
			}
			output.Table(headers, rows)
		default:
			output.JSON(users, prettyPrint)
		}
		return nil
	},
}

func init() {
	usersCmd.AddCommand(usersSearchCmd)
	usersCmd.AddCommand(usersResolveCmd)
	rootCmd.AddCommand(usersCmd)
}
