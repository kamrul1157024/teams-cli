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

type MessageListResponse struct {
	Messages []MessageListItem      `json:"messages"`
	Meta     map[string]interface{} `json:"_meta,omitempty"`
}

// GetMessagesOptions holds optional parameters for GetMessages
type GetMessagesOptions struct {
	Before string // Cursor: fetch messages before this time
	After  string // Cursor: fetch messages after this time
}

func (c *Client) GetMessages(chatID string, limit int) ([]MessageListItem, error) {
	return c.GetMessagesWithOptions(chatID, limit, GetMessagesOptions{})
}

func (c *Client) GetMessagesWithOptions(chatID string, limit int, opts GetMessagesOptions) ([]MessageListItem, error) {
	if limit <= 0 {
		limit = 50
	}

	endpoint := MessagesBase + "users/ME/conversations/" + url.PathEscape(chatID) + "/messages"
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("view", "msnp24Equivalent|supportsMessageProperties")
	q.Set("pageSize", fmt.Sprintf("%d", limit))

	if opts.Before != "" {
		// Use before time as endTime
		if t, err := time.Parse(time.RFC3339Nano, opts.Before); err == nil {
			q.Set("endTime", fmt.Sprintf("%d", t.UnixMilli()))
		}
		q.Set("startTime", "1")
	} else if opts.After != "" {
		// Use after time as startTime
		if t, err := time.Parse(time.RFC3339Nano, opts.After); err == nil {
			q.Set("startTime", fmt.Sprintf("%d", t.UnixMilli()))
		}
	} else {
		q.Set("startTime", "1")
	}
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

		from := msg.ImDisplayName
		if from == "" {
			// Fall back to MRI-based from field
			from = msg.From
		}

		item := MessageListItem{
			ID:      msg.Id,
			From:    from,
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
	Content   string `json:"content,omitempty"`
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

	// Append signature as a separate line if enabled
	cfg := LoadConfig()
	if cfg.SignatureEnabled && cfg.Signature != "" {
		content = content + "<p><em>— " + cfg.Signature + "</em></p>"
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

	// Invalidate conversations cache — unread state and last message changed
	c.CacheInvalidate("convs")

	return &SendResult{
		Status:    "sent",
		MessageID: resp.Id,
		ChatID:    chatID,
		Time:      fmt.Sprintf("%v", resp.OriginalArrivalTime),
	}, nil
}

// ReactToMessage adds an emoji reaction to a message
func (c *Client) ReactToMessage(chatID, messageID, emoji string) error {
	// Map common emoji names to Teams emotion keys
	emojiMap := map[string]string{
		"like":      "like",
		"heart":     "heart",
		"laugh":     "laugh",
		"surprised": "surprised",
		"sad":       "sad",
		"angry":     "angry",
		"👍":         "like",
		"❤️":        "heart",
		"😂":         "laugh",
		"😮":         "surprised",
		"😢":         "sad",
		"😡":         "angry",
		"thumbsup":  "like",
		"love":      "heart",
		"haha":      "laugh",
		"wow":       "surprised",
	}

	emotionKey, ok := emojiMap[strings.ToLower(emoji)]
	if !ok {
		emotionKey = strings.ToLower(emoji)
	}

	// Validate it's a known Teams reaction
	validReactions := map[string]bool{
		"like": true, "heart": true, "laugh": true,
		"surprised": true, "sad": true, "angry": true,
	}
	if !validReactions[emotionKey] {
		return fmt.Errorf("unknown reaction %q — valid: like, heart, laugh, surprised, sad, angry", emoji)
	}

	myEmail, err := auth.GetEmail()
	if err != nil {
		return fmt.Errorf("cannot get user email: %w", err)
	}

	me, err := c.GetMe()
	if err != nil {
		return fmt.Errorf("cannot get user info: %w", err)
	}

	reaction := map[string]interface{}{
		"key": emotionKey,
		"user": map[string]interface{}{
			"mri":         me.Mri,
			"displayName": me.DisplayName,
			"email":       myEmail,
		},
	}

	// The emotions property is a JSON string containing an array
	emotions := []interface{}{reaction}
	emotionsJSON, err := json.Marshal(emotions)
	if err != nil {
		return fmt.Errorf("cannot marshal reaction: %w", err)
	}

	endpoint := MessagesBase + "users/ME/conversations/" + url.PathEscape(chatID) +
		"/messages/" + url.PathEscape(messageID) + "/properties?name=emotions"

	return c.putRequest(endpoint, auth.TokenSkype, bytes.NewReader(emotionsJSON))
}

// EditMessage edits a previously sent message
func (c *Client) EditMessage(chatID, messageID, newContent string) error {
	// Wrap plain text in HTML
	if !strings.HasPrefix(newContent, "<") {
		newContent = "<p>" + newContent + "</p>"
	}

	body := map[string]interface{}{
		"content":     newContent,
		"messagetype": "RichText/Html",
		"contenttype": "text",
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("cannot marshal edit: %w", err)
	}

	endpoint := MessagesBase + "users/ME/conversations/" + url.PathEscape(chatID) +
		"/messages/" + url.PathEscape(messageID)

	return c.putRequest(endpoint, auth.TokenSkype, bytes.NewReader(jsonBody))
}

// DeleteMessage soft-deletes a previously sent message
func (c *Client) DeleteMessage(chatID, messageID string) error {
	body := map[string]interface{}{
		"content":       "",
		"messagetype":   "RichText/Html",
		"contenttype":   "text",
		"skypeeditedid": messageID,
		"properties": map[string]interface{}{
			"deletetime": fmt.Sprintf("%d", time.Now().UnixMilli()),
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("cannot marshal delete: %w", err)
	}

	endpoint := MessagesBase + "users/ME/conversations/" + url.PathEscape(chatID) +
		"/messages/" + url.PathEscape(messageID)

	return c.putRequest(endpoint, auth.TokenSkype, bytes.NewReader(jsonBody))
}

// ReplyToMessage sends a threaded reply to a specific message.
// For channel messages (@thread.tacv2), posts to the thread conversation ID (channelID;messageid=parentID).
// For chat messages, posts to /messages with replyChain in properties.
func (c *Client) ReplyToMessage(chatID, parentMessageID, content string) (*SendResult, error) {
	displayName := ""
	email, err := auth.GetEmail()
	if err == nil {
		displayName = email
	}

	// Wrap plain text in HTML
	if !strings.HasPrefix(content, "<") {
		content = "<p>" + content + "</p>"
	}

	// Append signature if enabled
	cfg := LoadConfig()
	if cfg.SignatureEnabled && cfg.Signature != "" {
		content = content + "<p><em>— " + cfg.Signature + "</em></p>"
	}

	body := map[string]interface{}{
		"content":         content,
		"messagetype":     "RichText/Html",
		"contenttype":     "text",
		"clientmessageid": fmt.Sprintf("%d", time.Now().UnixNano()/1e6),
		"imdisplayname":   displayName,
		"properties": map[string]interface{}{
			"importance": "",
			"subject":    "",
		},
	}

	// For channel threads, post to the thread conversation ID: channelID;messageid=parentID
	// For chat replies, post to /messages with replyChain in properties
	convID := chatID
	if strings.Contains(chatID, "@thread.tacv2") {
		convID = chatID + ";messageid=" + parentMessageID
	} else {
		// Chat replies use replyChain property
		replyChain, _ := json.Marshal([]map[string]string{
			{"messageId": parentMessageID, "conversationId": chatID},
		})
		props := body["properties"].(map[string]interface{})
		props["replyChain"] = string(replyChain)
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal reply: %w", err)
	}

	endpoint := MessagesBase + "users/ME/conversations/" + url.PathEscape(convID) + "/messages"

	var resp SendMessageResponse
	if err := c.postJSON(endpoint, auth.TokenSkype, bytes.NewReader(jsonBody), &resp); err != nil {
		return nil, err
	}

	return &SendResult{
		Status:    "replied",
		MessageID: resp.Id,
		ChatID:    chatID,
		Time:      fmt.Sprintf("%v", resp.OriginalArrivalTime),
	}, nil
}

// FindChatByName finds a group chat by its title/name
func (c *Client) FindChatByName(name string) (string, error) {
	convs, err := c.GetConversations()
	if err != nil {
		return "", err
	}

	name = strings.ToLower(name)
	var bestMatch string
	for _, chat := range convs.Chats {
		if chat.Hidden {
			continue
		}
		title := strings.ToLower(chat.Title)
		if title == name {
			return chat.Id, nil // Exact match
		}
		if strings.Contains(title, name) && bestMatch == "" {
			bestMatch = chat.Id
		}
	}

	if bestMatch != "" {
		return bestMatch, nil
	}
	return "", fmt.Errorf("no chat found matching %q", name)
}

// ConvertMarkdownToHTML converts basic markdown to Teams HTML
func ConvertMarkdownToHTML(md string) string {
	lines := strings.Split(md, "\n")
	var result []string
	inCodeBlock := false
	var codeLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End code block
				code := strings.Join(codeLines, "\n")
				result = append(result, "<pre><code>"+escapeHTML(code)+"</code></pre>")
				codeLines = nil
				inCodeBlock = false
			} else {
				inCodeBlock = true
			}
			continue
		}
		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		// Inline formatting
		line = convertInlineMarkdown(line)

		// Headers
		if strings.HasPrefix(line, "### ") {
			result = append(result, "<h3>"+line[4:]+"</h3>")
		} else if strings.HasPrefix(line, "## ") {
			result = append(result, "<h2>"+line[3:]+"</h2>")
		} else if strings.HasPrefix(line, "# ") {
			result = append(result, "<h1>"+line[2:]+"</h1>")
		} else if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			result = append(result, "<li>"+line[2:]+"</li>")
		} else if line == "" {
			result = append(result, "<br/>")
		} else {
			result = append(result, "<p>"+line+"</p>")
		}
	}

	// Close unclosed code block
	if inCodeBlock && len(codeLines) > 0 {
		code := strings.Join(codeLines, "\n")
		result = append(result, "<pre><code>"+escapeHTML(code)+"</code></pre>")
	}

	return strings.Join(result, "")
}

