package cmd

import (
	"fmt"

	"github.com/kamrul1157024/teams-cli/teams-cli/api"
	"github.com/kamrul1157024/teams-cli/teams-cli/output"
	"github.com/spf13/cobra"
)

var meCmd = &cobra.Command{
	Use:   "me",
	Short: "Show current user profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.NewClient()
		user, err := client.GetMe()
		if err != nil {
			return fmt.Errorf("failed to get user profile: %w", err)
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
				},
			)
		default:
			output.JSON(user, prettyPrint)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(meCmd)
}
