package api

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const TTLContacts = 24 * time.Hour

type Contact struct {
	DisplayName string   `json:"display_name"`
	Email       string   `json:"email,omitempty"`
	Mri         string   `json:"mri,omitempty"`
	Nicknames   []string `json:"nicknames,omitempty"`
	ChatCount   int      `json:"chat_count"`
	Persona     string   `json:"persona,omitempty"`
}

func personaDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".teams-agent", "contacts")
}

// GetContacts returns contacts sorted by most contacted (shared chat count).
func (c *Client) GetContacts() ([]Contact, error) {
	key := cacheKey("contacts")
	var cached []Contact
	if c.cacheGet(key, &cached) {
		return cached, nil
	}

	chats, err := c.ListChatsWithOptions(ChatListOptions{Limit: 0, IncludeBots: false})
	if err != nil {
		return nil, err
	}

	type info struct {
		count int
		mri   string
	}
	memberInfo := map[string]*info{}

	for _, chat := range chats {
		for _, member := range chat.Members {
			name := strings.ToLower(member)
			if name == "" {
				continue
			}
			if _, ok := memberInfo[name]; !ok {
				memberInfo[name] = &info{}
			}
			memberInfo[name].count++
		}
	}

	// Get MRIs from raw conversation data
	convs, err := c.GetConversations()
	if err == nil {
		for _, chat := range convs.Chats {
			for _, m := range chat.Members {
				if m.FriendlyName != "" && !strings.HasPrefix(m.Mri, "28:") {
					key := strings.ToLower(m.FriendlyName)
					if inf, ok := memberInfo[key]; ok && inf.mri == "" {
						inf.mri = m.Mri
					}
				}
			}
		}
	}

	var contacts []Contact
	for name, inf := range memberInfo {
		// Recover original casing
		displayName := name
		for _, chat := range chats {
			for _, member := range chat.Members {
				if strings.ToLower(member) == name {
					displayName = member
					goto found
				}
			}
		}
	found:
		contacts = append(contacts, Contact{
			DisplayName: displayName,
			Mri:         inf.mri,
			ChatCount:   inf.count,
		})
	}

	sort.Slice(contacts, func(i, j int) bool {
		return contacts[i].ChatCount > contacts[j].ChatCount
	})

	loadPersonaMetadata(contacts)

	c.cacheSet(key, contacts, TTLContacts)
	return contacts, nil
}

// loadPersonaMetadata reads email and nicknames from persona files.
func loadPersonaMetadata(contacts []Contact) {
	dir := personaDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	type meta struct {
		email     string
		name      string
		nicknames []string
	}
	byName := map[string]meta{}

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		email := strings.TrimSuffix(e.Name(), ".md")
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		m := meta{email: email}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "# ") {
				m.name = strings.TrimPrefix(line, "# ")
			}
			if strings.HasPrefix(line, "nicknames:") {
				for _, n := range strings.Split(strings.TrimPrefix(line, "nicknames:"), ",") {
					n = strings.TrimSpace(n)
					if n != "" {
						m.nicknames = append(m.nicknames, n)
					}
				}
			}
		}
		if m.name != "" {
			byName[strings.ToLower(m.name)] = m
		}
	}

	for i := range contacts {
		name := strings.ToLower(contacts[i].DisplayName)
		if m, ok := byName[name]; ok {
			contacts[i].Email = m.email
			contacts[i].Nicknames = m.nicknames
		}
	}
}

// SearchContacts searches by name, email, or nickname.
// Results are sorted: nickname matches first, then name, then email.
func (c *Client) SearchContacts(query string) ([]Contact, error) {
	all, err := c.GetContacts()
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var nickMatches, nameMatches, emailMatches []Contact

	for _, ct := range all {
		nickHit := false
		for _, nick := range ct.Nicknames {
			if strings.Contains(strings.ToLower(nick), query) {
				nickHit = true
				break
			}
		}
		if nickHit {
			nickMatches = append(nickMatches, ct)
		} else if strings.Contains(strings.ToLower(ct.DisplayName), query) {
			nameMatches = append(nameMatches, ct)
		} else if strings.Contains(strings.ToLower(ct.Email), query) {
			emailMatches = append(emailMatches, ct)
		}
	}

	results := append(nickMatches, nameMatches...)
	results = append(results, emailMatches...)
	return results, nil
}

// DiscoverMessage is a simplified message for persona discovery.
type DiscoverMessage struct {
	From    string `json:"from"`
	Content string `json:"content"`
	Time    string `json:"time"`
}

