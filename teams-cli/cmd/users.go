package cmd

import (
	"fmt"

	"github.com/kamrul1157024/teams-cli/teams-cli/api"
	"github.com/kamrul1157024/teams-cli/teams-cli/output"
	"github.com/spf13/cobra"
)

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Look up users",
}

var usersSearchCmd = &cobra.Command{
	Use:   "search <email>",
	Short: "Look up a user by email",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.NewClient()
		user, err := client.GetUser(args[0])
		if err != nil {
			return fmt.Errorf("failed to find user: %w", err)
		}

		switch outputFormat {
		case "text":
			fmt.Printf("%s (%s)\n", user.DisplayName, user.Email)
			if user.JobTitle != "" {
				fmt.Printf("Title: %s\n", user.JobTitle)
			}
			if user.Department != "" {
				fmt.Printf("Department: %s\n", user.Department)
			}
		case "table":
			output.Table(
				[]string{"FIELD", "VALUE"},
				[][]string{
					{"Name", user.DisplayName},
					{"Email", user.Email},
					{"Title", user.JobTitle},
					{"Department", user.Department},
					{"MRI", user.Mri},
					{"Phone", user.Phone},
				},
			)
		default:
			output.JSON(user, prettyPrint)
		}
		return nil
	},
}

func init() {
	usersCmd.AddCommand(usersSearchCmd)
	rootCmd.AddCommand(usersCmd)
}
