# teams-cli Technical Specification

## Overview

`teams-cli` is a Unix-style command-line tool for Microsoft Teams. Single Go binary. Scriptable, pipe-friendly, composable with standard unix tools. Replaces the need for both `teams-token` (Electron auth app) and `teams-cli` (TUI-only reader) with one tool that can authenticate, read, search, and send messages.

## Goals

- **Single binary**: No Node.js, no Electron, no external dependencies
- **Scriptable**: JSON output by default, human-readable with flags
- **Composable**: Works with pipes, jq, grep, cron, shell scripts
- **Self-contained auth**: Built-in OAuth via native webview (macOS WebKit, Linux WebKitGTK, Windows WebView2)
- **Read + Write**: Not just a viewer — can send messages too

## Non-Goals

- TUI/interactive mode (the existing `fossteams/teams-cli` covers this)
- Full Teams feature parity (calls, meetings, file sharing)
- Bot/app framework

---

## Architecture

```
teams-cli/
├── main.go                 # Entry point
├── cmd/                    # Cobra command definitions
│   ├── root.go             # Root command, global flags
│   ├── auth.go             # OAuth login flow (webview)
│   ├── status.go           # Token health check
│   ├── me.go               # Current user info
│   ├── chats.go            # List/filter conversations
│   ├── messages.go         # Read/send/search messages
│   ├── teams.go            # List teams
│   ├── channels.go         # List channels in a team
│   └── users.go            # Search/lookup users
├── auth/                   # Authentication layer
│   ├── oauth.go            # Webview OAuth flow
│   ├── tokens.go           # Token read/write/validate
│   └── refresh.go          # Token refresh via authz endpoint
├── api/                    # Teams API client
│   ├── client.go           # HTTP client, auth headers
│   ├── conversations.go    # Chat/conversation endpoints
│   ├── messages.go         # Message read/send endpoints
│   ├── users.go            # User lookup endpoints
│   └── teams.go            # Teams/channels endpoints
├── output/                 # Output formatting
│   ├── json.go             # JSON output (default)
│   ├── table.go            # Table output
│   └── text.go             # Plain text output
└── go.mod
```

---

## Commands

### `teams-cli auth`

Launches a native webview window for Microsoft OAuth. Acquires three JWT tokens (Teams, Skype, ChatSvcAgg) and saves them to `~/.config/teams-cli/`.

**Implementation**: Port of the proven Go webview approach (tested and working). Uses `github.com/webview/webview_go` to open a WebKit window, navigates through Microsoft's OAuth flow, intercepts redirects via JS polling, extracts tokens from URL fragments, decodes JWT to determine audience, and chains through all three token acquisitions automatically.

**OAuth Flow**:
1. Navigate to `https://login.microsoftonline.com/common/oauth2/authorize` with `response_type=id_token`, `client_id=5e3ce6c0-2b1f-4285-8d4b-75ee78787346`, `redirect_uri=https://teams.microsoft.com/go`
2. User authenticates (password + MFA)
3. Redirect to `https://teams.microsoft.com/go#id_token=...` — extract Teams token (aud=`5e3ce6c0-...`)
4. Extract `tid` (tenant ID) from JWT claims
5. Navigate again with `response_type=token`, `resource=https://api.spaces.skype.com` — extract Skype token
6. Navigate again with `resource=https://chatsvcagg.teams.microsoft.com` — extract ChatSvcAgg token
7. Save all three to disk, close window

**Token Storage**:
```
~/.config/teams-cli/
├── teams.jwt
├── skype.jwt
└── chatsvcagg.jwt
```

**Flags**:
```
--force    Re-authenticate even if tokens exist and are valid
```

**Output**:
```
Authenticated as MD. Kamrul Hassan (md.kamrul.hassan@optimizely.com)
Tenant: 3ec00d79-021a-42d4-aac8-dcb35973dff2
Tokens saved to ~/.config/teams-cli/
```

---

### `teams-cli status`

Check token health, expiry, and connectivity.

**Implementation**: Read each JWT file, decode claims, check `exp` field against current time. Optionally make a test API call to verify tokens work.

