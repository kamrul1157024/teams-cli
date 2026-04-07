---
name: teams
description: "Interactive Microsoft Teams assistant — read, reply, search messages, manage contacts and persona context using teams-cli."
---

# Teams Skill

Use `teams-cli` (installed at /usr/local/bin/teams-cli) to interact with Microsoft Teams.
This skill lets you check messages, draft replies, search conversations, and manage
relationship context for persona-aware communication.

All outgoing messages MUST be shown to the user for confirmation before sending.

---

## teams-cli Command Reference

### Authentication

```bash
# Authenticate (launches webview OAuth flow, saves tokens to ~/.config/teams-cli/)
teams-cli auth

# Check token health
teams-cli status --format json

# Show current user
teams-cli me --format json
```

### Chats

```bash
# List all chats
teams-cli chats list --format json

# List unread chats only
teams-cli chats list --unread --format json

# List 1:1 chats only
teams-cli chats list --type "1:1" --format json

# List group chats only
teams-cli chats list --type group --format json

# Limit results
teams-cli chats list --limit 10 --format json
```

### Messages

```bash
# Read messages from a chat (default 50)
teams-cli messages list <chat-id> --format json

# Read last N messages
teams-cli messages list <chat-id> -n 20 --format json

# Filter by sender
teams-cli messages list <chat-id> --from "alice" --format json

# Send a message by chat ID
teams-cli messages send <chat-id> "message text"

# Send a message by email (resolves to 1:1 chat)
teams-cli messages send --to user@company.com "message text"

# Search messages across chats
teams-cli messages search "search query" --format json

# Search within a specific chat
teams-cli messages search "query" --chat <chat-id> --format json

# Search with limit
teams-cli messages search "query" --limit 10 --format json
```

### Teams & Channels

```bash
# List joined teams
teams-cli teams list --format json

# List channels in a team
teams-cli channels list <team-id> --format json
```

### Users

```bash
# Look up a user by email
teams-cli users search user@company.com --format json
```

### Output Formats

All commands support `--format` with values: `json` (default), `table`, `text`.
Use `--pretty` for pretty-printed JSON.

---

## How to Use This Skill

When the user invokes `/teams`, start by asking what they want to do. Common tasks:

### Check Unread Messages

1. Run `teams-cli chats list --unread --format json`
2. For each unread chat, run `teams-cli messages list <chat-id> -n 10 --format json`
3. Summarize all unread messages grouped by chat/person
4. Highlight anything that looks urgent or needs a response
5. Offer to draft replies

### Read a Specific Chat

1. If user gives a chat ID, run `teams-cli messages list <chat-id> -n 30 --format json`
2. If user gives a name/email, first find the chat:
   - Run `teams-cli chats list --format json`
   - Match by title or member name
3. Present messages conversationally, not as raw JSON

### Send a Message

1. If user provides the message, show it back for confirmation
2. If user describes what to say, draft the message
3. ALWAYS show: `To: [recipient] | Message: "[exact text]"` and ask for confirmation
4. Only after user says yes/send/confirm, run the send command
5. Never send without explicit approval

### Search Messages

1. Run `teams-cli messages search "query" --format json`
2. Summarize results grouped by chat
3. Offer to read full context of any match

### Draft a Reply

1. Read recent messages for context: `teams-cli messages list <chat-id> -n 20 --format json`
2. If persona/contact context exists at `~/.teams-agent/`, load it (see Context System below)
3. Draft reply matching the user's style and relationship with the recipient
4. Show draft and wait for confirmation

---

## Context System

The agent context lives at `~/.teams-agent/`. This directory stores the user's communication
persona, contact relationships, group dynamics, and response patterns. The skill should read
these files when drafting replies and help the user build them up over time.

### Directory Structure

```
~/.teams-agent/
├── config.yaml              # Settings
├── persona.md               # User's communication style
├── safety-rules.md          # Hard safety constraints
├── contacts/                # Per-person relationship profiles
│   └── <name>.md
├── groups/                  # Group chat dynamics
│   └── <group-name>.md
└── patterns/                # Response templates by topic
    └── <topic>.md
```

### Setting Up Context (First Time)

If `~/.teams-agent/` does not exist, offer to set it up. Walk the user through each step:

#### Step 1: Create Directory Structure

```bash
mkdir -p ~/.teams-agent/{contacts,groups,patterns}
```

#### Step 2: Build Persona

Analyze the user's own messages to extract their communication style:

1. Get the user's chats: `teams-cli chats list --format json`
2. Pick 5-10 active chats and fetch messages: `teams-cli messages list <id> -n 50 --format json`
3. Get user's identity: `teams-cli me --format json`
4. From the fetched messages, identify which messages are FROM the user (match by display name from `me`)
5. Analyze the user's messages for:
   - Tone (casual, formal, mixed)
   - Common phrases they actually use (extract exact quotes)
   - Response length patterns (short/detailed)
   - Greeting style ("hey", "hi", "hello")
   - Sign-off style (if any)
   - Emoji usage (none, minimal, frequent)
   - Things they seem to avoid
