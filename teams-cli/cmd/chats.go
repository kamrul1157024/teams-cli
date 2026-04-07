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
	chatOffset     int
	chatCompact    bool
	chatWith       string
	chatActiveSince string
	createWith     string
)

var chatsCmd = &cobra.Command{
	Use:   "chats",
	Short: "Manage conversations",
}

var chatsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List conversations",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		chats, err := client.ListChatsWithOptions(api.ChatListOptions{
			FilterType:  chatFilterType,
			UnreadOnly:  chatUnread,
			Limit:       chatLimit,
			Offset:      chatOffset,
			Compact:     chatCompact,
			WithPerson:  chatWith,
			ActiveSince: chatActiveSince,
		})
		if err != nil {
			return fmt.Errorf("failed to list chats: %w", err)
		}

		switch outputFormat {
		case "text":
			for _, c := range chats {
				fmt.Println(c.ID)
			}
		case "table":
			headers := []string{"ID", "TYPE", "TITLE", "MEMBERS", "LAST MESSAGE", "READ"}
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
					fmt.Sprintf("%d", c.MemberCount),
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

var chatsResolveCmd = &cobra.Command{
	Use:   "resolve <email>",
	Short: "Resolve an email to a DM chat ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		chatID, err := client.FindChatByEmail(args[0])
		if err != nil {
			return fmt.Errorf("failed to resolve chat: %w", err)
		}

		switch outputFormat {
		case "text":
			fmt.Println(chatID)
		default:
			output.JSON(map[string]string{
				"email":   args[0],
				"chat_id": chatID,
			}, prettyPrint)
		}
		return nil
	},
}

var chatsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new chat",
	RunE: func(cmd *cobra.Command, args []string) error {
		if createWith == "" {
			return fmt.Errorf("--with <email> is required")
		}
		client := newClient()

		// Look up user to get MRI
		user, err := client.GetUser(createWith)
		if err != nil {
			return fmt.Errorf("failed to find user: %w", err)
		}
		if user.Mri == "" {
			return fmt.Errorf("user %s has no MRI", createWith)
		}

		chatID, err := client.CreateOneOnOneChat(user.Mri)
		if err != nil {
			return fmt.Errorf("failed to create chat: %w", err)
		}

		switch outputFormat {
		case "text":
			fmt.Printf("Created chat: %s\n", chatID)
		default:
			output.JSON(map[string]string{
				"chat_id": chatID,
				"with":    createWith,
				"status":  "created",
			}, prettyPrint)
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
	chatsListCmd.Flags().IntVar(&chatOffset, "offset", 0, "Skip first N results (pagination)")
	chatsListCmd.Flags().BoolVar(&chatCompact, "compact", false, "Compact output (omit members array)")
	chatsListCmd.Flags().StringVar(&chatWith, "with", "", "Find chats containing a person (name or email)")
	chatsListCmd.Flags().StringVar(&chatActiveSince, "active-since", "", "Only chats with activity since date (YYYY-MM-DD)")

	chatsCreateCmd.Flags().StringVar(&createWith, "with", "", "Email of user to create 1:1 chat with")

	chatsCmd.AddCommand(chatsListCmd)
	chatsCmd.AddCommand(chatsResolveCmd)
	chatsCmd.AddCommand(chatsCreateCmd)
	rootCmd.AddCommand(chatsCmd)
}