**Output**:
```json
{
  "user": "md.kamrul.hassan@optimizely.com",
  "tenant_id": "3ec00d79-...",
  "tokens": {
    "teams":      { "valid": true, "expires_at": "2026-04-08T02:30:00Z", "expires_in": "1h23m" },
    "skype":      { "valid": true, "expires_at": "2026-04-08T02:35:00Z", "expires_in": "1h28m" },
    "chatsvcagg": { "valid": true, "expires_at": "2026-04-08T02:20:00Z", "expires_in": "1h13m" }
  }
}
```

**Flags**:
```
--check    Also verify tokens against API (not just expiry)
```

---

### `teams-cli me`

Show current user profile.

**API**: GET `https://teams.microsoft.com/api/mt/emea/beta/users/{email}/`
- Email extracted from JWT `upn` or `email` claim
- Auth: `Authorization: Bearer {skype-spaces-token}`

**Output**:
```json
{
  "display_name": "MD. Kamrul Hassan",
  "email": "md.kamrul.hassan@optimizely.com",
  "job_title": "...",
  "department": "...",
  "mri": "8:orgid:6e7e7990-c73f-4faa-bb35-87eadd9b49d5",
  "tenant": "optimizely.com"
}
```

---

### `teams-cli chats list`

List all conversations.

**API**: GET `https://teams.microsoft.com/api/csa/api/v1/teams/users/me?isPrefetch=false&enableMembershipSummary=true`
- Auth: `Authorization: Bearer {chatsvcagg-token}`
- Response: `ConversationResponse` containing `Chats[]` array

**Flags**:
```
--type <1:1|group|meeting>   Filter by chat type
--unread                     Only show unread conversations
--limit <n>                  Limit results (default: all)
--format <json|table|text>   Output format (default: json)
```

**Output (JSON)**:
```json
[
  {
    "id": "19:abc123@thread.v2",
    "title": "John Doe",
    "type": "1:1",
    "is_read": true,
    "last_message": {
      "from": "John Doe",
      "content": "sounds good",
      "time": "2026-04-07T15:30:00Z"
    },
    "members": ["md.kamrul.hassan@optimizely.com", "john.doe@optimizely.com"]
  }
]
```

**Output (table)**:
```
ID                          TYPE   TITLE          LAST MESSAGE         TIME
19:abc123@thread.v2         1:1    John Doe       sounds good          2h ago
19:def456@thread.v2         group  Project Alpha  meeting at 3?        5h ago
```

---

### `teams-cli messages list <chat-id>`

List messages in a conversation.

**API**: GET `https://emea.ng.msg.teams.microsoft.com/v1/users/ME/conversations/{chat-id}/messages`
- Query params: `view=msnp24Equivalent|supportsMessageProperties`, `pageSize={limit}`, `startTime=1`
- Auth: `Authentication: skypetoken={skype-token}`

**Flags**:
```
-n, --limit <count>          Number of messages (default: 50)
--since <duration|date>      Messages since time (e.g. "2h", "2026-04-07")
--from <name|email>          Filter by sender
--format <json|table|text>   Output format
```

**Output (JSON)**:
```json
[
  {
    "id": "1712345678901",
    "from": "John Doe",
    "content": "Hey, can you review my PR?",
    "time": "2026-04-07T15:30:00Z",
    "type": "Message"
  }
]
```

**Output (text)** — optimized for reading in terminal:
```
[2026-04-07 15:30] John Doe: Hey, can you review my PR?
[2026-04-07 15:32] MD. Kamrul Hassan: Sure, looking now
[2026-04-07 15:45] John Doe: Thanks!
```

---

### `teams-cli messages send <chat-id> [message]`

Send a message to a conversation.

**API**: POST `https://emea.ng.msg.teams.microsoft.com/v1/users/ME/conversations/{chat-id}/messages`
- Auth: `Authentication: skypetoken={skype-token}`
- Content-Type: `application/json`

**Request Body** (reverse-engineered from Teams web client):
```json
{
  "content": "<p>Hello from CLI</p>",
  "messagetype": "RichText/Html",
  "contenttype": "text",
  "clientmessageid": "<unique-id>",
  "imdisplayname": "MD. Kamrul Hassan",
  "properties": {
    "importance": "",
    "subject": ""
  }
}
```

**Flags**:
```
--to <email>     Send to user by email (resolves to chat ID automatically)
--html           Send as HTML (default: plain text wrapped in <p> tags)
```

**Message from stdin**:
```bash
echo "Build passed" | teams-cli messages send 19:abc123@thread.v2
cat report.txt | teams-cli messages send --to john@company.com
```