func convertInlineMarkdown(s string) string {
	// Bold: **text** or __text__
	s = replaceMarkdownPairs(s, "**", "<b>", "</b>")
	s = replaceMarkdownPairs(s, "__", "<b>", "</b>")
	// Italic: *text* or _text_
	s = replaceMarkdownPairs(s, "*", "<i>", "</i>")
	s = replaceMarkdownPairs(s, "_", "<i>", "</i>")
	// Inline code: `text`
	s = replaceMarkdownPairs(s, "`", "<code>", "</code>")
	// Strikethrough: ~~text~~
	s = replaceMarkdownPairs(s, "~~", "<s>", "</s>")
	return s
}

func replaceMarkdownPairs(s, marker, openTag, closeTag string) string {
	for {
		start := strings.Index(s, marker)
		if start == -1 {
			break
		}
		end := strings.Index(s[start+len(marker):], marker)
		if end == -1 {
			break
		}
		end += start + len(marker)
		inner := s[start+len(marker) : end]
		s = s[:start] + openTag + inner + closeTag + s[end+len(marker):]
	}
	return s
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// BuildMentionHTML creates a Teams @mention HTML tag for a user
func (c *Client) BuildMentionHTML(email string) (string, error) {
	user, err := c.GetUser(email)
	if err != nil {
		return "", fmt.Errorf("cannot find user %s: %w", email, err)
	}

	name := user.DisplayName
	if name == "" {
		name = email
	}

	return fmt.Sprintf(`<at id="%s">%s</at>`, user.Mri, name), nil
}

// BuildQuoteHTML fetches a message and returns a Teams-formatted blockquote embedding it.
// The chatID can be a channel or chat ID; messageID is the message to quote.
func (c *Client) BuildQuoteHTML(chatID, messageID string) (string, error) {
	// Fetch recent messages and find the target
	messages, err := c.GetMessagesRaw(chatID, 50)
	if err != nil {
		return "", fmt.Errorf("cannot fetch messages: %w", err)
	}

	var found *ChatMessage
	for i := range messages {
		if messages[i].Id == messageID {
			found = &messages[i]
			break
		}
	}
	if found == nil {
		return "", fmt.Errorf("message %s not found in conversation", messageID)
	}

	// Extract sender MRI from the "from" URL (last path segment)
	senderMRI := found.From
	if parts := strings.Split(found.From, "/"); len(parts) > 0 {
		senderMRI = parts[len(parts)-1]
	}

	senderName := found.ImDisplayName
	if senderName == "" {
		senderName = senderMRI
	}

	preview := stripHTML(found.Content)
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}

	quote := fmt.Sprintf(
		`<blockquote itemscope itemtype="http://schema.skype.com/Reply" itemid="%s">`+
			`<strong itemprop="mri" itemid="%s">%s</strong>`+
			`<span itemprop="time" itemid="%s"></span>`+
			`<p itemprop="preview">%s</p>`+
			`</blockquote>`,
		messageID, senderMRI, escapeHTML(senderName), messageID, escapeHTML(preview),
	)
	return quote, nil
}