6. Write the profile to `~/.teams-agent/persona.md`

**persona.md format:**

```markdown
# [Name]'s Communication Style

## General Tone
- [Overall style: casual/formal/mixed]
- [Response length: brief/moderate/detailed]
- [Formality: low/medium/high]

## Common Phrases
- "[phrase 1]"
- "[phrase 2]"
- "[phrase 3]"

## Greeting Style
- [How they typically open messages]

## Things I Avoid
- [Patterns NOT seen in their messages]

## Work Context
- Role: [from user profile]
- Timezone: [if determinable]
```

#### Step 3: Build Contact Profiles

For each frequent contact found in chats:

1. Fetch conversation history: `teams-cli messages list <chat-id> -n 50 --format json`
2. Look up contact info: `teams-cli users search <email> --format json`
3. Analyze the conversation for:
   - Relationship type (teammate, manager, report, external)
   - Closeness level (close, friendly, professional, formal, new)
   - Common topics discussed
   - Tone of exchanges (how formal/casual the conversation is)
   - Who initiates more often
   - Sample exchanges that capture the dynamic
4. Write to `~/.teams-agent/contacts/<firstname-lastname>.md`

**Contact profile format:**

```markdown
# [Contact Name]
email: [email]
chat_id: [1:1 chat ID]

## Relationship
- Type: [teammate | manager | report | cross-team | external]
- Closeness: [close | friendly | professional | formal | new]
- Team: [team name if known]

## Cultural Context
- Humor: [none | light-work-only | casual]
- Formality: [low | medium | high]
- Note: [any relevant context, default to "neutral — use professional tone"]

## Communication Pattern
- They send: [message style description]
- I reply with: [my typical reply style to this person]
- Common topics: [what we discuss]

## Mention Behavior (in groups)
- How to @mention: [first name]

## My Phrases With Them
- "[phrase 1]"
- "[phrase 2]"

## Sample Exchanges
Them: "[example message]"
Me: "[example reply]"

## Preferences
- Puns allowed: no
- Auto-reply: no
```

#### Step 4: Build Group Profiles

For each group chat:

1. Fetch messages: `teams-cli messages list <chat-id> -n 50 --format json`
2. Identify all members and cross-reference with contact profiles
3. Analyze group dynamics:
   - Who talks most
   - What topics come up
   - Overall formality level
   - Who sets the tone
4. Build a relationship diagram showing how members relate
5. Write to `~/.teams-agent/groups/<group-name>.md`

**Group profile format:**

```markdown
# [Group Name]
chat_id: [chat ID]
type: [team-channel | group-chat]

## Members & Relationships
- [Name] — [relationship to user], [tone] (email: [email])
- [Name] — [relationship to user], [tone] (email: [email])

## Relationship Diagram
[Describe how members relate to each other and to the user]
[Who reports to whom, who works closely, who is new]

## Group Dynamics
- Overall tone: [description]
- Tone setter: [who sets the formality floor]
- Cultural mix: [if relevant, note to stay neutral/inclusive]

## Mention Rules
- @[Name]: [when to mention]
- Default: don't over-mention, only @ when specifically addressing someone

## My Behavior in This Group
- [How user adjusts tone vs 1:1]
- [What to avoid in this group]
```

#### Step 5: Build Response Patterns

Look across all conversations for recurring topics and how the user responds:

1. Common patterns to extract:
   - Code review requests
   - Meeting scheduling
   - Status updates
   - Greetings and sign-offs
   - Help requests
2. Write to `~/.teams-agent/patterns/<topic>.md`

**Pattern format:**

```markdown
# [Topic Name]

## When This Applies
- [Trigger descriptions]

## Response by Relationship
- Close teammate: "[casual response]"
- Manager: "[professional response]"
- New contact: "[neutral response]"
```

#### Step 6: Write Safety Rules

Write the hardcoded safety rules to `~/.teams-agent/safety-rules.md`:

```markdown
# Safety Rules — ABSOLUTE, NON-NEGOTIABLE

## Communication Safety
- NEVER make jokes about race, religion, ethnicity, gender, politics, sexuality, disability
- NEVER use sarcasm that could be misread across cultures
- NEVER make puns unless EXPLICITLY enabled for a specific contact
- NEVER assume cultural norms — default to professional-neutral
- NEVER discuss politics, religion, or controversial topics
- NEVER use slang or idioms that may not translate across cultures
- NEVER use passive-aggressive language
- If humor is allowed for a contact, keep it strictly work-related and light

## Operational Safety
- NEVER send a message without user confirmation
- NEVER reply to unknown contacts without asking the user
- NEVER forward or quote messages between different people
- NEVER share personal info about one contact with another
- NEVER escalate tone — if someone is upset, stay calm and neutral
- If unsure about ANYTHING, ask the user

## Group Chat Safety
- NEVER use inside jokes from 1:1 chats in group contexts
- NEVER over-mention (@) people
- In mixed groups, use the most neutral/formal tone appropriate
```

