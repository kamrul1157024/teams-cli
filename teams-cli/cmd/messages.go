package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kamrul1157024/teams-cli/teams-cli/api"
	"github.com/kamrul1157024/teams-cli/teams-cli/output"
	"github.com/spf13/cobra"
)

var (
	msgLimit    int
	msgFrom     string
	sendTo      string
	sendHTML    bool
	searchChat  string
	searchLimit int
)

var messagesCmd = &cobra.Command{
	Use:   "messages",
	Short: "Read, send, and search messages",
}

var messagesListCmd = &cobra.Command{
	Use:   "list <chat-id>",
	Short: "List messages in a conversation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.NewClient()
		messages, err := client.GetMessages(args[0], msgLimit)
		if err != nil {
			return fmt.Errorf("failed to get messages: %w", err)
		}

		// Filter by sender if specified
		if msgFrom != "" {
			var filtered []api.MessageListItem
			from := strings.ToLower(msgFrom)
			for _, m := range messages {
				if strings.Contains(strings.ToLower(m.From), from) {
					filtered = append(filtered, m)
				}
			}
			messages = filtered
		}

		switch outputFormat {
		case "text":
			for _, m := range messages {
				t := formatMessageTime(m.Time)
				fmt.Printf("[%s] %s: %s\n", t, m.From, m.Content)
			}
		case "table":
			headers := []string{"TIME", "FROM", "MESSAGE"}
			var rows [][]string
			for _, m := range messages {
				rows = append(rows, []string{
					formatMessageTime(m.Time),
					truncate(m.From, 20),
					truncate(m.Content, 60),
				})
			}
			output.Table(headers, rows)
		default:
			output.JSON(messages, prettyPrint)
		}
		return nil
	},
}

var messagesSendCmd = &cobra.Command{
	Use:   "send [chat-id] [message]",
	Short: "Send a message to a conversation",
	Long: `Send a message to a conversation by chat ID or by email (--to).

Message can be provided as an argument or piped via stdin.

Examples:
  teams-cli messages send 19:abc@thread.v2 "Hello"
  teams-cli messages send --to john@company.com "Hello"
  echo "Build passed" | teams-cli messages send 19:abc@thread.v2`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.NewClient()

		var chatID, message string

		if sendTo != "" {
			// Resolve email to chat ID
			id, err := client.FindChatByEmail(sendTo)
			if err != nil {
				return fmt.Errorf("failed to find chat: %w", err)
			}
			chatID = id
			if len(args) > 0 {
				message = strings.Join(args, " ")
			}
		} else {
			if len(args) < 1 {
				return fmt.Errorf("chat-id is required (or use --to <email>)")
			}
			chatID = args[0]
			if len(args) > 1 {
				message = strings.Join(args[1:], " ")
			}
		}

		// Read from stdin if no message argument
		if message == "" {
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				scanner := bufio.NewScanner(os.Stdin)
				var lines []string
				for scanner.Scan() {
					lines = append(lines, scanner.Text())
				}
				message = strings.Join(lines, "\n")
			}
		}

		if message == "" {
			return fmt.Errorf("no message provided")
		}

		result, err := client.SendMessage(chatID, message)
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}

		switch outputFormat {
		case "text":
			fmt.Printf("Message sent to %s\n", chatID)
		default:
			output.JSON(result, prettyPrint)
		}
		return nil
	},
}

var messagesSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search messages across conversations",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.NewClient()
		results, err := client.SearchMessages(args[0], searchChat, searchLimit)
		if err != nil {
			return fmt.Errorf("failed to search messages: %w", err)
		}

		switch outputFormat {
		case "text":
			for _, r := range results {
				t := formatMessageTime(r.Message.Time)
				fmt.Printf("[%s] [%s] %s: %s\n", t, truncate(r.ChatID, 20), r.Message.From, r.Message.Content)
			}
		case "table":
			headers := []string{"TIME", "CHAT", "FROM", "MESSAGE"}
			var rows [][]string
			for _, r := range results {
				rows = append(rows, []string{
					formatMessageTime(r.Message.Time),
					truncate(r.ChatID, 20),
					truncate(r.Message.From, 15),
					truncate(r.Message.Content, 50),
				})
			}
			output.Table(headers, rows)
		default:
			output.JSON(results, prettyPrint)
		}
		return nil
	},
}

func formatMessageTime(t string) string {
	parsed, err := time.Parse(time.RFC3339Nano, t)
	if err != nil {
		return t
	}
	return parsed.Format("2006-01-02 15:04")
}

func init() {
	messagesListCmd.Flags().IntVarP(&msgLimit, "limit", "n", 50, "Number of messages to fetch")
	messagesListCmd.Flags().StringVar(&msgFrom, "from", "", "Filter by sender name or email")

	messagesSendCmd.Flags().StringVar(&sendTo, "to", "", "Send to user by email (resolves to chat ID)")
	messagesSendCmd.Flags().BoolVar(&sendHTML, "html", false, "Send as raw HTML")

	messagesSearchCmd.Flags().StringVar(&searchChat, "chat", "", "Search within specific chat ID")
	messagesSearchCmd.Flags().IntVar(&searchLimit, "limit", 20, "Max results")

	messagesCmd.AddCommand(messagesListCmd)
	messagesCmd.AddCommand(messagesSendCmd)
	messagesCmd.AddCommand(messagesSearchCmd)
	rootCmd.AddCommand(messagesCmd)
}
