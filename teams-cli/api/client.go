package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kamrul1157024/teams-cli/teams-cli/auth"
)

const (
	ChatSvcAggBase = "https://teams.microsoft.com/api/csa/api/v1/"
	MessagesBase   = "https://emea.ng.msg.teams.microsoft.com/v1/"
	MiddleTierBase = "https://teams.microsoft.com/api/mt/emea/beta/"
)

type Client struct {
	http     *http.Client
	cache    CacheConfig
	reauthed bool
}

func NewClient() *Client {
	return &Client{
		http:  &http.Client{Timeout: 60 * time.Second},
		cache: CacheConfig{Enabled: true},
	}
}

func NewClientWithCache(cfg CacheConfig) *Client {
	return &Client{
		http:  &http.Client{Timeout: 60 * time.Second},
		cache: cfg,
	}
}

func (c *Client) doRequest(method, url string, body io.Reader, tokenType auth.TokenType) (*http.Response, error) {
	token, err := auth.EnsureValidToken(tokenType)
	if err != nil {
		if c.reauthed {
			return nil, err
		}
		if authErr := auth.Reauth(); authErr != nil {
			return nil, fmt.Errorf("authentication failed: %w", authErr)
		}
		c.reauthed = true
		return c.doRequest(method, url, body, tokenType)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if tokenType == auth.TokenSkype {
		// The skype token is an exchanged skypetoken, always use skypetoken= format
		req.Header.Set("Authentication", "skypetoken="+token)
	} else {
		// Bearer token for ChatSvcAgg, SkypeSpaces (MiddleTier), etc.
		req.Header.Set("Authorization", "Bearer "+token)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	var resp *http.Response
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err = c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			resp.Body.Close()
			backoff := time.Duration(250<<attempt) * time.Millisecond
			if backoff > time.Second {
				backoff = time.Second
			}
			time.Sleep(backoff)
			continue
		}
		break
	}

	if (resp.StatusCode == 401 || resp.StatusCode == 403) && !c.reauthed {
		resp.Body.Close()
		if err := auth.Reauth(); err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
		c.reauthed = true
		return c.doRequest(method, url, body, tokenType)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

func (c *Client) getJSON(url string, tokenType auth.TokenType, result any) error {
	resp, err := c.doRequest("GET", url, nil, tokenType)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(result)
}

func (c *Client) putRequest(url string, tokenType auth.TokenType, body io.Reader) error {
	resp, err := c.doRequest("PUT", url, body, tokenType)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) postJSON(url string, tokenType auth.TokenType, body io.Reader, result any) error {
	resp, err := c.doRequest("POST", url, body, tokenType)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}
