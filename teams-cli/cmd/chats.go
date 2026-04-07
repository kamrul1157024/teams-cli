package cmd

import (
	"fmt"

	"github.com/kamrul1157024/teams-cli/teams-cli/api"
	"github.com/kamrul1157024/teams-cli/teams-cli/output"
	"github.com/spf13/cobra"
)

var (
	chatFilterType string
	chatUnread     bool
	chatLimit      int
)

var chatsCmd = &cobra.Command{
	Use:   "chats",
	Short: "Manage conversations",
}

var chatsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List conversations",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.NewClient()
		chats, err := client.ListChats(chatFilterType, chatUnread, chatLimit)
		if err != nil {
			return fmt.Errorf("failed to list chats: %w", err)
		}

		switch outputFormat {
		case "text":
			for _, c := range chats {
				fmt.Println(c.ID)
			}
		case "table":
			headers := []string{"ID", "TYPE", "TITLE", "LAST MESSAGE", "READ"}
			var rows [][]string
			for _, c := range chats {
				lastMsg := ""
				if c.LastMessage != nil {
					lastMsg = truncate(c.LastMessage.Content, 40)
				}
				read := "yes"
				if !c.IsRead {
					read = "no"
				}
				rows = append(rows, []string{
					truncate(c.ID, 30),
					c.Type,
					truncate(c.Title, 25),
					lastMsg,
					read,
				})
			}
			output.Table(headers, rows)
		default:
			output.JSON(chats, prettyPrint)
		}
		return nil
	},
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func init() {
	chatsListCmd.Flags().StringVar(&chatFilterType, "type", "", "Filter by chat type: 1:1, group")
	chatsListCmd.Flags().BoolVar(&chatUnread, "unread", false, "Only show unread conversations")
	chatsListCmd.Flags().IntVar(&chatLimit, "limit", 0, "Limit number of results")

	chatsCmd.AddCommand(chatsListCmd)
	rootCmd.AddCommand(chatsCmd)
}
