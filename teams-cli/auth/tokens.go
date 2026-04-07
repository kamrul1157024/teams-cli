package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type TokenType string

const (
	TokenTeams       TokenType = "teams"
	TokenSkype       TokenType = "skype"
	TokenSkypeSpaces TokenType = "skype-spaces" // Original Bearer token for MiddleTier API
	TokenChatSvcAgg  TokenType = "chatsvcagg"
)

type TokenInfo struct {
	Type      TokenType `json:"type"`
	Raw       string    `json:"raw,omitempty"`
	Valid     bool      `json:"valid"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	ExpiresIn string    `json:"expires_in,omitempty"`
	Audience  string    `json:"audience,omitempty"`
	Subject   string    `json:"subject,omitempty"`
	Email     string    `json:"email,omitempty"`
	TenantID  string    `json:"tenant_id,omitempty"`
}

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "teams-cli"), nil
}

func TokenPath(t TokenType) (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, string(t)+".jwt"), nil
}

func SaveToken(token string, t TokenType) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}
	path := filepath.Join(dir, string(t)+".jwt")
	return os.WriteFile(path, []byte(token), 0600)
}

func LoadToken(t TokenType) (string, error) {
	path, err := TokenPath(t)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("token %s not found (run 'teams-cli auth' first): %w", t, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func DecodeJWTClaims(token string) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format: expected 3 parts, got %d", len(parts))
	}

	payload := parts[1]
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("cannot decode JWT payload: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, fmt.Errorf("cannot parse JWT claims: %w", err)
	}
	return claims, nil
}

func GetTokenInfo(t TokenType) (*TokenInfo, error) {
	info := &TokenInfo{Type: t}

	raw, err := LoadToken(t)
	if err != nil {
		return info, err
	}
	info.Raw = raw

	claims, err := DecodeJWTClaims(raw)
	if err != nil {
		return info, err
	}

	if exp, ok := claims["exp"].(float64); ok {
		info.ExpiresAt = time.Unix(int64(exp), 0)
		remaining := time.Until(info.ExpiresAt)
		if remaining > 0 {
			info.Valid = true
			info.ExpiresIn = remaining.Truncate(time.Second).String()
		}
	}

	if aud, ok := claims["aud"].(string); ok {
		info.Audience = aud
	}
	if sub, ok := claims["sub"].(string); ok {
		info.Subject = sub
	}
	if email, ok := claims["upn"].(string); ok {
		info.Email = email
	} else if email, ok := claims["email"].(string); ok {
		info.Email = email
	}
	if tid, ok := claims["tid"].(string); ok {
		info.TenantID = tid
	}

	return info, nil
}

func GetEmail() (string, error) {
	info, err := GetTokenInfo(TokenTeams)
	if err != nil {
		return "", err
	}
	if info.Email == "" {
		return "", fmt.Errorf("no email found in token claims")
	}
	return info.Email, nil
}

func GetTenantID() (string, error) {
	info, err := GetTokenInfo(TokenTeams)
	if err != nil {
		return "", err
	}
	if info.TenantID == "" {
		return "", fmt.Errorf("no tenant ID found in token claims")
	}
	return info.TenantID, nil
}
