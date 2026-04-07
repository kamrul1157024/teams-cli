package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type AuthzTokens struct {
	SkypeToken string `json:"skypeToken"`
	ExpiresIn  int    `json:"expiresIn"`
}

type AuthzResponse struct {
	Tokens AuthzTokens `json:"tokens"`
	Region string      `json:"region"`
}

func RefreshSkypeToken() (string, error) {
	// Use the original Skype Spaces Bearer token for the authz exchange
	raw, err := LoadToken(TokenSkypeSpaces)
	if err != nil {
		// Fall back to skype token if skype-spaces doesn't exist (legacy)
		raw, err = LoadToken(TokenSkype)
		if err != nil {
			return "", fmt.Errorf("cannot load skype token for refresh: %w", err)
		}
	}

	req, err := http.NewRequest("POST", "https://teams.microsoft.com/api/authsvc/v1.0/authz", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+raw)
	req.Header.Set("ms-teams-authz-type", "TokenRefresh")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token refresh failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var authzResp AuthzResponse
	if err := json.NewDecoder(resp.Body).Decode(&authzResp); err != nil {
		return "", fmt.Errorf("cannot decode refresh response: %w", err)
	}

	if authzResp.Tokens.SkypeToken == "" {
		return "", fmt.Errorf("refresh response contained no token")
	}

	if err := SaveToken(authzResp.Tokens.SkypeToken, TokenSkype); err != nil {
		return "", fmt.Errorf("cannot save refreshed token: %w", err)
	}

	return authzResp.Tokens.SkypeToken, nil
}

func EnsureValidToken(t TokenType) (string, error) {
	info, err := GetTokenInfo(t)
	if err != nil {
		return "", err
	}

	if info.Valid && time.Until(info.ExpiresAt) > 5*time.Minute {
		return info.Raw, nil
	}

	// Token expired or expiring soon — try refresh
	if t == TokenSkype {
		return RefreshSkypeToken()
	}

	// SkypeSpaces uses the same underlying token, can't refresh independently
	if t == TokenSkypeSpaces {
		return "", fmt.Errorf("token %s expired (run 'teams-cli auth' to re-authenticate)", t)
	}

	return "", fmt.Errorf("token %s expired (run 'teams-cli auth' to re-authenticate)", t)
}