// GetMessagesRaw returns raw ChatMessage structs (with MRI info) for a conversation
func (c *Client) GetMessagesRaw(chatID string, limit int) ([]ChatMessage, error) {
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

	return msgResp.Messages, nil
}

// NotificationItem represents a parsed notification
type NotificationItem struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Content      string `json:"content"`
	From         string `json:"from"`
	Context      string `json:"context"`
	ChatID       string `json:"chat_id,omitempty"`
	Time         string `json:"time"`
}

// GetNotifications fetches the activity/notification feed
func (c *Client) GetNotifications(limit int, filterType string, since string) ([]NotificationItem, error) {
	if limit <= 0 {
		limit = 30
	}

	endpoint := MessagesBase + "users/ME/conversations/48:notifications/messages?pageSize=" +
		fmt.Sprintf("%d", limit) + "&startTime=0&view=msnp24Equivalent"

	rawResp, err := c.doRequest("GET", endpoint, nil, auth.TokenSkype)
	if err != nil {
		return nil, err
	}
	defer rawResp.Body.Close()

	var rawResult struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.NewDecoder(rawResp.Body).Decode(&rawResult); err != nil {
		return nil, fmt.Errorf("cannot decode notifications: %w", err)
	}

	var items []NotificationItem
	for _, raw := range rawResult.Messages {
		var msg map[string]interface{}
		json.Unmarshal(raw, &msg)

		actType := ""
		context := ""
		chatID := ""
		from := ""

		if props, ok := msg["properties"].(map[string]interface{}); ok {
			if activity, ok := props["activity"].(map[string]interface{}); ok {
				if at, ok := activity["activityType"].(string); ok {
					actType = at
				}
				if ctx, ok := activity["activityContext"].(map[string]interface{}); ok {
					if ct, ok := ctx["ClumpTitle"].(string); ok {
						context = ct
					}
				}
			}
		}

		if clumpID, ok := msg["clumpId"].(string); ok {
			chatID = clumpID
		}

		if fromLink, ok := msg["from"].(string); ok {
			// Extract MRI from URL
			parts := strings.Split(fromLink, "/")
			if len(parts) > 0 {
				from = parts[len(parts)-1]
			}
		}
		if dn, ok := msg["imdisplayname"].(string); ok && dn != "" {
			from = dn
		}

		content := ""
		if c, ok := msg["content"].(string); ok {
			content = stripHTML(c)
		}

		composeTime := ""
		if ct, ok := msg["composetime"].(string); ok {
			composeTime = ct
		}

		// Filter by --since time
		if since != "" && composeTime != "" {
			var sinceTime time.Time
			for _, layout := range []string{"2006-01-02", "2006-01-02T15:04:05Z", time.RFC3339} {
				if t, err := time.Parse(layout, since); err == nil {
					sinceTime = t
					break
				}
			}
			if !sinceTime.IsZero() {
				if msgTime, err := time.Parse(time.RFC3339Nano, composeTime); err == nil {
					if msgTime.Before(sinceTime) {
						continue
					}
				}
			}
		}

		// Filter by type if specified
		if filterType != "" {
			match := false
			switch filterType {
			case "mention", "mentions":
				match = actType == "mention" || actType == "mentionInChat"
			case "reply", "replies":
				match = actType == "replyToReply" || actType == "replyToConversation"
			case "reaction", "reactions":
				match = actType == "reactionInChat"
			default:
				match = actType == filterType
			}
			if !match {
				continue
			}
		}

		items = append(items, NotificationItem{
			ID:      fmt.Sprintf("%v", msg["id"]),
			Type:    actType,
			Content: content,
			From:    from,
			Context: context,
			ChatID:  chatID,
			Time:    composeTime,
		})
	}

	return items, nil
}

