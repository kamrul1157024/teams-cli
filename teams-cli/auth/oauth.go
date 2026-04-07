package auth

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	webview "github.com/webview/webview_go"
)

const (
	TeamsAppID         = "5e3ce6c0-2b1f-4285-8d4b-75ee78787346"
	SkypeResource      = "https://api.spaces.skype.com"
	ChatSvcAggResource = "https://chatsvcagg.teams.microsoft.com"
)

func getLoginURL(t TokenType, tenantID string) string {
	loginURL, _ := url.Parse("https://login.microsoftonline.com/" + tenantID + "/oauth2/authorize")
	q := loginURL.Query()
	state := uuid.New().String()

	switch t {
	case TokenTeams:
		q.Set("response_type", "id_token")
		q.Set("state", state)
	case TokenSkype:
		q.Set("response_type", "token")
		q.Set("state", state+"|"+SkypeResource)
		q.Set("resource", SkypeResource)
	case TokenChatSvcAgg:
		q.Set("response_type", "token")
		q.Set("state", state+"|"+ChatSvcAggResource)
		q.Set("resource", ChatSvcAggResource)
	}

	q.Set("client_id", TeamsAppID)
	q.Set("client-request-id", uuid.New().String())
	q.Set("redirect_uri", "https://teams.microsoft.com/go")
	q.Set("x-client-SKU", "Js")
	q.Set("x-client-Ver", "1.0.9")
	q.Set("nonce", uuid.New().String())

	loginURL.RawQuery = q.Encode()
	return loginURL.String()
}

type AuthResult struct {
	Email    string
	TenantID string
}

func RunOAuth() (*AuthResult, error) {
	w := webview.New(true)
	defer w.Destroy()

	w.SetTitle("Teams CLI - Login")
	w.SetSize(800, 600, webview.HintNone)

	gotTokens := map[TokenType]bool{
		TokenTeams:      false,
		TokenSkype:      false,
		TokenChatSvcAgg: false,
	}
	currentTenant := ""
	redirectCount := 0
	var result AuthResult
	var authErr error

	w.Bind("goHandleNavigation", func(currentURL string) {
		if !strings.HasPrefix(currentURL, "https://teams.microsoft.com/go#") {
			return
		}

		fragment := strings.SplitN(currentURL, "#", 2)
		if len(fragment) < 2 {
			return
		}

		params, _ := url.ParseQuery(fragment[1])
		token := params.Get("id_token")
		if token == "" {
			token = params.Get("access_token")
		}
		if token == "" {
			if params.Get("error") != "" {
				authErr = fmt.Errorf("OAuth error: %s - %s", params.Get("error"), params.Get("error_description"))
				w.Dispatch(func() { w.Terminate() })
			}
			return
		}

		redirectCount++
		if redirectCount > 6 {
			authErr = fmt.Errorf("too many redirects during authentication")
			w.Dispatch(func() { w.Terminate() })
			return
		}

		claims, err := DecodeJWTClaims(token)
		if err != nil {
			authErr = fmt.Errorf("invalid token received: %w", err)
			w.Dispatch(func() { w.Terminate() })
			return
		}

		aud, _ := claims["aud"].(string)
		tid, _ := claims["tid"].(string)

		if currentTenant == "" && tid != "" {
			currentTenant = tid
			result.TenantID = tid
		}

		if email, ok := claims["upn"].(string); ok && result.Email == "" {
			result.Email = email
		}

		if aud == TeamsAppID && !gotTokens[TokenTeams] {
			if err := SaveToken(token, TokenTeams); err != nil {
				authErr = fmt.Errorf("failed to save teams token: %w", err)
				w.Dispatch(func() { w.Terminate() })
				return
			}
			gotTokens[TokenTeams] = true
			tenant := currentTenant
			if tenant == "" {
				tenant = "common"
			}
			w.Dispatch(func() { w.Navigate(getLoginURL(TokenSkype, tenant)) })

		} else if aud == SkypeResource && !gotTokens[TokenSkype] {
			if err := SaveToken(token, TokenSkype); err != nil {
				authErr = fmt.Errorf("failed to save skype token: %w", err)
				w.Dispatch(func() { w.Terminate() })
				return
			}
			gotTokens[TokenSkype] = true
			w.Dispatch(func() { w.Navigate(getLoginURL(TokenChatSvcAgg, currentTenant)) })

		} else if aud == ChatSvcAggResource && !gotTokens[TokenChatSvcAgg] {
			if err := SaveToken(token, TokenChatSvcAgg); err != nil {
				authErr = fmt.Errorf("failed to save chatsvcagg token: %w", err)
				w.Dispatch(func() { w.Terminate() })
				return
			}
			gotTokens[TokenChatSvcAgg] = true
			w.Dispatch(func() { w.Terminate() })
		}
	})

	w.Init(`
		(function() {
			var lastUrl = '';
			setInterval(function() {
				var u = window.location.href;
				if (u !== lastUrl) {
					lastUrl = u;
					goHandleNavigation(u);
				}
			}, 100);
		})();
	`)

	w.Navigate(getLoginURL(TokenTeams, "common"))
	w.Run()

	if authErr != nil {
		return nil, authErr
	}

	if !gotTokens[TokenTeams] || !gotTokens[TokenSkype] || !gotTokens[TokenChatSvcAgg] {
		return nil, fmt.Errorf("authentication incomplete: not all tokens were acquired")
	}

	return &result, nil
}
