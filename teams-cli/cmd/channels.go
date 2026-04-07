package cmd

import (
	"fmt"

	"github.com/kamrul1157024/teams-cli/teams-cli/api"
	"github.com/kamrul1157024/teams-cli/teams-cli/output"
	"github.com/spf13/cobra"
)

var channelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "Manage channels",
}

var channelsListCmd = &cobra.Command{
	Use:   "list <team-id>",
	Short: "List channels in a team",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.NewClient()
		channels, err := client.ListChannels(args[0])
		if err != nil {
			return fmt.Errorf("failed to list channels: %w", err)
		}

		switch outputFormat {
		case "text":
			for _, ch := range channels {
				fmt.Println(ch.ID)
			}
		case "table":
			headers := []string{"ID", "NAME", "GENERAL", "FAVORITE", "PINNED", "TEAM"}
			var rows [][]string
			for _, ch := range channels {
				gen := ""
				if ch.IsGeneral {
					gen = "yes"
				}
				fav := ""
				if ch.IsFavorite {
					fav = "yes"
				}
				pin := ""
				if ch.IsPinned {
					pin = "yes"
				}
				rows = append(rows, []string{
					truncate(ch.ID, 30),
					ch.Name,
					gen,
					fav,
					pin,
					ch.Team,
				})
			}
			output.Table(headers, rows)
		default:
			output.JSON(channels, prettyPrint)
		}
		return nil
	},
}

func init() {
	channelsCmd.AddCommand(channelsListCmd)
	rootCmd.AddCommand(channelsCmd)
}