// FindChatByEmail finds a 1:1 chat with the given email.
// It prefers true 1:1 chats over meeting chats, and falls back to creating
// a new 1:1 chat if none exists.
func (c *Client) FindChatByEmail(email string) (string, error) {
	// Check cache for email -> chat ID mapping
	key := cacheKey("resolve", strings.ToLower(email))
	var cachedID string
	if c.cacheGet(key, &cachedID) {
		return cachedID, nil
	}

	// Look up the user to get their MRI (MRIs are UUIDs, not emails)
	user, err := c.GetUser(email)
	if err != nil {
		return "", fmt.Errorf("cannot find user %s: %w", email, err)
	}

	convs, err := c.GetConversations()
	if err != nil {
		return "", err
	}

	targetMri := strings.ToLower(user.Mri)
	email = strings.ToLower(email)
	var fallbackChatID string

	for _, chat := range convs.Chats {
		isOneOnOne := chat.IsOneOnOne || chat.ChatType == "oneOnOne" || len(chat.Members) == 2
		if !isOneOnOne {
			continue
		}
		for _, m := range chat.Members {
			mri := strings.ToLower(m.Mri)
			// Match by MRI (primary) or by email in MRI (fallback for older formats)
			if mri == targetMri || strings.Contains(mri, email) {
				// Prefer non-meeting chats
				if !strings.Contains(chat.Id, "meeting_") {
					c.cacheSet(key, chat.Id, TTLResolveChat)
					return chat.Id, nil
				}
				// Keep meeting chat as fallback
				if fallbackChatID == "" {
					fallbackChatID = chat.Id
				}
			}
		}
	}

	if fallbackChatID != "" {
		c.cacheSet(key, fallbackChatID, TTLResolveChat)
		return fallbackChatID, nil
	}

	// No existing chat found — create a new 1:1 chat
	if user.Mri == "" {
		return "", fmt.Errorf("user %s has no MRI — cannot create chat", email)
	}
	return c.CreateOneOnOneChat(user.Mri)
}

