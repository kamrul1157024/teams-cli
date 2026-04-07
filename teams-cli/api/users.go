package api

import (
	"net/url"

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

	return &UserListItem{
		DisplayName: resp.Value.DisplayName,
		Email:       emailAddr,
		JobTitle:    resp.Value.JobTitle,
		Department:  resp.Value.Department,
		Mri:         resp.Value.Mri,
		Phone:       phone,
	}, nil
}

func (c *Client) GetMe() (*UserListItem, error) {
	email, err := auth.GetEmail()
	if err != nil {
		return nil, err
	}
	return c.GetUser(email)
}
