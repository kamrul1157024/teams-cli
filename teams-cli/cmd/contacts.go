package cmd

import (
	"fmt"
	"strings"

	"github.com/kamrul1157024/teams-cli/teams-cli/api"
	"github.com/kamrul1157024/teams-cli/teams-cli/output"
	"github.com/spf13/cobra"
)

var (
	discoverDays    int
	discoverChats   int
	discoverMsgs    int
	discoverContact string
)

var contactsCmd = &cobra.Command{
	Use:   "contacts",
	Short: "Manage cached contact list",
}

var contactsListCmd = &cobra.Command{
	Use:   "list [search-query]",
	Short: "List or search contacts (sorted by most contacted)",
	Long: `List all known contacts sorted by most contacted, or search by name/email/nickname.

When searching, persona interaction data is included if available.
The contact list is cached for 24 hours. Use --refresh to force rebuild.

Examples:
  teams-cli contacts list                    # All contacts, most contacted first
  teams-cli contacts list "nabil"            # Search by name/email/nickname
  teams-cli contacts list --refresh          # Force fresh scan
  teams-cli contacts list --format table     # Table output`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()

		var query string
		if len(args) > 0 {
			query = args[0]
		}

		var contacts []api.Contact
		var err error

		if query != "" {
			contacts, err = client.SearchContacts(query)
		} else {
			contacts, err = client.GetContacts()
		}
		if err != nil {
			return fmt.Errorf("failed to get contacts: %w", err)
		}

		if len(contacts) == 0 {
			if query != "" {
				return fmt.Errorf("no contacts found matching %q", query)
			}
			return fmt.Errorf("no contacts found")
		}

		// Attach persona data when searching specific contacts
		if query != "" {
			for i := range contacts {
				if contacts[i].Email != "" {
					contacts[i].Persona = api.LoadPersona(contacts[i].Email)
				}
			}
		}

		switch outputFormat {
		case "text":
			for _, c := range contacts {
				if c.Email != "" {
					fmt.Printf("%s <%s> [%d chats]\n", c.DisplayName, c.Email, c.ChatCount)
				} else {
					fmt.Printf("%s [%d chats]\n", c.DisplayName, c.ChatCount)
				}
				if c.Persona != "" {
					fmt.Printf("  ---\n  %s\n", c.Persona)
				}
			}
		case "table":
			headers := []string{"NAME", "EMAIL", "CHATS", "NICKNAMES"}
			var rows [][]string
			for _, c := range contacts {
				nicks := ""
				if len(c.Nicknames) > 0 {
					nicks = joinStrings(c.Nicknames, ", ")
				}
				rows = append(rows, []string{c.DisplayName, c.Email, fmt.Sprintf("%d", c.ChatCount), nicks})
			}
			output.Table(headers, rows)
		default:
			output.JSON(contacts, prettyPrint)
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "\n%d contacts\n", len(contacts))
		return nil
	},
}

var contactsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Force refresh the contacts cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		client.CacheInvalidate("contacts")

		contacts, err := client.GetContacts()
		if err != nil {
			return fmt.Errorf("failed to sync contacts: %w", err)
		}

		fmt.Printf("Synced %d contacts\n", len(contacts))
		return nil
	},
}

var contactsDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover conversations grouped by contact for persona building",
	Long: `Fetch messages from active chats and group them by contact, then by chat type.

Output is grouped: contact -> conversations (dm, group, channel) -> messages.
Each message is tagged as "me" or the sender's name.
Includes message counts (total, my_messages, their_messages) per contact.

Examples:
  teams-cli contacts discover --format json                    # Last 30 days, top 50 chats
  teams-cli contacts discover --days 7 --format json           # Last week only
  teams-cli contacts discover --chats 100 --format json        # More chats
  teams-cli contacts discover --contact "nabil" --format json  # Single contact only`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()

		fmt.Fprintf(cmd.ErrOrStderr(), "Discovering conversations from last %d days...\n", discoverDays)

		results, err := client.DiscoverContacts(discoverDays, discoverChats, discoverMsgs)
		if err != nil {
			return fmt.Errorf("failed to discover contacts: %w", err)
		}

		// Filter to specific contact — resolve nicknames via contacts list
		if discoverContact != "" {
			query := strings.ToLower(discoverContact)
			// First resolve nickname to real names via contacts search
			matchNames := map[string]bool{}
			if contacts, err := client.SearchContacts(discoverContact); err == nil {
				for _, c := range contacts {
					matchNames[strings.ToLower(c.DisplayName)] = true
				}
			}
			var filtered []api.ContactDiscovery
			for _, r := range results {
				low := strings.ToLower(r.Contact)
				if strings.Contains(low, query) || matchNames[low] {
					filtered = append(filtered, r)
				}
			}
			results = filtered
		}

		if len(results) == 0 {
			return fmt.Errorf("no conversations found")
		}

		totalMsgs := 0
		for _, r := range results {
			totalMsgs += r.TotalMessages
		}

		switch outputFormat {
		case "text":
			for _, r := range results {
				fmt.Printf("\n=== %s (total:%d me:%d them:%d) ===\n", r.Contact, r.TotalMessages, r.MyMessages, r.TheirMessages)
				for _, conv := range r.Conversations {
					fmt.Printf("  [%s] %s\n", conv.Type, conv.ChatName)
					for _, msg := range conv.Messages {
						content := msg.Content
						if len(content) > 150 {
							content = content[:147] + "..."
						}
						fmt.Printf("    %s: %s\n", msg.From, content)
					}
				}
			}
		default:
			output.JSON(results, prettyPrint)
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "\n%d contacts, %d messages\n", len(results), totalMsgs)
		return nil
	},
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

func init() {
	contactsDiscoverCmd.Flags().IntVar(&discoverDays, "days", 30, "Look back N days")
	contactsDiscoverCmd.Flags().IntVar(&discoverChats, "chats", 50, "Max chats to scan")
	contactsDiscoverCmd.Flags().IntVar(&discoverMsgs, "msgs", 50, "Messages per chat")
	contactsDiscoverCmd.Flags().StringVar(&discoverContact, "contact", "", "Filter to a specific contact by name")

	contactsCmd.AddCommand(contactsListCmd)
	contactsCmd.AddCommand(contactsSyncCmd)
	contactsCmd.AddCommand(contactsDiscoverCmd)
	rootCmd.AddCommand(contactsCmd)
}
