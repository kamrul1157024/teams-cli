package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kamrul1157024/teams-cli/teams-cli/api"
	"github.com/kamrul1157024/teams-cli/teams-cli/auth"
	"github.com/kamrul1157024/teams-cli/teams-cli/output"
	"github.com/spf13/cobra"
)

var (
	msgLimit       int
	msgFrom        string
	msgSince       string
	msgMine        bool
	msgPlain       bool
	msgBefore      string
	msgAfter       string
	sendTo         string
	sendHTML       bool
	sendFormat     string
	sendMention    []string
	sendGroup      string
	sendReplyTo    string
	searchChat     string
	searchLimit    int
	listTo         string
	exportFile     string
	deleteConfirm  bool
	notifLimit     int
	notifType      string
	notifSince     string
)

var messagesCmd = &cobra.Command{
	Use:   "messages",
	Short: "Read, send, and search messages",
}

var messagesListCmd = &cobra.Command{
	Use:   "list [chat-id]",
	Short: "List messages in a conversation",
	Long: `List messages in a conversation by chat ID or by email (--to).

Examples:
  teams-cli messages list 19:abc@thread.v2
  teams-cli messages list --to john@company.com
  teams-cli messages list 19:abc@thread.v2 --mine --since 2024-01-01`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()

		var chatID string
		if listTo != "" {
			id, err := client.FindChatByEmail(listTo)
			if err != nil {
				return fmt.Errorf("failed to resolve chat: %w", err)
			}
			chatID = id
		} else if len(args) > 0 {
			chatID = args[0]
		} else {
			return fmt.Errorf("chat-id is required (or use --to <email>)")
		}

		messages, err := client.GetMessagesWithOptions(chatID, msgLimit, api.GetMessagesOptions{
			Before: msgBefore,
			After:  msgAfter,
		})
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

		// Filter to own messages only
		if msgMine {
			email, err := auth.GetEmail()
			if err == nil {
				me, merr := client.GetMe()
				var filtered []api.MessageListItem
				for _, m := range messages {
					fromLower := strings.ToLower(m.From)
					if strings.Contains(fromLower, strings.ToLower(email)) {
						filtered = append(filtered, m)
					} else if merr == nil && me.DisplayName != "" && strings.Contains(fromLower, strings.ToLower(me.DisplayName)) {
						filtered = append(filtered, m)
					}
				}
				messages = filtered
			}
		}

		// Filter by --since date
		if msgSince != "" {
			var sinceTime time.Time
			for _, layout := range []string{"2006-01-02", "2006-01-02T15:04:05Z", time.RFC3339} {
				if t, err := time.Parse(layout, msgSince); err == nil {
					sinceTime = t
					break
				}
			}
			if !sinceTime.IsZero() {
				var filtered []api.MessageListItem
				for _, m := range messages {
					if t, err := time.Parse(time.RFC3339Nano, m.Time); err == nil {
						if !t.Before(sinceTime) {
							filtered = append(filtered, m)
						}
					}
				}
				messages = filtered
			}
		}

		switch outputFormat {
		case "text":
			for _, m := range messages {
				t := formatMessageTime(m.Time)
				content := m.Content
				if msgPlain {
					content = stripPlainText(content)
				}
				fmt.Printf("[%s] %s: %s\n", t, m.From, content)
			}
		case "table":
			headers := []string{"TIME", "FROM", "MESSAGE"}
			var rows [][]string
			for _, m := range messages {
				content := m.Content
				if msgPlain {
					content = stripPlainText(content)
				}
				rows = append(rows, []string{
					formatMessageTime(m.Time),
					truncate(m.From, 20),
					truncate(content, 60),
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
	Long: `Send a message to a conversation by chat ID, email (--to), or group name (--group).

Message can be provided as an argument or piped via stdin.

Examples:
  teams-cli messages send 19:abc@thread.v2 "Hello"
  teams-cli messages send --to john@company.com "Hello"
  teams-cli messages send --group "Monad Standup" "Hello team"
  teams-cli messages send 19:abc@thread.v2 "**bold** text" --format markdown
  teams-cli messages send 19:abc@thread.v2 "Hey @alice check this" --mention alice=alice@co.com
  teams-cli messages send 19:abc@thread.v2 "reply text" --reply-to 1234567890
  echo "Build passed" | teams-cli messages send 19:abc@thread.v2`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()

		var chatID, message string

		if sendTo != "" {
			id, err := client.FindChatByEmail(sendTo)
			if err != nil {
				return fmt.Errorf("failed to find chat: %w", err)
			}
			chatID = id
			if len(args) > 0 {
				message = strings.Join(args, " ")
			}
		} else if sendGroup != "" {
			id, err := client.FindChatByName(sendGroup)
			if err != nil {
				return fmt.Errorf("failed to find group: %w", err)
			}
			chatID = id
			if len(args) > 0 {
				message = strings.Join(args, " ")
			}
		} else {
			if len(args) < 1 {
				return fmt.Errorf("chat-id is required (or use --to <email> / --group <name>)")
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

		// Convert markdown to HTML if requested
		if sendFormat == "markdown" {
			message = api.ConvertMarkdownToHTML(message)
		}

		// Process @mentions
		if len(sendMention) > 0 {
			for _, m := range sendMention {
				parts := strings.SplitN(m, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid mention format %q — use name=email", m)
				}
				mentionHTML, err := client.BuildMentionHTML(parts[1])
				if err != nil {
					return fmt.Errorf("failed to resolve mention %s: %w", parts[1], err)
				}
				message = strings.ReplaceAll(message, "@"+parts[0], mentionHTML)
			}
		}

		// Reply to specific message
		if sendReplyTo != "" {
			result, err := client.ReplyToMessage(chatID, sendReplyTo, message)
			if err != nil {
				return fmt.Errorf("failed to reply: %w", err)
			}
			result.Content = message
			switch outputFormat {
			case "text":
				fmt.Printf("Replied to %s in %s\n", sendReplyTo, chatID)
			default:
				output.JSON(result, prettyPrint)
			}
			return nil
		}

		result, err := client.SendMessage(chatID, message)
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}

		result.Content = message

		switch outputFormat {
		case "text":
			fmt.Printf("Message sent to %s: %s\n", chatID, message)
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
		client := newClient()
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

var messagesMineCmd = &cobra.Command{
	Use:   "mine",
	Short: "Fetch your own messages across recent chats",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()

		email, err := auth.GetEmail()
		if err != nil {
			return fmt.Errorf("failed to get user email: %w", err)
		}
		me, _ := client.GetMe()
		displayName := ""
		if me != nil {
			displayName = me.DisplayName
		}

		chats, err := client.ListChats("", false, 20)
		if err != nil {
			return fmt.Errorf("failed to list chats: %w", err)
		}

		var allMessages []api.MessageListItem
		for _, chat := range chats {
			messages, err := client.GetMessages(chat.ID, msgLimit)
			if err != nil {
				continue
			}
			for _, m := range messages {
				fromLower := strings.ToLower(m.From)
				if strings.Contains(fromLower, strings.ToLower(email)) ||
					(displayName != "" && strings.Contains(fromLower, strings.ToLower(displayName))) {
					allMessages = append(allMessages, m)
				}
			}
		}

		switch outputFormat {
		case "text":
			for _, m := range allMessages {
				t := formatMessageTime(m.Time)
				fmt.Printf("[%s] %s\n", t, m.Content)
			}
		default:
			output.JSON(allMessages, prettyPrint)
		}
		return nil
	},
}

var messagesExportCmd = &cobra.Command{
	Use:   "export <chat-id>",
	Short: "Export chat history to a file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		messages, err := client.GetMessages(args[0], msgLimit)
		if err != nil {
			return fmt.Errorf("failed to get messages: %w", err)
		}

		if exportFile == "" {
			exportFile = "chat-export.json"
		}

		data, err := json.MarshalIndent(messages, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal messages: %w", err)
		}

		if err := os.WriteFile(exportFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write export file: %w", err)
		}

		fmt.Printf("Exported %d messages to %s\n", len(messages), exportFile)
		return nil
	},
}

var contactsCmd = &cobra.Command{
	Use:   "contacts",
	Short: "Manage contacts",
}

var contactsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all unique contacts from your chats",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		chats, err := client.ListChats("", false, 0)
		if err != nil {
			return fmt.Errorf("failed to list chats: %w", err)
		}

		seen := map[string]bool{}
		var contacts []map[string]string
		for _, chat := range chats {
			for _, member := range chat.Members {
				if seen[member] {
					continue
				}
				seen[member] = true
				contacts = append(contacts, map[string]string{
					"name":    member,
					"chat_id": chat.ID,
					"type":    chat.Type,
				})
			}
		}

		switch outputFormat {
		case "text":
			for _, c := range contacts {
				fmt.Println(c["name"])
			}
		case "table":
			headers := []string{"NAME", "CHAT TYPE", "CHAT ID"}
			var rows [][]string
			for _, c := range contacts {
				rows = append(rows, []string{c["name"], c["type"], truncate(c["chat_id"], 40)})
			}
			output.Table(headers, rows)
		default:
			output.JSON(contacts, prettyPrint)
		}
		return nil
	},
}

var messagesStatsCmd = &cobra.Command{
	Use:   "stats <chat-id>",
	Short: "Show message statistics for a chat",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		messages, err := client.GetMessages(args[0], msgLimit)
		if err != nil {
			return fmt.Errorf("failed to get messages: %w", err)
		}

		counts := map[string]int{}
		for _, m := range messages {
			counts[m.From]++
		}

		type stat struct {
			From  string `json:"from"`
			Count int    `json:"count"`
		}
		var stats []stat
		for from, count := range counts {
			stats = append(stats, stat{From: from, Count: count})
		}

		switch outputFormat {
		case "text":
			fmt.Printf("Total messages: %d\n", len(messages))
			for _, s := range stats {
				fmt.Printf("  %s: %d\n", s.From, s.Count)
			}
		case "table":
			headers := []string{"FROM", "COUNT"}
			var rows [][]string
			for _, s := range stats {
				rows = append(rows, []string{s.From, fmt.Sprintf("%d", s.Count)})
			}
			output.Table(headers, rows)
		default:
			output.JSON(map[string]interface{}{
				"total":    len(messages),
				"by_sender": stats,
			}, prettyPrint)
		}
		return nil
	},
}

var messagesReactCmd = &cobra.Command{
	Use:   "react <chat-id> <message-id> <emoji>",
	Short: "React to a message with an emoji",
	Long: `React to a message with an emoji reaction.

Valid reactions: like (👍), heart (❤️), laugh (😂), surprised (😮), sad (😢), angry (😡)

Examples:
  teams-cli messages react 19:abc@thread.v2 1234567890 like
  teams-cli messages react 19:abc@thread.v2 1234567890 heart
  teams-cli messages react 19:abc@thread.v2 1234567890 👍`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		err := client.ReactToMessage(args[0], args[1], args[2])
		if err != nil {
			return fmt.Errorf("failed to react: %w", err)
		}

		switch outputFormat {
		case "text":
			fmt.Printf("Reacted with %s\n", args[2])
		default:
			output.JSON(map[string]string{
				"status":     "reacted",
				"chat_id":    args[0],
				"message_id": args[1],
				"emoji":      args[2],
			}, prettyPrint)
		}
		return nil
	},
}

var messagesEditCmd = &cobra.Command{
	Use:   "edit <chat-id> <message-id> <new-text>",
	Short: "Edit a sent message",
	Long: `Edit a previously sent message.

Examples:
  teams-cli messages edit 19:abc@thread.v2 1234567890 "updated text"`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		err := client.EditMessage(args[0], args[1], args[2])
		if err != nil {
			return fmt.Errorf("failed to edit message: %w", err)
		}

		switch outputFormat {
		case "text":
			fmt.Printf("Message %s edited\n", args[1])
		default:
			output.JSON(map[string]string{
				"status":     "edited",
				"chat_id":    args[0],
				"message_id": args[1],
			}, prettyPrint)
		}
		return nil
	},
}

