package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/kamrul1157024/teams-cli/teams-cli/auth"
)

type ConsumptionHorizon struct {
	OriginalArrivalTime int    `json:"originalArrivalTime"`
	TimeStamp           int    `json:"timeStamp"`
	ClientMessageId     string `json:"clientMessageId"`
}

type LastMessage struct {
	MessageType     string `json:"messageType"`
	Content         string `json:"content"`
	ImDisplayName   string `json:"imDisplayName"`
	Id              string `json:"id"`
	Type            string `json:"type"`
	ComposeTime     string `json:"composeTime"`
	From            string `json:"from"`
	SequenceId      int    `json:"sequenceId"`
	Version         int    `json:"version"`
}

type ChatMember struct {
	IsMuted      bool   `json:"isMuted"`
	Mri          string `json:"mri"`
	Role         string `json:"role"`
	FriendlyName string `json:"friendlyName"`
	TenantId     string `json:"tenantId"`
	ObjectId     string `json:"objectId"`
}

type Chat struct {
	Id                  string             `json:"id"`
	Title               string             `json:"title"`
	ChatType            string             `json:"chatType"`
	ChatSubType         int                `json:"chatSubType"`
	IsOneOnOne          bool               `json:"isOneOnOne"`
	IsRead              bool               `json:"isRead"`
	IsDisabled          bool               `json:"isDisabled"`
	Hidden              bool               `json:"hidden"`
	IsSticky            bool               `json:"isSticky"`
	LastMessage         LastMessage        `json:"lastMessage"`
	Members             []ChatMember       `json:"members"`
	CreatedAt           string             `json:"createdAt"`
	TenantId            string             `json:"tenantId"`
	ConsumptionHorizon  ConsumptionHorizon `json:"consumptionHorizon"`
}

type Channel struct {
	Id          string      `json:"id"`
	DisplayName string      `json:"displayName"`
	Description string      `json:"description"`
	IsGeneral   bool        `json:"isGeneral"`
	IsFavorite  bool        `json:"isFavorite"`
	IsPinned    bool        `json:"isPinned"`
	IsDeleted   bool        `json:"isDeleted"`
	IsArchived  bool        `json:"isArchived"`
	IsMember    bool        `json:"isMember"`
	LastMessage LastMessage `json:"lastMessage"`
	TenantId    string      `json:"tenantId"`
}

type MembershipSummary struct {
	TotalMemberCount int `json:"totalMemberCount"`
	AdminRoleCount   int `json:"adminRoleCount"`
	UserRoleCount    int `json:"userRoleCount"`
	GuestRoleCount   int `json:"guestRoleCount"`
}

type Team struct {
	Id                string             `json:"id"`
	DisplayName       string             `json:"displayName"`
	Description       string             `json:"description"`
	IsFavorite        bool               `json:"isFavorite"`
	IsArchived        bool               `json:"isArchived"`
	IsDeleted         bool               `json:"isDeleted"`
	Channels          []Channel          `json:"channels"`
	MembershipSummary *MembershipSummary `json:"membershipSummary"`
	TenantId          string             `json:"tenantId"`
}

type ConversationMetadata struct {
	SyncToken     string `json:"syncToken"`
	IsPartialData bool   `json:"isPartialData"`
}

type ConversationResponse struct {
	Chats    []Chat                `json:"chats"`
	Teams    []Team                `json:"teams"`
	Metadata ConversationMetadata  `json:"_metadata"`
}

