package api

import (
	"fmt"
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
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	IsRead      bool     `json:"is_read"`
	LastMessage *ChatLastMsg `json:"last_message,omitempty"`
	Members     []string `json:"members,omitempty"`
}

type ChatLastMsg struct {
	From    string `json:"from"`
	Content string `json:"content"`
	Time    string `json:"time"`
}

func chatType(chat Chat) string {
	if chat.IsOneOnOne || chat.ChatType == "oneOnOne" || len(chat.Members) == 2 {
		return "1:1"
	}
	return "group"
}

func (c *Client) ListChats(filterType string, unreadOnly bool, limit int) ([]ChatListItem, error) {
	convs, err := c.GetConversations()
	if err != nil {
		return nil, err
	}

	var items []ChatListItem
	for _, chat := range convs.Chats {
		if chat.Hidden {
			continue
		}

		ct := chatType(chat)
		if filterType != "" && ct != filterType {
			continue
		}
		if unreadOnly && chat.IsRead {
			continue
		}

		item := ChatListItem{
			ID:     chat.Id,
			Title:  chat.Title,
			Type:   ct,
			IsRead: chat.IsRead,
		}

		if chat.LastMessage.Content != "" {
			item.LastMessage = &ChatLastMsg{
				From:    chat.LastMessage.ImDisplayName,
				Content: stripHTML(chat.LastMessage.Content),
				Time:    chat.LastMessage.ComposeTime,
			}
		}

		for _, m := range chat.Members {
			if m.FriendlyName != "" {
				item.Members = append(item.Members, m.FriendlyName)
			} else {
				item.Members = append(item.Members, m.Mri)
			}
		}

		// If no title, build from members
		if item.Title == "" && len(item.Members) > 0 {
			item.Title = joinNames(item.Members, 3)
		}

		items = append(items, item)

		if limit > 0 && len(items) >= limit {
			break
		}
	}

	return items, nil
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