var messagesDeleteCmd = &cobra.Command{
	Use:   "delete <chat-id> <message-id>",
	Short: "Delete a sent message",
	Long: `Delete a previously sent message.

Examples:
  teams-cli messages delete 19:abc@thread.v2 1234567890
  teams-cli messages delete 19:abc@thread.v2 1234567890 --confirm`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !deleteConfirm {
			fmt.Printf("Delete message %s from %s? Use --confirm to proceed.\n", args[1], args[0])
			return nil
		}

		client := newClient()
		err := client.DeleteMessage(args[0], args[1])
		if err != nil {
			return fmt.Errorf("failed to delete message: %w", err)
		}

		switch outputFormat {
		case "text":
			fmt.Printf("Message %s deleted\n", args[1])
		default:
			output.JSON(map[string]string{
				"status":     "deleted",
				"chat_id":    args[0],
				"message_id": args[1],
			}, prettyPrint)
		}
		return nil
	},
}

var messagesReplyCmd = &cobra.Command{
	Use:   "reply <chat-id> <message-id> <text>",
	Short: "Reply to a specific message (threaded)",
	Long: `Reply to a specific message to create or continue a thread.

Examples:
  teams-cli messages reply 19:abc@thread.v2 1234567890 "I agree"
  teams-cli messages reply 19:abc@thread.v2 1234567890 "**bold reply**" --format markdown`,
	Args: cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		message := strings.Join(args[2:], " ")

		if sendFormat == "markdown" {
			message = api.ConvertMarkdownToHTML(message)
		}

		result, err := client.ReplyToMessage(args[0], args[1], message)
		if err != nil {
			return fmt.Errorf("failed to reply: %w", err)
		}
		result.Content = message

		switch outputFormat {
		case "text":
			fmt.Printf("Replied to %s in %s\n", args[1], args[0])
		default:
			output.JSON(result, prettyPrint)
		}
		return nil
	},
}

