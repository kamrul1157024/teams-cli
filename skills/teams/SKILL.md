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

## Step 0: Preflight Checks (ALWAYS DO FIRST)

Before ANY Teams operation, run these checks in order. Do not skip.

### 0a. Verify Authentication

```bash
teams-cli status --format json
```

Check that ALL three tokens (teams, skype, chatsvcagg) show `"valid": true`.
If ANY token is invalid or missing, tell the user:

> "Your Teams tokens are expired. Please run `! teams-cli auth` to re-authenticate."

**Do NOT proceed until all tokens are valid.** The chats API uses chatsvcagg, the messages
API uses skype — if only one is valid, some commands will fail with 401.

**Token expiry awareness:** Tokens last ~1 hour. If you're doing a long task (like profile
setup), re-check `teams-cli status --format json` every 15-20 minutes. The messages API
may fail with 401 even when the chats API still works because they use different tokens.

### 0b. Check Profile Exists

```bash
ls ~/.teams-agent/persona.md 2>/dev/null
```

If `~/.teams-agent/persona.md` does NOT exist, **start profile setup immediately** before
doing anything else. Do not ask — just begin the setup process described in "Building Your
Profile" below. The profile is required for persona-aware replies.

If the profile exists, proceed to what the user asked for.

---

## Built-in Cache

The CLI has a built-in file-based cache at `~/.config/teams-cli/cache/`. This means you
do NOT need to maintain your own cache in `~/.teams-agent/` — the CLI handles it.

**What is cached (and TTLs):**

| Data | TTL | Notes |
|------|-----|-------|
| Conversations (chats list) | never | Always fetches fresh data |
| User info (users search) | 1 hour | Name, email, MRI, department |
| Own profile (me) | 1 hour | Very stable |
| Chat ID resolution (chats resolve) | 10 min | Email → chat ID mapping |
| Teams/channels | 30 min | Team structure |

**What is NEVER cached:**

- `messages list` — always fetches fresh messages
- DM conversations (Skype API) — always fetches fresh DM list with unread state
- `chats list --unread` — always bypasses cache for fresh unread state
- `messages search` — always searches live
- `messages send` — invalidates conversations cache after sending

**Cache control flags:**

```bash
teams-cli chats list --no-cache --format json     # Bypass cache this time
teams-cli chats list --refresh --format json       # Ignore cache, write fresh
teams-cli cache clear                              # Delete all cached data
```

**Why this matters for the skill:** Since the CLI caches user lookups, chat resolution,
and conversation lists, the skill doesn't need to cache these. Just call the commands
normally — repeated calls within the TTL window are instant from cache. Use `--no-cache`
when you need guaranteed-fresh data (e.g., checking if a new message arrived).

**People also chat via Teams UI:** If someone sends a message through the desktop/web app,
the conversations cache (5 min TTL) will pick it up on the next call. For `--unread`
queries, the cache is always bypassed to avoid showing stale read/unread state.

---

## Building Your Profile (First-Time Setup)

This runs automatically when `/teams` is invoked and no profile exists. The profile
enables persona-aware messaging — without it, replies will be generic.

### Step 1: Create Directory Structure & Get Identity

```bash
mkdir -p ~/.teams-agent/{contacts,groups,patterns}
teams-cli me --format json
```

Save the user's display name — this is used to identify which messages in chat history
are FROM the user.

### Step 2: Build Persona

Analyze the user's own messages to extract their communication style.

1. Get active chats (keep output small):
```bash
teams-cli chats list --format json 2>&1 | python3 -c "
import json,sys
data=json.load(sys.stdin)
# Pick chats with recent messages, prefer 1:1 for cleaner signal
for c in data[:15]:
    print(json.dumps({'id':c['id'],'title':c.get('title',''),'type':c['type']}))" 
```

