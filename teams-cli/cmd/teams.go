package cmd

import (
	"fmt"

	"github.com/kamrul1157024/teams-cli/teams-cli/output"
	"github.com/spf13/cobra"
)

var teamsCmd = &cobra.Command{
	Use:   "teams",
	Short: "Manage teams",
}

var teamsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List joined teams",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		teams, err := client.ListTeams()
		if err != nil {
			return fmt.Errorf("failed to list teams: %w", err)
		}

		switch outputFormat {
		case "text":
			for _, t := range teams {
				fmt.Println(t.ID)
			}
		case "table":
			headers := []string{"ID", "NAME", "FAVORITE", "ARCHIVED", "CHANNELS", "MEMBERS"}
			var rows [][]string
			for _, t := range teams {
				fav := ""
				if t.IsFavorite {
					fav = "yes"
				}
				arch := ""
				if t.IsArchived {
					arch = "yes"
				}
				rows = append(rows, []string{
					truncate(t.ID, 30),
					t.Name,
					fav,
					arch,
					fmt.Sprintf("%d", t.ChannelCount),
					fmt.Sprintf("%d", t.MemberCount),
				})
			}
			output.Table(headers, rows)
		default:
			output.JSON(teams, prettyPrint)
		}
		return nil
	},
}

func init() {
	teamsCmd.AddCommand(teamsListCmd)
	rootCmd.AddCommand(teamsCmd)
}