#### Step 7: Write Default Config

```yaml
# ~/.teams-agent/config.yaml
safe_mode: true
confirm_before_send: true
default_tone: "professional"
default_formality: "medium"
max_history_messages: 30
puns_allowed: false
humor_default: "none"
cultural_default: "neutral"
```

---

## Drafting Replies with Context

When drafting a reply, load context in this order:

1. **Safety rules** — read `~/.teams-agent/safety-rules.md` (always apply)
2. **Persona** — read `~/.teams-agent/persona.md` (base tone)
3. **Contact profile** — read `~/.teams-agent/contacts/<name>.md` (relationship-specific tone)
4. **Group profile** — if group chat, read `~/.teams-agent/groups/<name>.md` (group dynamics)
5. **Patterns** — read relevant `~/.teams-agent/patterns/*.md` (topic-specific templates)
6. **Conversation history** — fetch via `teams-cli messages list` (recent context)

Then draft the reply following these rules:

- Match the user's exact communication style from persona.md
- Adjust tone based on the contact's closeness and formality level
- In group chats, the formality floor = the most formal member present
- Use the user's actual phrases, not generic language
- Keep response length consistent with the user's pattern
- If the contact profile says "humor: none", keep it strictly professional
- If cultural context is unknown, default to neutral/professional
- For mentions in group chats, use `<at id="[mri]">[Name]</at>` format

---

## Updating Context

### Update After Conversations

After helping with messages, offer to update context if new patterns emerged:

- "I noticed you used a new phrase with Alice — want me to update her contact profile?"
- "This is a new contact (Dave). Want me to create a profile for them?"
- "The group dynamics seem to have shifted — Carol is more active now. Update the group profile?"

### Manually Update

The user can ask to:
- "Update my persona" — re-analyze recent messages
- "Add a contact" — create a new contact profile
- "Update Alice's profile" — fetch recent conversations and update
- "Add a response pattern for deployment questions"
- "Show me my persona" — read and display persona.md
- "Show me Alice's profile" — read and display the contact file

### Learning from New Conversations

When updating profiles, MERGE new information — don't replace. Add new phrases, update
patterns, note tone shifts. Always show the user what changed before writing.

---

## Handling Common Scenarios

### Unknown Contact Messages

If a message is from someone not in contacts/:
1. Check if a contact profile exists
2. If not, say: "This is from [name] — I don't have a profile for them yet. Replying in professional-neutral tone."
3. Draft reply with default professional tone
4. After sending, offer to create a contact profile

### Group Chat Replies

1. Load the group profile from groups/
2. Identify who sent the message you're replying to
3. If replying to a specific person, mention them: `<at id="[mri]">[Name]</at>`
4. If it's a general message, don't mention anyone
5. Use the group's tone rules, not the 1:1 tone for that person
6. Never use inside references from 1:1 conversations

### Sensitive Topics

If a message touches on politics, religion, or controversial topics:
- Do NOT engage with the topic
- Draft a neutral deflection: "I'd rather not get into that here" or redirect to work topics
- Flag to the user: "This message touches on [topic]. I've drafted a neutral response."

### Angry or Upset Messages

- Never match or escalate the tone
- Draft a calm, empathetic response
- Flag to user: "This person seems upset. Here's a measured response."

---

## Quick Reference

| Task | Command |
|------|---------|
| Check unreads | `teams-cli chats list --unread --format json` |
| Read messages | `teams-cli messages list <chat-id> -n N --format json` |
| Send message | `teams-cli messages send <chat-id> "text"` |
| Send by email | `teams-cli messages send --to email "text"` |
| Search | `teams-cli messages search "query" --format json` |
| List teams | `teams-cli teams list --format json` |
| List channels | `teams-cli channels list <team-id> --format json` |
| Look up user | `teams-cli users search email --format json` |
| My profile | `teams-cli me --format json` |
| Auth status | `teams-cli status --format json` |
| Re-auth | `teams-cli auth` |

## Error Handling

- If any `teams-cli` command fails with "authentication failed": tell the user to run `teams-cli auth`
- If a chat ID is not found: list chats and help the user find the right one
- If `~/.teams-agent/` doesn't exist: offer to set it up (see Context System above)
- If a contact profile is missing: proceed with professional-neutral tone, offer to create one after