2. Fetch messages from 5-10 active chats. Run these in **parallel** to save time:
```bash
teams-cli messages list "<chat-id>" -n 50 --format json 2>&1 | python3 -c "
import json,sys
data=json.load(sys.stdin)
for m in data:
    print(json.dumps({'from':m['from'],'content':m['content'][:200],'time':m['time']}))"
```

3. From the fetched messages, filter to messages FROM the user (match display name from Step 1)
4. Analyze the user's messages for:
   - Tone (casual, formal, mixed)
   - Common phrases they actually use (extract exact quotes)
   - Response length patterns (short/detailed)
   - Greeting style ("hey", "hi", "hello")
   - Sign-off style (if any)
   - Emoji usage (none, minimal, frequent)
   - Things they seem to avoid
5. Write to `~/.teams-agent/persona.md`

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

## Bangla Communication
- Default pronoun: tumi
- Use "apni" for: [seniors, managers, formal contacts]
- Use "tui" for: [very close friends only, if explicitly allowed]
- Bangla script usage: [yes/no, when]

## Work Context
- Role: [from user profile]
- Timezone: [if determinable]
```

### Step 3: Build Contact Profiles

For each frequent contact found in chat history:

1. Fetch conversation history (already done in Step 2, reuse that data)
2. Look up contact info if email is available: `teams-cli users search <email> --format json`
3. Analyze the conversation for:
   - Relationship type (teammate, manager, report, external)
   - Closeness level (close, friendly, professional, formal, new)
   - Common topics discussed
   - Tone of exchanges
   - Who initiates more often
   - Sample exchanges that capture the dynamic
4. Write to `~/.teams-agent/contacts/<firstname-lastname>.md`

**Name disambiguation:** Multiple people can have similar names (e.g., "Kamrul Hassan" vs
"Kamrul Hasan"). Always cross-reference with email or MRI to avoid merging different people
into one profile. When in doubt, ask the user.

**Contact profile format:**

```markdown
# [Contact Name]
email: [email]

## Bangla Pronoun
- pronoun: [tui | tumi | apni]
- Note: default to "tumi" if unsure. "apni" for seniors/managers, "tui" only if very close

## Chat IDs
- [1:1 chat ID, if exists]
- [group chat IDs where this contact appears]

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

### Step 4: Build Group Profiles

For each group chat with recent activity:

1. Fetch messages (reuse from Step 2 where possible)
2. Identify all members and cross-reference with contact profiles
3. **Resolve MRI-only titles:** Some group chats have no human title and show raw MRI
   strings (e.g., "8:orgid:xxx, 8:orgid:yyy +1 more"). Resolve member names from
   message `from` fields or `teams-cli users search` and create a meaningful title
   like "Alice, Bob, Carol" for the group profile.
4. Analyze group dynamics:
   - Who talks most
   - What topics come up
   - Overall formality level
   - Who sets the tone
5. Build a relationship diagram showing how members relate
6. Write to `~/.teams-agent/groups/<group-name>.md`

**Group profile format:**