**Output**:
```json
{
  "status": "sent",
  "message_id": "1712345678901",
  "chat_id": "19:abc123@thread.v2",
  "time": "2026-04-08T01:00:00Z"
}
```

**User-to-chat resolution** (`--to` flag):
1. Fetch all conversations via `chats list`
2. Find 1:1 chat where members include the target email
3. If no existing chat found: create new conversation via POST to Teams API (stretch goal)

---

### `teams-cli messages search <query>`

Search messages across conversations.

**Implementation**: Client-side search (Teams API doesn't expose a search endpoint via these tokens). Fetches recent messages from all/specified conversations and filters locally.

**Flags**:
```
--chat <chat-id>             Search within specific chat
--from <name|email>          Filter by sender
--since <duration|date>      Time range
--limit <n>                  Max results (default: 20)
--format <json|table|text>   Output format
```

**Output**:
```json
[
  {
    "chat_id": "19:abc123@thread.v2",
    "chat_title": "John Doe",
    "message_id": "1712345678901",
    "from": "John Doe",
    "content": "can you deploy the fix?",
    "time": "2026-04-07T15:30:00Z",
    "match": "deploy"
  }
]
```

---

### `teams-cli teams list`

List all joined teams.

**API**: Same `GetConversations()` endpoint — extracts `Teams[]` from response.

**Output**:
```json
[
  {
    "id": "19:team-id@thread.tacv2",
    "name": "Engineering",
    "is_favorite": true,
    "is_archived": false,
    "channel_count": 12,
    "member_count": 45
  }
]
```

---

### `teams-cli channels list <team-id>`

List channels in a team.

**API**: Same `GetConversations()` endpoint — finds team by ID, extracts `Channels[]`.

**Output**:
```json
[
  {
    "id": "19:channel-id@thread.tacv2",
    "name": "General",
    "is_general": true,
    "is_favorite": true,
    "is_pinned": false,
    "team": "Engineering"
  }
]
```

---

### `teams-cli users search <query>`

Look up users by name or email.

**API**: GET `https://teams.microsoft.com/api/mt/emea/beta/users/{email}/`
- Auth: `Authorization: Bearer {skype-spaces-token}`

**Output**:
```json
{
  "display_name": "John Doe",
  "email": "john.doe@optimizely.com",
  "job_title": "Software Engineer",
  "department": "Engineering",
  "mri": "8:orgid:...",
  "phone": "+1234567890"
}
```

---

## API Layer

### Authentication

Three tokens, three purposes:

| Token | File | Used For | Header |
|-------|------|----------|--------|
| Teams | `teams.jwt` | Identity (JWT claims: email, tenant) | Not used for API calls directly |
| Skype | `skype.jwt` | Messages API, Skype Spaces API | `Authentication: skypetoken={raw}` or `Authorization: Bearer {raw}` |
| ChatSvcAgg | `chatsvcagg.jwt` | Chat Service Aggregator (conversations) | `Authorization: Bearer {raw}` |

### Token Refresh

The Skype API token can be refreshed via:
- POST `https://teams.microsoft.com/api/authsvc/v1.0/authz`
- Header: `ms-teams-authz-type: TokenRefresh`
- Header: `Authorization: Bearer {skype-spaces-token}`
- Returns new `skypetoken` + region/partition info

Auto-refresh logic:
1. Before each API call, check token expiry
2. If < 5 minutes remaining, attempt refresh
3. If refresh fails, prompt user to run `teams-cli auth`

### Endpoints Summary

| Endpoint | Method | Base URL | Auth Token |
|----------|--------|----------|------------|
| Get conversations | GET | `https://teams.microsoft.com/api/csa/api/v1/teams/users/me` | ChatSvcAgg Bearer |
| Get messages | GET | `https://emea.ng.msg.teams.microsoft.com/v1/users/ME/conversations/{id}/messages` | Skype skypetoken |
| Send message | POST | `https://emea.ng.msg.teams.microsoft.com/v1/users/ME/conversations/{id}/messages` | Skype skypetoken |
| Get user | GET | `https://teams.microsoft.com/api/mt/emea/beta/users/{email}/` | Skype Spaces Bearer |
| Get tenants | GET | `https://teams.microsoft.com/api/mt/emea/beta/users/tenants` | Skype Spaces Bearer |
| Fetch profiles | POST | `https://teams.microsoft.com/api/mt/emea/beta/users/fetchShortProfile` | Skype Spaces Bearer |
| Get pinned channels | GET | `https://teams.microsoft.com/api/csa/api/v1/teams/users/me/pinnedChannels` | ChatSvcAgg Bearer |
| Refresh token | POST | `https://teams.microsoft.com/api/authsvc/v1.0/authz` | Skype Spaces Bearer |

### Error Handling

- **401/403**: Token expired or invalid → suggest `teams-cli auth`
- **429**: Rate limited → retry with exponential backoff (250ms, 500ms, 1s, max 3 attempts)
- **5xx**: Server error → retry with backoff
- **Network errors**: Clear error message with suggestion to check connectivity

---

## Output Formatting

### Global Flags

```
--format <json|table|text>   Output format (default: json)
--pretty                     Pretty-print JSON
--no-color                   Disable color output
--quiet                      Suppress non-essential output
```

### JSON (default)

Machine-readable. All commands output valid JSON to stdout. Errors go to stderr.

```bash
teams-cli chats list | jq '.[0].title'
teams-cli messages list 19:abc@thread.v2 | jq '.[] | select(.from == "John")'
```

### Table

Human-readable columnar output.

```bash
teams-cli chats list --format table
```

### Text

Minimal output, one item per line. Good for piping.

```bash
teams-cli chats list --format text
# Output: just chat IDs, one per line

teams-cli messages list <id> --format text
# Output: [time] sender: message
```

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/webview/webview_go` | OAuth webview (used only by `init`) |
| `github.com/google/uuid` | Client message IDs, OAuth nonces |
| `github.com/golang-jwt/jwt/v5` | JWT parsing and validation |

Intentionally minimal. No TUI libraries, no ORM, no config frameworks.

---

## Configuration

### Config File (optional)

`~/.config/teams-cli/config.yaml`:
```yaml
# Default output format
format: json

# Region (affects API base URLs)
region: emea

# Default message limit
message_limit: 50

# Token directory (override)
token_dir: ~/.config/teams-cli
```

### Environment Variables

```
TEAMS_CLI_FORMAT=json|table|text
TEAMS_CLI_TOKEN_DIR=~/.config/teams-cli
TEAMS_CLI_REGION=emea
```

Priority: CLI flags > env vars > config file > defaults.

---

## Implementation Phases

### Phase 1: Foundation
- [ ] Project scaffold (go.mod, cobra, directory structure)
- [ ] Auth: `init` command with webview OAuth
- [ ] Auth: Token read/write/validate
- [ ] `status` command
- [ ] `me` command
- [ ] Output formatters (JSON, table, text)

### Phase 2: Read
- [ ] `chats list` — list conversations with filtering
- [ ] `messages list` — read messages from a conversation
- [ ] `teams list` — list joined teams
- [ ] `channels list` — list channels in a team
- [ ] `users search` — look up users

### Phase 3: Write
- [ ] `messages send` — send to chat by ID
- [ ] `messages send --to` — send by email (resolve to chat ID)
- [ ] Stdin pipe support for message body

### Phase 4: Search & Polish
- [ ] `messages search` — client-side message search
- [ ] Token auto-refresh
- [ ] Shell completions (bash, zsh, fish)
- [ ] `--since` / `--from` filters on messages
- [ ] Config file support

---

## Example Workflows

### Morning catch-up
```bash
# Check unread chats
teams-cli chats list --unread --format table

# Read latest from a specific chat
teams-cli messages list 19:abc@thread.v2 -n 10 --format text
```

### CI/CD notification
```bash
# Send build result to a channel
echo "Deploy v2.3.1 complete" | teams-cli messages send 19:channel@thread.v2
```

### Search for a discussion
```bash
# Find where someone mentioned a topic
teams-cli messages search "database migration" --since 7d --format table
```

### Scripting
```bash
# Get all unread chat IDs
teams-cli chats list --unread | jq -r '.[].id'

# Read last message from each unread chat
for id in $(teams-cli chats list --unread | jq -r '.[].id'); do
  teams-cli messages list "$id" -n 1
done
```

### Quick reply
```bash
# Send a message to a colleague
teams-cli messages send --to john.doe@company.com "PR approved, merging now"
```