// ContactConversation groups messages by chat type for a single contact.
type ContactConversation struct {
	ChatID   string            `json:"chat_id"`
	ChatName string            `json:"chat_name"`
	Type     string            `json:"type"` // dm, group, channel
	Messages []DiscoverMessage `json:"messages"`
}

// ContactDiscovery is the full discovery result for one contact.
type ContactDiscovery struct {
	Contact       string                `json:"contact"`
	TotalMessages int                   `json:"total_messages"`
	MyMessages    int                   `json:"my_messages"`
	TheirMessages int                   `json:"their_messages"`
	Conversations []ContactConversation `json:"conversations"`
}

// DiscoverContacts fetches messages from active chats in the last N days,
// grouped by contact then by chat. Returns data for persona building.
func (c *Client) DiscoverContacts(days int, maxChats int, msgsPerChat int) ([]ContactDiscovery, error) {
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	chats, err := c.ListChatsWithOptions(ChatListOptions{
		Limit:       0,
		ActiveSince: since,
		IncludeBots: false,
	})
	if err != nil {
		return nil, err
	}

	// Build set of my name variations for "me" tagging
	me, _ := c.GetMe()
	myNames := map[string]bool{}
	if me != nil {
		myNames[strings.ToLower(me.DisplayName)] = true
		if me.Email != "" {
			myNames[strings.ToLower(me.Email)] = true
			namePart := strings.Split(strings.ToLower(me.Email), "@")[0]
			myNames[strings.ReplaceAll(namePart, ".", " ")] = true
		}
	}
	isMe := func(name string) bool {
		low := strings.ToLower(name)
		if myNames[low] {
			return true
		}
		// Check if all words of name appear in any of myNames
		words := strings.Fields(low)
		if len(words) < 2 {
			return false
		}
		for my := range myNames {
			allFound := true
			for _, w := range words {
				if !strings.Contains(my, w) {
					allFound = false
					break
				}
			}
			if allFound {
				return true
			}
		}
		return false
	}

	// Limit number of chats to process
	if maxChats > 0 && len(chats) > maxChats {
		chats = chats[:maxChats]
	}

	cutoff := time.Now().AddDate(0, 0, -days)

	contactChats := map[string][]ContactConversation{}

	for i, chat := range chats {
		fmt.Fprintf(os.Stderr, "\r  Fetching %d/%d: %s", i+1, len(chats), truncStr(chat.Title, 40))

		msgs, err := c.GetMessagesWithOptions(chat.ID, msgsPerChat, GetMessagesOptions{})
		if err != nil {
			continue
		}

		// Filter to messages within the time window
		var filtered []DiscoverMessage
		for _, msg := range msgs {
			if t, err := time.Parse(time.RFC3339Nano, msg.Time); err == nil {
				if t.Before(cutoff) {
					continue
				}
			}
			from := msg.From
			if isMe(from) {
				from = "me"
			}
			filtered = append(filtered, DiscoverMessage{
				From:    from,
				Content: msg.Content,
				Time:    msg.Time,
			})
		}

		if len(filtered) == 0 {
			continue
		}

		ct := chat.Type
		chatName := chat.Title

		// For DMs, the contact is the other person
		if ct == "dm" || ct == "1:1" {
			for _, member := range chat.Members {
				if !isMe(member) {
					conv := ContactConversation{
						ChatID:   chat.ID,
						ChatName: chatName,
						Type:     ct,
						Messages: filtered,
					}
					contactChats[member] = append(contactChats[member], conv)
					break
				}
			}
		} else {
			// Group/channel — attribute to each non-me member who has messages
			participants := map[string]bool{}
			for _, msg := range filtered {
				if msg.From != "me" {
					participants[msg.From] = true
				}
			}
			conv := ContactConversation{
				ChatID:   chat.ID,
				ChatName: chatName,
				Type:     ct,
				Messages: filtered,
			}
			for p := range participants {
				contactChats[p] = append(contactChats[p], conv)
			}
		}
	}
	fmt.Fprintln(os.Stderr)

	// Build sorted result with counts
	var results []ContactDiscovery
	for contact, convs := range contactChats {
		d := ContactDiscovery{
			Contact:       contact,
			Conversations: convs,
		}
		for _, conv := range convs {
			for _, msg := range conv.Messages {
				d.TotalMessages++
				if msg.From == "me" {
					d.MyMessages++
				} else {
					d.TheirMessages++
				}
			}
		}
		results = append(results, d)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].TotalMessages > results[j].TotalMessages
	})

	return results, nil
}

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// LoadPersona reads the full persona file for a contact by email.
func LoadPersona(email string) string {
	path := filepath.Join(personaDir(), email+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