```markdown
# [Group Name]
chat_id: [chat ID]
type: [team-channel | group-chat]
human_title: [resolved readable title if original was MRI strings]

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

### Step 5: Build Response Patterns

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

### Step 6: Write Safety Rules

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

### Step 7: Write Default Config

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

## teams-cli Command Reference

### Authentication

```bash
teams-cli auth                    # Launch OAuth login
teams-cli auth --force            # Force re-auth even if tokens valid
teams-cli status --format json    # Check token health
teams-cli me --format json        # Current user profile
```

### Configuration

```bash
teams-cli config show --format json        # Show all config
teams-cli config signature on              # Enable "sent via claude" signature
teams-cli config signature off             # Disable signature
teams-cli config signature set "custom"    # Set custom signature text
```

**Message signature:** By default, all messages sent via the CLI are appended with
`— sent via claude 🤖`. This lets recipients know the message was composed with AI
assistance. The user can ask to turn it on/off:

- "Turn off the claude signature" → run `teams-cli config signature off`
- "Turn it back on" → run `teams-cli config signature on`
- "Change the signature to ..." → run `teams-cli config signature set "new text"`

### Chats

```bash
teams-cli chats list --format json                           # All chats
teams-cli chats list --unread --format json                   # Unread only
teams-cli chats list --type dm --format json                  # True DMs only (not meeting chats)
teams-cli chats list --type "1:1" --format json               # 1:1 chats (meeting-based)
teams-cli chats list --type group --format json               # Group chats only
teams-cli chats list --type dm --include-bots --format json   # DMs including bot members
teams-cli chats list --limit 10 --format json                 # Limit results
teams-cli chats list --offset 10 --limit 10 --format json     # Pagination
teams-cli chats list --compact --format json                  # Omit members array
teams-cli chats list --with "alice" --format json             # Find chats with a person
teams-cli chats list --active-since 2024-01-01 --format json  # Filter by activity date
teams-cli chats resolve user@company.com                      # Get DM chat ID from email
teams-cli chats create --with user@company.com --format json  # Create new 1:1 chat
```

**Chat types:**
- `dm` — True 1:1 direct messages (personal chats at `@unq.gbl.spaces`)
- `1:1` — Meeting-based 1:1 chats (scheduled meetings with 2 people)
- `group` — Group chats and channels

By default, bot members (Jira, Copilot, etc.) are hidden from DM listings. Use `--include-bots` to show them.

### Messages

```bash
teams-cli messages list <chat-id> --format json               # Read messages (default 50)
teams-cli messages list --to user@company.com --format json    # Read by email
teams-cli messages list <chat-id> -n 20 --format json         # Last N messages
teams-cli messages list <chat-id> --mine --format json         # Only my messages
teams-cli messages list <chat-id> --since 2024-01-01           # Since date
teams-cli messages list <chat-id> --from "alice" --format json # Filter by sender
teams-cli messages list <chat-id> --plain --format text        # Clean text output
teams-cli messages list <chat-id> --before <time> --after <time> # Cursor pagination
teams-cli messages send <chat-id> "message text"               # Send by chat ID
teams-cli messages send --to user@company.com "text"           # Send by email (creates 1:1 if needed)
teams-cli messages send --group "Monad Standup" "Hello team"   # Send to group by name
teams-cli messages send <chat-id> "**bold** text" --msg-format markdown  # Markdown → HTML
teams-cli messages send <chat-id> "Hey @alice" --mention alice=alice@co.com  # @mention
teams-cli messages send <chat-id> "text" --reply-to <msg-id>  # Reply to specific message
teams-cli messages reply <chat-id> <msg-id> "reply text"       # Threaded reply
teams-cli messages reply <chat-id> <msg-id> "text" --quote <quote-msg-id>  # Reply quoting another message
teams-cli messages send <chat-id> "text" --quote <msg-id>      # Send with embedded quote
teams-cli messages edit <chat-id> <msg-id> "updated text"      # Edit sent message
teams-cli messages edit <chat-id> <msg-id> "text" --quote <quote-msg-id>  # Edit with embedded quote
teams-cli messages delete <chat-id> <msg-id> --confirm         # Delete sent message
teams-cli messages search "query" --format json                # Search messages
teams-cli messages search "query" --chat <id> --format json    # Search in specific chat
teams-cli messages mine --format json                          # My messages across chats
teams-cli messages stats <chat-id> --format json               # Message count by sender
teams-cli messages export <chat-id> -o chat.json               # Export chat history
teams-cli messages react <chat-id> <message-id> like           # React with emoji
teams-cli messages react <chat-id> <message-id> heart          # React with heart
```

**Markdown support (`--msg-format markdown`):** Converts markdown to Teams HTML:
- `**bold**` → bold, `*italic*` → italic, `` `code` `` → inline code
- ` ```code blocks``` `, `~~strikethrough~~`, `# headers`, `- lists`

