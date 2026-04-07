package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/kamrul1157024/teams-cli/teams-cli/auth"
)

type ChatMessage struct {
	Id              string `json:"id"`
	SequenceId      int64  `json:"sequenceId"`
	ClientMessageId string `json:"clientMessageId"`
	Version         string `json:"version"`
	ConversationId  string `json:"conversationId"`
	Type            string `json:"type"`
	MessageType     string `json:"messagetype"`
	ContentType     string `json:"contenttype"`
	Content         string `json:"content"`
	From            string `json:"from"`
	ImDisplayName   string `json:"imdisplayname"`
	ComposeTime     string `json:"composetime"`
}

type MessagesResponse struct {
	Messages []ChatMessage `json:"messages"`
}

type MessageListItem struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	Content string `json:"content"`
	Time    string `json:"time"`
	Type    string `json:"type"`
}

func (c *Client) GetMessages(chatID string, limit int) ([]MessageListItem, error) {
	if limit <= 0 {
		limit = 50
	}

	endpoint := MessagesBase + "users/ME/conversations/" + url.PathEscape(chatID) + "/messages"
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("view", "msnp24Equivalent|supportsMessageProperties")
	q.Set("pageSize", fmt.Sprintf("%d", limit))
	q.Set("startTime", "1")
	u.RawQuery = q.Encode()

	var msgResp MessagesResponse
	if err := c.getJSON(u.String(), auth.TokenSkype, &msgResp); err != nil {
		return nil, err
	}

	// Sort by time ascending
	sortMessagesByTime(msgResp.Messages)

	// Filter to actual messages only
	var items []MessageListItem
	for _, msg := range msgResp.Messages {
		if msg.MessageType != "" && msg.MessageType != "Text" &&
			msg.MessageType != "RichText" && msg.MessageType != "RichText/Html" {
			continue
		}
		if msg.Type != "Message" && msg.Type != "" {
			continue
		}

		item := MessageListItem{
			ID:      msg.Id,
			From:    msg.ImDisplayName,
			Content: stripHTML(msg.Content),
			Time:    msg.ComposeTime,
			Type:    msg.MessageType,
		}
		items = append(items, item)
	}

	return items, nil
}

type SendMessageRequest struct {
	Content         string                 `json:"content"`
	MessageType     string                 `json:"messagetype"`
	ContentType     string                 `json:"contenttype"`
	ClientMessageId string                 `json:"clientmessageid"`
	ImDisplayName   string                 `json:"imdisplayname"`
	Properties      map[string]interface{} `json:"properties"`
}

type SendMessageResponse struct {
	Id                  string      `json:"id"`
	OriginalArrivalTime interface{} `json:"OriginalArrivalTime"`
}

type SendResult struct {
	Status    string `json:"status"`
	MessageID string `json:"message_id"`
	ChatID    string `json:"chat_id"`
	Time      string `json:"time"`
}

func (c *Client) SendMessage(chatID string, content string) (*SendResult, error) {
	displayName := ""
	email, err := auth.GetEmail()
	if err == nil {
		displayName = email
	}

	// Wrap plain text in HTML
	if !strings.HasPrefix(content, "<") {
		content = "<p>" + content + "</p>"
	}

	body := SendMessageRequest{
		Content:         content,
		MessageType:     "RichText/Html",
		ContentType:     "text",
		ClientMessageId: fmt.Sprintf("%d", time.Now().UnixNano()/1e6),
		ImDisplayName:   displayName,
		Properties: map[string]interface{}{
			"importance": "",
			"subject":    "",
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal message: %w", err)
	}

	endpoint := MessagesBase + "users/ME/conversations/" + url.PathEscape(chatID) + "/messages"

	var resp SendMessageResponse
	if err := c.postJSON(endpoint, auth.TokenSkype, bytes.NewReader(jsonBody), &resp); err != nil {
		return nil, err
	}

	return &SendResult{
		Status:    "sent",
		MessageID: resp.Id,
		ChatID:    chatID,
		Time:      fmt.Sprintf("%v", resp.OriginalArrivalTime),
	}, nil
}

// FindChatByEmail finds a 1:1 chat with the given email
func (c *Client) FindChatByEmail(email string) (string, error) {
	convs, err := c.GetConversations()
	if err != nil {
		return "", err
	}

	email = strings.ToLower(email)
	for _, chat := range convs.Chats {
		if !chat.IsOneOnOne {
			continue
		}
		for _, m := range chat.Members {
			mri := strings.ToLower(m.Mri)
			if strings.Contains(mri, email) {
				return chat.Id, nil
			}
		}
	}

	return "", fmt.Errorf("no existing 1:1 chat found with %s", email)
}

// SearchMessages searches messages across conversations
func (c *Client) SearchMessages(query string, chatID string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}

	var chatIDs []string
	if chatID != "" {
		chatIDs = []string{chatID}
	} else {
		convs, err := c.GetConversations()
		if err != nil {
			return nil, err
		}
		for _, chat := range convs.Chats {
			if !chat.Hidden {
				chatIDs = append(chatIDs, chat.Id)
			}
		}
		// Limit to first 20 chats for performance
		if len(chatIDs) > 20 {
			chatIDs = chatIDs[:20]
		}
	}

	query = strings.ToLower(query)
	var results []SearchResult

	for _, id := range chatIDs {
		messages, err := c.GetMessages(id, 100)
		if err != nil {
			continue
		}
		for _, msg := range messages {
			if strings.Contains(strings.ToLower(msg.Content), query) {
				results = append(results, SearchResult{
					ChatID:  id,
					Message: msg,
					Match:   query,
				})
				if len(results) >= limit {
					return results, nil
				}
			}
		}
	}

	return results, nil
}

type SearchResult struct {
	ChatID  string          `json:"chat_id"`
	Message MessageListItem `json:"message"`
	Match   string          `json:"match"`
}

func sortMessagesByTime(messages []ChatMessage) {
	for i := 0; i < len(messages); i++ {
		for j := i + 1; j < len(messages); j++ {
			ti, _ := time.Parse(time.RFC3339Nano, messages[i].ComposeTime)
			tj, _ := time.Parse(time.RFC3339Nano, messages[j].ComposeTime)
			if ti.After(tj) {
				messages[i], messages[j] = messages[j], messages[i]
			}
		}
	}
}
