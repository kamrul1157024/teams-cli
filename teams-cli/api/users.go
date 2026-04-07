package api

import (
	"net/url"
	"strings"

	"github.com/kamrul1157024/teams-cli/teams-cli/auth"
)

type FeatureSettings struct {
	IsPrivateChatEnabled bool `json:"isPrivateChatEnabled"`
	CoExistenceMode      string `json:"coExistenceMode"`
}

type User struct {
	DisplayName       string          `json:"displayName"`
	Email             string          `json:"email"`
	Mail              string          `json:"mail"`
	GivenName         string          `json:"givenName"`
	Surname           string          `json:"surname"`
	JobTitle          string          `json:"jobTitle"`
	Department        string          `json:"department"`
	Mri               string          `json:"mri"`
	ObjectId          string          `json:"objectId"`
	UserPrincipalName string          `json:"userPrincipalName"`
	TenantName        string          `json:"tenantName"`
	Mobile            string          `json:"mobile"`
	TelephoneNumber   string          `json:"telephoneNumber"`
	UserType          string          `json:"userType"`
	FeatureSettings   FeatureSettings `json:"featureSettings"`
}

type UserResponse struct {
	Value User   `json:"value"`
	Type  string `json:"type"`
}

type UserListItem struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	JobTitle    string `json:"job_title,omitempty"`
	Department  string `json:"department,omitempty"`
	Mri         string `json:"mri,omitempty"`
	Phone       string `json:"phone,omitempty"`
}

func (c *Client) GetUser(email string) (*UserListItem, error) {
	key := cacheKey("user", strings.ToLower(email))
	var cached UserListItem
	if c.cacheGet(key, &cached) {
		return &cached, nil
	}

	u, _ := url.Parse(MiddleTierBase + "users/" + url.PathEscape(email) + "/")
	q := u.Query()
	q.Set("throwIfNotFound", "false")
	q.Set("isMailAddress", "true")
	q.Set("enableGuest", "true")
	q.Set("includeIBBarredUsers", "true")
	q.Set("skypeTeamsInfo", "true")
	u.RawQuery = q.Encode()

	var resp UserResponse
	if err := c.getJSON(u.String(), auth.TokenSkypeSpaces, &resp); err != nil {
		return nil, err
	}

	emailAddr := resp.Value.Email
	if emailAddr == "" {
		emailAddr = resp.Value.Mail
	}
	if emailAddr == "" {
		emailAddr = resp.Value.UserPrincipalName
	}

	phone := resp.Value.Mobile
	if phone == "" {
		phone = resp.Value.TelephoneNumber
	}

	result := &UserListItem{
		DisplayName: resp.Value.DisplayName,
		Email:       emailAddr,
		JobTitle:    resp.Value.JobTitle,
		Department:  resp.Value.Department,
		Mri:         resp.Value.Mri,
		Phone:       phone,
	}

	c.cacheSet(key, result, TTLUser)
	return result, nil
}

func (c *Client) GetMe() (*UserListItem, error) {
	key := cacheKey("me")
	var cached UserListItem
	if c.cacheGet(key, &cached) {
		return &cached, nil
	}

	email, err := auth.GetEmail()
	if err != nil {
		return nil, err
	}
	result, err := c.GetUser(email)
	if err != nil {
		return nil, err
	}

	c.cacheSet(key, result, TTLMe)
	return result, nil
}

// SearchUsersByName searches for users by display name across chat member lists.
// The MiddleTier API only supports exact email lookup, so name search works by
// scanning known chat members.
func (c *Client) SearchUsersByName(name string) ([]UserListItem, error) {
	convs, err := c.GetConversations()
	if err != nil {
		return nil, err
	}

	name = strings.ToLower(name)
	seen := map[string]bool{}
	var results []UserListItem

	for _, chat := range convs.Chats {
		for _, m := range chat.Members {
			mri := m.Mri
			if seen[mri] {
				continue
			}
			friendlyName := strings.ToLower(m.FriendlyName)
			if friendlyName != "" && strings.Contains(friendlyName, name) {
				seen[mri] = true
				results = append(results, UserListItem{
					DisplayName: m.FriendlyName,
					Mri:         mri,
				})
			}
		}
	}

	// Try to enrich with email/details via user lookup for top results
	for i := range results {
		if i >= 5 {
			break
		}
		// Extract object ID from MRI (8:orgid:<uuid>)
		if parts := strings.SplitN(results[i].Mri, ":", 3); len(parts) == 3 {
			// Can't look up by MRI directly, leave as-is
		}
	}

	return results, nil
}

// SearchUsers tries email lookup first, falls back to name search
func (c *Client) SearchUsers(query string) ([]UserListItem, error) {
	// Try exact email lookup first
	if strings.Contains(query, "@") {
		user, err := c.GetUser(query)
		if err == nil {
			return []UserListItem{*user}, nil
		}
	}

	// Fall back to name search across chat members
	return c.SearchUsersByName(query)
}

// ResolveMRIs resolves multiple MRI strings to user info using chat member data
func (c *Client) ResolveMRIs(mris []string) ([]UserListItem, error) {
	convs, err := c.GetConversations()
	if err != nil {
		return nil, err
	}

	// Build MRI -> name map from all chat members
	nameMap := map[string]string{}
	for _, chat := range convs.Chats {
		for _, m := range chat.Members {
			if m.FriendlyName != "" {
				nameMap[m.Mri] = m.FriendlyName
			}
		}
	}

	var results []UserListItem
	for _, mri := range mris {
		name := nameMap[mri]
		if name == "" {
			name = mri
		}
		results = append(results, UserListItem{
			DisplayName: name,
			Mri:         mri,
		})
	}
	return results, nil
}