**@mentions (`--mention`):** Resolves email to Teams MRI and wraps in `<at>` tag.
Multiple mentions: `--mention alice=alice@co.com --mention bob=bob@co.com`

**Emoji reactions:** Valid reactions are: `like` (👍), `heart` (❤️), `laugh` (😂),
`surprised` (😮), `sad` (😢), `angry` (😡). You can also use the emoji characters directly.

**Quoting messages (`--quote <msg-id>`):** Embeds another user's message as a blockquote
in your reply/send/edit. The quoted message appears as a visual quote block in Teams with
the original sender's name and a preview of their message. Use this when responding to a
specific message in a thread — it makes clear which message you're addressing.

When the skill targets a specific message (e.g., replying to what someone said in a thread),
always use `--quote` to embed the referenced message for context.

### Teams & Channels

```bash
teams-cli teams list --format json                     # List joined teams
teams-cli channels list <team-id> --format json        # List channels in a team
```

### Notifications

```bash
teams-cli notifications --format json                    # All notifications
teams-cli notifications --type mentions --format json    # Only @mentions
teams-cli notifications --type replies --format json     # Only replies to your messages
teams-cli notifications --type reactions --format json   # Only emoji reactions
teams-cli notifications --since 2024-04-07 --format json # Since a specific date
teams-cli notifications --since 2024-04-07 --type mentions --format json  # Combined filters
```

**Notification types:** `mention` (channel @mention), `mentionInChat` (DM @mention),
`replyToReply` (thread reply), `reactionInChat` (emoji reaction), `follow` (channel activity).

**Use cases:**
- "Catch me up on today" → `notifications --since today's-date --format json`
- "Who mentioned me?" → `notifications --type mentions --format json`
- "What needs my attention?" → `notifications --type mentions --since <date> --format json`

### Users

```bash
teams-cli users search user@company.com --format json  # Look up by email
teams-cli users search "Alice Smith" --format json     # Look up by display name
teams-cli users resolve "mri1,mri2" --format json     # Batch resolve MRIs
```

### Contacts

```bash
teams-cli contacts list --format json                  # All unique contacts from chats
```

### Output Formats

All commands support `--format` with values: `json` (default), `table`, `text`.
Use `--pretty` for pretty-printed JSON.

---

## Handling Large JSON Output

The chat list can return 100KB+ of JSON that floods the context window. **ALWAYS** pipe
through python3 or jq to extract only the fields you need:

```bash
# Bad — dumps everything into context
teams-cli chats list --format json

# Good — extract only what you need
teams-cli chats list --format json 2>&1 | python3 -c "
import json,sys
data=json.load(sys.stdin)
for c in data[:20]:
    print(json.dumps({
        'id': c['id'],
        'title': c.get('title',''),
        'type': c['type'],
        'is_read': c['is_read'],
        'last_msg': c.get('last_message',{}).get('content','')[:80] if c.get('last_message') else ''
    }))"
```

```bash
# Bad — raw message dump
teams-cli messages list <id> -n 50 --format json

# Good — compact summary
teams-cli messages list <id> -n 50 --format json 2>&1 | python3 -c "
import json,sys
data=json.load(sys.stdin)
for m in data:
    print(json.dumps({'from':m['from'],'content':m['content'][:200],'time':m['time']}))"
```

When fetching from multiple chats, always use the compact form. The raw JSON from even
a single chat can be several KB.

---

## How to Use This Skill

When the user invokes `/teams`:

1. Run **Step 0: Preflight Checks** (auth + profile existence)
2. If no profile exists, run **Building Your Profile** automatically
3. Then ask what they want to do

### Check Unread Messages

