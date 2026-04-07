# teams-cli

A Unix-style CLI for Microsoft Teams. Read, send, and search messages from your terminal. Scriptable, pipe-friendly, and integrates with Claude Code as a skill.

## Prerequisites

- **Go 1.26.1+**
- **macOS** (uses WebKit webview for OAuth)
- A Microsoft Teams account

## Install

```bash
git clone https://github.com/kamrul1157024/teams-cli.git
cd teams-cli
make install
```

This builds the binary and copies it to `/usr/local/bin/teams-cli`.

To install to a custom location:

```bash
make install INSTALL_DIR=~/bin
```

## Authentication

teams-cli uses native OAuth via a webview window. Run once to authenticate:

```bash
teams-cli auth
```

This opens a browser-like window, you log in with your Microsoft account, and it acquires 3 tokens (Teams, Skype, ChatSvcAgg). Tokens are saved to `~/.config/teams-cli/` and last ~1 hour.

To check token status:

```bash
teams-cli status --format table
```

To force re-authentication:

```bash
teams-cli auth --force
```

## Usage

### Chats

```bash
# List all chats
teams-cli chats list --format table

# List unread chats
teams-cli chats list --unread --format table

# List 1:1 chats only
teams-cli chats list --type "1:1" --format table
```

### Messages

```bash
# Read last 20 messages from a chat
teams-cli messages list <chat-id> -n 20 --format table

# Send a message
teams-cli messages send <chat-id> "Hello!"

# Send by email (resolves to 1:1 chat)
teams-cli messages send --to user@company.com "Hello!"

# Pipe from stdin
echo "Build passed" | teams-cli messages send <chat-id>

# Search messages
teams-cli messages search "deploy" --format table
```

### Teams & Channels

```bash
# List your teams
teams-cli teams list --format table

# List channels in a team
teams-cli channels list <team-id> --format table
```

### Users

```bash
# Look up a user
teams-cli users search user@company.com --format table
```

### Output Formats

All commands support `--format json|table|text`. Default is `json`. Add `--pretty` for formatted JSON.

```bash
# JSON (default, good for piping)
teams-cli chats list --format json | jq '.[] | .title'

# Table (human-readable)
teams-cli chats list --format table

# Text (minimal, one ID per line)
teams-cli chats list --format text
```

## Claude Code Skill

teams-cli includes a Claude Code skill that turns Claude into an interactive Teams assistant. It can check your messages, draft replies matching your communication style, and manage contact relationship profiles.

### Install from Marketplace

If teams-cli is published as a Claude Code plugin:

```bash
claude plugin add kamrul1157024/teams-cli
```

This installs the skill and makes `/teams` available in Claude Code.

### Install Manually

```bash
make skill-install
```

Or manually:

```bash
mkdir -p ~/.claude/skills/teams
cp skills/teams/SKILL.md ~/.claude/skills/teams/SKILL.md
```

### Using the Skill

In any Claude Code session, type:

```
/teams
```

Claude will:
1. Check your auth tokens are valid
2. If first time, build your communication persona by analyzing your chat history
3. Ask what you want to do — check unreads, send messages, search, etc.

### What the Skill Does

- **Check unread messages** — summarizes what's new across all chats
- **Send messages** — drafts replies matching your tone, always confirms before sending
- **Search** — finds messages across conversations
- **Persona management** — learns your communication style from chat history
- **Contact profiles** — tracks your relationship and tone with each contact
- **Safety rules** — never sends without approval, avoids sensitive topics, respects cultural differences

### Persona & Context System

The skill stores communication context at `~/.teams-agent/`:

```
~/.teams-agent/
├── persona.md         # Your communication style
├── safety-rules.md    # Hard safety constraints
├── config.yaml        # Settings
├── contacts/          # Per-person relationship profiles
├── groups/            # Group chat dynamics
└── patterns/          # Response templates by topic
```

This is built automatically the first time you use `/teams`. It analyzes your chat history to extract your actual phrases, tone, and relationship dynamics — so replies sound like you, not a bot.

## Development

```bash
make build          # Build binary to bin/
make install        # Build and install to /usr/local/bin
make test           # Run tests
make fmt            # Format code
make tidy           # Tidy dependencies
make clean          # Remove build artifacts
make help           # Show all targets
```

## Architecture

```
teams-cli/
├── main.go              # Entry point
├── cmd/                 # Cobra CLI commands
│   ├── root.go          # Global flags (--format, --pretty, --quiet)
│   ├── auth.go          # teams-cli auth
│   ├── status.go        # teams-cli status
│   ├── me.go            # teams-cli me
│   ├── chats.go         # teams-cli chats list
│   ├── messages.go      # teams-cli messages list/send/search
│   ├── teams.go         # teams-cli teams list
│   ├── channels.go      # teams-cli channels list
│   └── users.go         # teams-cli users search
├── auth/                # Authentication
│   ├── oauth.go         # Webview OAuth flow (3 tokens)
│   ├── tokens.go        # Token storage and JWT decoding
│   └── refresh.go       # Token refresh via authz endpoint
├── api/                 # Teams API client
│   ├── client.go        # HTTP client with retry and auth headers
│   ├── conversations.go # Chats, teams, channels
│   ├── messages.go      # Read/send/search messages
│   └── users.go         # User lookup
└── output/              # Output formatters
    ├── json.go
    ├── table.go
    └── text.go
```

## License

MIT