var notificationsCmd = &cobra.Command{
	Use:   "notifications",
	Short: "View activity notifications (mentions, replies, reactions)",
	Long: `View your Teams activity feed — mentions, replies, reactions, and follows.

Examples:
  teams-cli notifications --format json              # All notifications
  teams-cli notifications --type mentions             # Only @mentions
  teams-cli notifications --type replies              # Only replies to your messages
  teams-cli notifications --type reactions            # Only emoji reactions
  teams-cli notifications --since 2024-04-07          # Since a specific date
  teams-cli notifications -n 10 --format table        # Last 10, table format`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		items, err := client.GetNotifications(notifLimit, notifType, notifSince)
		if err != nil {
			return fmt.Errorf("failed to get notifications: %w", err)
		}

		switch outputFormat {
		case "text":
			for _, n := range items {
				t := formatMessageTime(n.Time)
				ctx := ""
				if n.Context != "" {
					ctx = " [" + n.Context + "]"
				}
				content := n.Content
				if content == "" {
					content = "(no content)"
				}
				fmt.Printf("[%s] %s%s: %s\n", t, n.Type, ctx, truncate(content, 80))
			}
		case "table":
			headers := []string{"TIME", "TYPE", "CONTEXT", "CONTENT"}
			var rows [][]string
			for _, n := range items {
				rows = append(rows, []string{
					formatMessageTime(n.Time),
					n.Type,
					truncate(n.Context, 25),
					truncate(n.Content, 50),
				})
			}
			output.Table(headers, rows)
		default:
			output.JSON(items, prettyPrint)
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

// stripPlainText removes extra whitespace/newlines from already-HTML-stripped text
func stripPlainText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	return s
}

func init() {
	messagesListCmd.Flags().IntVarP(&msgLimit, "limit", "n", 50, "Number of messages to fetch")
	messagesListCmd.Flags().StringVar(&msgFrom, "from", "", "Filter by sender name or email")
	messagesListCmd.Flags().StringVar(&msgSince, "since", "", "Filter messages since date (YYYY-MM-DD)")
	messagesListCmd.Flags().BoolVar(&msgMine, "mine", false, "Show only my messages")
	messagesListCmd.Flags().BoolVar(&msgPlain, "plain", false, "Strip HTML and clean up text output")
	messagesListCmd.Flags().StringVar(&listTo, "to", "", "Resolve chat by email instead of chat ID")
	messagesListCmd.Flags().StringVar(&msgBefore, "before", "", "Cursor: fetch messages before this time (RFC3339)")
	messagesListCmd.Flags().StringVar(&msgAfter, "after", "", "Cursor: fetch messages after this time (RFC3339)")

	messagesSendCmd.Flags().StringVar(&sendTo, "to", "", "Send to user by email (resolves to chat ID)")
	messagesSendCmd.Flags().StringVar(&sendGroup, "group", "", "Send to group chat by name")
	messagesSendCmd.Flags().BoolVar(&sendHTML, "html", false, "Send as raw HTML")
	messagesSendCmd.Flags().StringVar(&sendFormat, "msg-format", "", "Message format: markdown (converts to Teams HTML)")
	messagesSendCmd.Flags().StringArrayVar(&sendMention, "mention", nil, "Mention user: name=email (e.g., alice=alice@co.com)")
	messagesSendCmd.Flags().StringVar(&sendReplyTo, "reply-to", "", "Reply to a specific message ID (threaded reply)")

	messagesSearchCmd.Flags().StringVar(&searchChat, "chat", "", "Search within specific chat ID")
	messagesSearchCmd.Flags().IntVar(&searchLimit, "limit", 20, "Max results")

	messagesMineCmd.Flags().IntVarP(&msgLimit, "limit", "n", 50, "Number of messages per chat")

	messagesExportCmd.Flags().IntVarP(&msgLimit, "limit", "n", 200, "Number of messages to export")
	messagesExportCmd.Flags().StringVarP(&exportFile, "output", "o", "chat-export.json", "Output file path")

	messagesStatsCmd.Flags().IntVarP(&msgLimit, "limit", "n", 100, "Number of messages to analyze")

	messagesDeleteCmd.Flags().BoolVar(&deleteConfirm, "confirm", false, "Confirm deletion (required)")

	messagesReplyCmd.Flags().StringVar(&sendFormat, "msg-format", "", "Message format: markdown")

	messagesCmd.AddCommand(messagesListCmd)
	messagesCmd.AddCommand(messagesSendCmd)
	messagesCmd.AddCommand(messagesSearchCmd)
	messagesCmd.AddCommand(messagesMineCmd)
	messagesCmd.AddCommand(messagesExportCmd)
	messagesCmd.AddCommand(messagesStatsCmd)
	messagesCmd.AddCommand(messagesReactCmd)
	messagesCmd.AddCommand(messagesEditCmd)
	messagesCmd.AddCommand(messagesDeleteCmd)
	messagesCmd.AddCommand(messagesReplyCmd)
	rootCmd.AddCommand(messagesCmd)

	notificationsCmd.Flags().IntVarP(&notifLimit, "limit", "n", 30, "Number of notifications to fetch")
	notificationsCmd.Flags().StringVar(&notifType, "type", "", "Filter by type: mentions, replies, reactions")
	notificationsCmd.Flags().StringVar(&notifSince, "since", "", "Only notifications since date (YYYY-MM-DD)")
	rootCmd.AddCommand(notificationsCmd)

	contactsCmd.AddCommand(contactsListCmd)
	rootCmd.AddCommand(contactsCmd)
}