1. Fetch unread chats (compact output):
```bash
teams-cli chats list --unread --format json 2>&1 | python3 -c "
import json,sys
data=json.load(sys.stdin)
for c in data:
    print(json.dumps({'id':c['id'],'title':c.get('title',''),'type':c['type']}))"
```
2. For each unread chat, fetch messages (compact):
```bash
teams-cli messages list <chat-id> -n 10 --format json 2>&1 | python3 -c "
import json,sys
data=json.load(sys.stdin)
for m in data:
    print(json.dumps({'from':m['from'],'content':m['content'][:200],'time':m['time']}))"
```
3. Summarize all unread messages grouped by chat/person
4. Highlight anything that looks urgent or needs a response
5. Offer to draft replies

### Read a Specific Chat

1. If user gives a chat ID, fetch messages directly (compact form)
2. If user gives a name/email, first find the chat:
   - Fetch chat list (compact) and match by title or member name
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

1. Read recent messages for context (compact form)
2. Load persona + contact profile + safety rules from `~/.teams-agent/`
3. Draft reply matching the user's style and relationship with the recipient
4. Show draft and wait for confirmation

---

## Drafting Replies with Context

When drafting a reply, load context in this order. This is a **mandatory checklist** — do not
skip any step. If a file is missing, note it and proceed with defaults.

- [ ] **Safety rules** — read `~/.teams-agent/safety-rules.md` (always apply)
- [ ] **Persona** — read `~/.teams-agent/persona.md` (base tone)
- [ ] **Contact profile** — read `~/.teams-agent/contacts/<name>.md` (relationship-specific tone)
- [ ] **Group profile** — if group chat, read `~/.teams-agent/groups/<name>.md` (group dynamics)
- [ ] **Patterns** — read relevant `~/.teams-agent/patterns/*.md` (topic-specific templates)
- [ ] **Conversation history** — fetch via `teams-cli messages list` (recent context)

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

### Incremental Updates

When updating profiles, don't re-fetch everything from scratch:

1. Check when the profile was last updated (look at file modification time)
2. Only fetch messages newer than the last update
3. MERGE new information — don't replace the existing profile
4. Add new phrases, update patterns, note tone shifts
5. Always show the user what changed before writing

### Manually Update

The user can ask to:
- "Update my persona" — re-analyze recent messages
- "Add a contact" — create a new contact profile
- "Update Alice's profile" — fetch recent conversations and update
- "Add a response pattern for deployment questions"
- "Show me my persona" — read and display persona.md
- "Show me Alice's profile" — read and display the contact file

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
| Notifications | `teams-cli notifications --format json` |
| Mentions | `teams-cli notifications --type mentions --format json` |
| Catch up | `teams-cli notifications --since YYYY-MM-DD --format json` |
| List DMs | `teams-cli chats list --type dm --format json` |
| Check unreads | `teams-cli chats list --unread --format json` |
| Read messages | `teams-cli messages list <chat-id> -n N --format json` |
| Read by email | `teams-cli messages list --to email --format json` |
| My messages | `teams-cli messages list <chat-id> --mine --format json` |
| Send message | `teams-cli messages send <chat-id> "text"` |
| Send by email | `teams-cli messages send --to email "text"` |
| Send to group | `teams-cli messages send --group "name" "text"` |
| Send markdown | `teams-cli messages send <chat-id> "**bold**" --msg-format markdown` |
| @mention | `teams-cli messages send <chat-id> "Hey @alice" --mention alice=email` |
| Reply thread | `teams-cli messages reply <chat-id> <msg-id> "text"` |
| Reply + quote | `teams-cli messages reply <chat-id> <msg-id> "text" --quote <quote-id>` |
| Edit message | `teams-cli messages edit <chat-id> <msg-id> "new text"` |
| Delete message | `teams-cli messages delete <chat-id> <msg-id> --confirm` |
| Search | `teams-cli messages search "query" --format json` |
| Resolve email | `teams-cli chats resolve email` |
| Create DM | `teams-cli chats create --with email --format json` |
| Find chats with | `teams-cli chats list --with "name" --format json` |
| Compact chats | `teams-cli chats list --compact --format json` |
| Chat stats | `teams-cli messages stats <chat-id> --format json` |
| Export chat | `teams-cli messages export <chat-id> -o file.json` |
| All contacts | `teams-cli contacts list --format json` |
| List teams | `teams-cli teams list --format json` |
| List channels | `teams-cli channels list <team-id> --format json` |
| Look up user | `teams-cli users search email-or-name --format json` |
| Resolve MRIs | `teams-cli users resolve "mri1,mri2" --format json` |
| My profile | `teams-cli me --format json` |
| Auth status | `teams-cli status --format json` |
| Re-auth | `teams-cli auth` |
| React emoji | `teams-cli messages react <chat-id> <msg-id> like` |
| Clear cache | `teams-cli cache clear` |
| Show config | `teams-cli config show --format json` |
| Signature on | `teams-cli config signature on` |
| Signature off | `teams-cli config signature off` |
| Set signature | `teams-cli config signature set "text"` |