// CreateOneOnOneChat creates a new 1:1 chat thread with the given user MRI
func (c *Client) CreateOneOnOneChat(targetMri string) (string, error) {
	// Get our own MRI
	me, err := c.GetMe()
	if err != nil {
		return "", fmt.Errorf("cannot get current user: %w", err)
	}
	if me.Mri == "" {
		return "", fmt.Errorf("current user has no MRI — cannot create chat")
	}

	body := map[string]interface{}{
		"members": []map[string]interface{}{
			{"id": me.Mri, "role": "Admin"},
			{"id": targetMri, "role": "Admin"},
		},
		"properties": map[string]interface{}{
			"threadType":       "chat",
			"chatFilesEnabled": "true",
			"fixedRoster":      "true",
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("cannot marshal create chat request: %w", err)
	}

	endpoint := MessagesBase + "threads"

	resp, err := c.doRequest("POST", endpoint, bytes.NewReader(jsonBody), auth.TokenSkype)
	if err != nil {
		return "", fmt.Errorf("failed to create chat: %w", err)
	}
	defer resp.Body.Close()

	// The response Location header or body contains the new thread ID
	loc := resp.Header.Get("Location")
	if loc != "" {
		// Location is typically the full URL; extract the thread ID
		parts := strings.Split(loc, "/")
		if len(parts) > 0 {
			threadID := parts[len(parts)-1]
			if threadID != "" {
				c.invalidateAfterChatCreate(threadID)
				return threadID, nil
			}
		}
	}

	// Try parsing response body
	var createResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err == nil {
		if id, ok := createResp["id"].(string); ok && id != "" {
			c.invalidateAfterChatCreate(id)
			return id, nil
		}
	}

	return "", fmt.Errorf("chat created but could not determine chat ID")
}

func (c *Client) invalidateAfterChatCreate(chatID string) {
	// New chat created — invalidate conversations and resolve caches
	c.CacheInvalidate("convs", "resolve")
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
