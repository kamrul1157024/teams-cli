# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

teams-cli is a Unix-style CLI for Microsoft Teams. It provides scriptable, pipe-friendly commands for reading, sending, and searching messages from the terminal. Built as a single Go binary with native OAuth via webview.

Module: `github.com/kamrul1157024/teams-cli/teams-cli`
Go version: 1.26.1+

## Build & Development Commands

```bash
cd teams-cli
go build -o teams-cli .              # Build binary
go build -o /usr/local/bin/teams-cli .  # Build and install
go test ./...                         # Run tests
go vet ./...                          # Vet code
```

## Architecture

### Authentication (`auth/`)

OAuth flow via Go webview (`github.com/webview/webview_go`). Acquires 3 tokens:
- **Teams** — id_token (audience: Teams app ID)
- **Skype** — access_token for `api.spaces.skype.com`, exchanged via `/authsvc/v1.0/authz` for a skypeToken
- **ChatSvcAgg** — access_token for `chatsvcagg.teams.microsoft.com`

Token files stored at `~/.config/teams-cli/`. The Skype Spaces Bearer token must be exchanged via the authz endpoint to get the actual skypeToken used by the messages API.

Key files:
- `oauth.go` — Webview OAuth flow, silent SSO for 2nd/3rd tokens
- `tokens.go` — Token read/write/validate, JWT claims decoding
- `refresh.go` — Token refresh via authz endpoint, `EnsureValidToken()`

### API Client (`api/`)

HTTP client with retry (3 attempts, exponential backoff) and auth header injection.

- `client.go` — Base HTTP client. Uses `skypetoken=<token>` for Messages API, `Bearer <token>` for others
- `conversations.go` — GetConversations(), ListChats(), ListTeams(), ListChannels()
- `messages.go` — GetMessages(), SendMessage(), FindChatByEmail(), SearchMessages()
- `users.go` — GetUser(), GetMe()

Base URLs:
- ChatSvcAgg: `https://teams.microsoft.com/api/csa/api/v1/`
- Messages: `https://emea.ng.msg.teams.microsoft.com/v1/`
- MiddleTier: `https://teams.microsoft.com/api/mt/emea/beta/`

### CLI Commands (`cmd/`)

Cobra-based. All commands support `--format json|table|text` and `--pretty`.

- `auth` — OAuth login (with `--force` to re-auth)
- `status` — Token health check
- `me` — Current user profile
- `chats list` — List conversations (with `--type`, `--unread`, `--limit`)
- `messages list` — Read messages (with `-n`, `--from`)
- `messages send` — Send message (by chat ID or `--to email`, supports stdin pipe)
- `messages search` — Search across conversations
- `teams list` — List joined teams
- `channels list` — List channels in a team
- `users search` — Look up user by email

### Output (`output/`)

- `json.go` — JSON output with optional pretty-print
- `table.go` — Aligned table output
- `text.go` — Plain text output

### Claude Code Integration

- `.claude-plugin/plugin.json` — Plugin metadata for marketplace
- `skills/teams/SKILL.md` — Interactive Teams assistant skill

## Code Conventions

- Error wrapping: `fmt.Errorf("context: %w", err)`
- Output format switch: `switch outputFormat { case "text": ... case "table": ... default: output.JSON(...) }`
- Token types: `auth.TokenTeams`, `auth.TokenSkype`, `auth.TokenChatSvcAgg`