## Error Handling

- **"authentication failed" / HTTP 401:** Tell the user to run `! teams-cli auth`. Note that
  the chats API (chatsvcagg token) and messages API (skype token) use different tokens — one
  can expire while the other is still valid.
- **Chat ID not found:** List chats and help the user find the right one.
- **`~/.teams-agent/` doesn't exist:** Start profile setup automatically.
- **Contact profile missing:** Proceed with professional-neutral tone, offer to create one after.
- **Large JSON response:** Always pipe through python3 to extract needed fields (see "Handling Large JSON Output").
- **`--from` filter returns empty:** Use python3 post-processing filter instead (see "Known issue" in Messages section).
- **Send message parse error:** The send command may return a JSON parsing error even when the
  message was actually delivered. If you get a parse/unmarshal error after sending, verify
  delivery by fetching recent messages from the same chat:
  ```bash
  teams-cli messages list <chat-id> -n 5 --format json
  ```
  If the sent message appears in the results, it was delivered despite the error.
- **Users search returns empty/error:** The `teams-cli users search` command requires an
  **exact email address** — it does NOT support display name search. If you only have a
  display name, find the email by:
  1. Checking contact profiles in `~/.teams-agent/contacts/`
  2. Looking at chat member lists from `teams-cli chats list --format json` (members include MRI which contains the email)
  3. Asking the user for the email

## Chat Validation Before Sending

**ALWAYS validate before sending a message to a chat ID:**

1. **Distinguish true 1:1 from meeting chats:** Meeting chat IDs often contain `meeting_` in
   the ID or have titles like "Understand Task..." that don't match a person's name. True 1:1
   chats have IDs like `19:xxxx@thread.v2` without `meeting_`.
   - For personal messages, **prefer true 1:1 chats** (no `meeting_` in ID)
   - If only meeting chats exist with the person, warn the user:
     > "This is a meeting chat, not a true 1:1. Other participants may have been in this chat.
     > Want me to send here or use `--to email` to create a proper 1:1?"
   - When in doubt, use `teams-cli messages send --to email "text"` which guarantees a proper
     direct chat

2. **Check for stale/deleted accounts:** Before sending to a chat, verify members are active:
   - If a member MRI appears in the chat but `teams-cli users search <email>` returns no
     result or an empty display name, that account may be deleted/deactivated
   - Don't send to chats with unknown/deleted members without warning the user:
     > "This chat has a member ([MRI]) whose account appears deleted. Messages here may not
     > reach the intended recipient. Want me to create a fresh 1:1 instead?"

3. **Verify chat is active:** For any chat you haven't used recently:
   - Fetch a few recent messages to confirm the chat has activity
   - If the chat title doesn't match the intended recipient's name, flag it
   - If the chat has no recent messages (>30 days), suggest creating a new conversation

4. **Prefer `--to email` for 1:1 messages:** Using `--to user@company.com` is safer than
   sending to a raw chat ID because it resolves to the correct 1:1 chat or creates one.
