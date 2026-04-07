package auth

import (
	"fmt"
	"net/url"
	"strings"
	"time"

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
		q.Set("prompt", "none") // Silent SSO — user already authenticated
	case TokenChatSvcAgg:
		q.Set("response_type", "token")
		q.Set("state", state+"|"+ChatSvcAggResource)
		q.Set("resource", ChatSvcAggResource)
		q.Set("prompt", "none") // Silent SSO — user already authenticated
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

	// navigateNext dispatches the next token URL with a small delay
	// to allow the Init JS to load on the new page
	navigateNext := func(nextURL string) {
		w.Dispatch(func() {
			// Navigate to a blank page first to reset, then to the target
			// This ensures Init JS is injected before the redirect happens
			w.Navigate("about:blank")
			go func() {
				time.Sleep(300 * time.Millisecond)
				w.Dispatch(func() {
					w.Navigate(nextURL)
				})
			}()
		})
	}

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
			fmt.Println("  Got teams token")
			tenant := currentTenant
			if tenant == "" {
				tenant = "common"
			}
			navigateNext(getLoginURL(TokenSkype, tenant))

		} else if aud == SkypeResource && !gotTokens[TokenSkype] {
			// Save the Skype Spaces Bearer token first
			if err := SaveToken(token, TokenSkype); err != nil {
				authErr = fmt.Errorf("failed to save skype token: %w", err)
				w.Dispatch(func() { w.Terminate() })
				return
			}
			// Exchange for the real skypeToken via authz endpoint
			// The messages API needs this token, not the Bearer token
			_, refreshErr := RefreshSkypeToken()
			if refreshErr != nil {
				fmt.Printf("  Warning: skype token exchange failed: %v\n", refreshErr)
				// Continue anyway — the Bearer token is saved and can be exchanged later
			} else {
				fmt.Println("  Got skype token (exchanged via authz)")
			}
			gotTokens[TokenSkype] = true
			navigateNext(getLoginURL(TokenChatSvcAgg, currentTenant))

		} else if aud == ChatSvcAggResource && !gotTokens[TokenChatSvcAgg] {
			if err := SaveToken(token, TokenChatSvcAgg); err != nil {
				authErr = fmt.Errorf("failed to save chatsvcagg token: %w", err)
				w.Dispatch(func() { w.Terminate() })
				return
			}
			gotTokens[TokenChatSvcAgg] = true
			fmt.Println("  Got chatsvcagg token")
			w.Dispatch(func() {
				w.SetTitle("Teams CLI - Login Complete")
				w.Navigate("about:blank")
				w.Eval(`document.body.innerHTML = '<div style="display:flex;align-items:center;justify-content:center;height:100vh;font-family:system-ui;font-size:24px;color:#333;">Authentication complete. Closing...</div>';`)
				go func() {
					time.Sleep(500 * time.Millisecond)
					w.Dispatch(func() { w.Terminate() })
				}()
			})
		}
	})

	// Init JS runs on every new page navigation in webview
	w.Init(`
		(function() {
			var lastUrl = '';
			function check() {
				var u = window.location.href;
				if (u !== lastUrl) {
					lastUrl = u;
					try { goHandleNavigation(u); } catch(e) {}
				}
			}
			// Poll frequently to catch fast SSO redirects
			setInterval(check, 30);
			// Also listen for hash changes and history API
			window.addEventListener('hashchange', check);
			window.addEventListener('popstate', check);
			// Check immediately on load
			check();
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