func (c *Client) GetConversations() (*ConversationResponse, error) {
	url := ChatSvcAggBase + "teams/users/me?isPrefetch=false&enableMembershipSummary=true"
	var result ConversationResponse
	if err := c.getJSON(url, auth.TokenChatSvcAgg, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ChatListItem is a simplified view of a chat for list output
type ChatListItem struct {
	ID                string       `json:"id"`
	Title             string       `json:"title"`
	Type              string       `json:"type"`
	IsRead            bool         `json:"is_read"`
	MemberCount       int          `json:"member_count"`
	LastMessage       *ChatLastMsg `json:"last_message,omitempty"`
	LastMessageFromMe bool         `json:"last_message_from_me"`
	Members           []string     `json:"members,omitempty"`
}

type ChatLastMsg struct {
	From    string `json:"from"`
	Content string `json:"content"`
	Time    string `json:"time"`
}

// ChatListResponse wraps the chat list with pagination metadata
type ChatListResponse struct {
	Chats []ChatListItem         `json:"chats"`
	Meta  map[string]interface{} `json:"_meta"`
}

func chatType(chat Chat) string {
	if chat.IsOneOnOne || chat.ChatType == "oneOnOne" || len(chat.Members) == 2 {
		return "1:1"
	}
	return "group"
}

// ChatListOptions holds all filtering options for ListChats
type ChatListOptions struct {
	FilterType  string
	UnreadOnly  bool
	Limit       int
	Offset      int
	Compact     bool
	WithPerson  string
	ActiveSince string
}

func (c *Client) ListChats(filterType string, unreadOnly bool, limit int) ([]ChatListItem, error) {
	return c.ListChatsWithOptions(ChatListOptions{
		FilterType: filterType,
		UnreadOnly: unreadOnly,
		Limit:      limit,
	})
}

func (c *Client) ListChatsWithOptions(opts ChatListOptions) ([]ChatListItem, error) {
	convs, err := c.GetConversations()
	if err != nil {
		return nil, err
	}

	// Parse active-since date if provided
	var activeSince time.Time
	if opts.ActiveSince != "" {
		for _, layout := range []string{"2006-01-02", "2006-01-02T15:04:05Z", time.RFC3339} {
			if t, err := time.Parse(layout, opts.ActiveSince); err == nil {
				activeSince = t
				break
			}
		}
	}

	withPerson := strings.ToLower(opts.WithPerson)

	// Build name map from message senders for resolving MRI members
	nameMap := buildNameMap(convs.Chats)

	// Get current user email for last_message_from_me
	myEmail, _ := auth.GetEmail()
	myEmailLower := strings.ToLower(myEmail)

	var items []ChatListItem
	skipped := 0

	for _, chat := range convs.Chats {
		if chat.Hidden {
			continue
		}

		ct := chatType(chat)
		if opts.FilterType != "" && ct != opts.FilterType {
			continue
		}
		if opts.UnreadOnly && chat.IsRead {
			continue
		}

		// Filter by active-since
		if !activeSince.IsZero() && chat.LastMessage.ComposeTime != "" {
			if msgTime, err := time.Parse(time.RFC3339Nano, chat.LastMessage.ComposeTime); err == nil {
				if msgTime.Before(activeSince) {
					continue
				}
			}
		}

		// Filter by --with person
		if withPerson != "" {
			found := false
			for _, m := range chat.Members {
				name := strings.ToLower(resolveName(m, nameMap))
				if strings.Contains(name, withPerson) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Handle offset
		if opts.Offset > 0 && skipped < opts.Offset {
			skipped++
			continue
		}

		item := ChatListItem{
			ID:          chat.Id,
			Title:       chat.Title,
			Type:        ct,
			IsRead:      chat.IsRead,
			MemberCount: len(chat.Members),
		}

		if chat.LastMessage.Content != "" {
			item.LastMessage = &ChatLastMsg{
				From:    chat.LastMessage.ImDisplayName,
				Content: stripHTML(chat.LastMessage.Content),
				Time:    chat.LastMessage.ComposeTime,
			}
			// Check if last message is from the current user
			if myEmailLower != "" {
				lastFrom := strings.ToLower(chat.LastMessage.ImDisplayName)
				if strings.Contains(lastFrom, myEmailLower) || strings.Contains(strings.ToLower(chat.LastMessage.From), myEmailLower) {
					item.LastMessageFromMe = true
				}
			}
		}

		if !opts.Compact {
			for _, m := range chat.Members {
				item.Members = append(item.Members, resolveName(m, nameMap))
			}
		}

		// If no title, build from members
		if item.Title == "" {
			if opts.Compact {
				// Resolve names just for title
				var names []string
				for _, m := range chat.Members {
					names = append(names, resolveName(m, nameMap))
				}
				item.Title = joinNames(names, 3)
			} else if len(item.Members) > 0 {
				item.Title = joinNames(item.Members, 3)
			}
		}

		items = append(items, item)

		if opts.Limit > 0 && len(items) >= opts.Limit {
			break
		}
	}

	return items, nil
}

// buildNameMap creates a map of MRI -> display name from chat message senders
func buildNameMap(chats []Chat) map[string]string {
	m := map[string]string{}
	for _, chat := range chats {
		if chat.LastMessage.ImDisplayName != "" && chat.LastMessage.From != "" {
			m[chat.LastMessage.From] = chat.LastMessage.ImDisplayName
		}
		for _, member := range chat.Members {
			if member.FriendlyName != "" {
				m[member.Mri] = member.FriendlyName
			}
		}
	}
	return m
}

// resolveName returns a human-readable name for a chat member
func resolveName(m ChatMember, nameMap map[string]string) string {
	if m.FriendlyName != "" {
		return m.FriendlyName
	}
	if name, ok := nameMap[m.Mri]; ok {
		return name
	}
	return m.Mri
}

// TeamListItem is a simplified view of a team for list output
type TeamListItem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	IsFavorite   bool   `json:"is_favorite"`
	IsArchived   bool   `json:"is_archived"`
	ChannelCount int    `json:"channel_count"`
	MemberCount  int    `json:"member_count,omitempty"`
}

func (c *Client) ListTeams() ([]TeamListItem, error) {
	convs, err := c.GetConversations()
	if err != nil {
		return nil, err
	}

	var items []TeamListItem
	for _, team := range convs.Teams {
		if team.IsDeleted {
			continue
		}
		item := TeamListItem{
			ID:           team.Id,
			Name:         team.DisplayName,
			IsFavorite:   team.IsFavorite,
			IsArchived:   team.IsArchived,
			ChannelCount: len(team.Channels),
		}
		if team.MembershipSummary != nil {
			item.MemberCount = team.MembershipSummary.TotalMemberCount
		}
		items = append(items, item)
	}
	return items, nil
}

// ChannelListItem is a simplified view of a channel for list output
type ChannelListItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsGeneral  bool   `json:"is_general"`
	IsFavorite bool   `json:"is_favorite"`
	IsPinned   bool   `json:"is_pinned"`
	Team       string `json:"team"`
}

func (c *Client) ListChannels(teamID string) ([]ChannelListItem, error) {
	convs, err := c.GetConversations()
	if err != nil {
		return nil, err
	}

	for _, team := range convs.Teams {
		if team.Id != teamID {
			continue
		}
		var items []ChannelListItem
		for _, ch := range team.Channels {
			if ch.IsDeleted {
				continue
			}
			items = append(items, ChannelListItem{
				ID:         ch.Id,
				Name:       ch.DisplayName,
				IsGeneral:  ch.IsGeneral,
				IsFavorite: ch.IsFavorite,
				IsPinned:   ch.IsPinned,
				Team:       team.DisplayName,
			})
		}
		return items, nil
	}

	return nil, fmt.Errorf("team %s not found", teamID)
}

// helper: strip HTML tags from content
func stripHTML(s string) string {
	var result []byte
	inTag := false
	for i := 0; i < len(s); i++ {
		if s[i] == '<' {
			inTag = true
			continue
		}
		if s[i] == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result = append(result, s[i])
		}
	}
	return string(result)
}

func joinNames(names []string, max int) string {
	if len(names) <= max {
		return join(names, ", ")
	}
	return join(names[:max], ", ") + fmt.Sprintf(" +%d more", len(names)-max)
}

func join(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

func formatTime(t string) string {
	parsed, err := time.Parse(time.RFC3339Nano, t)
	if err != nil {
		return t
	}
	elapsed := time.Since(parsed)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(elapsed.Hours()/24))
	}
}
