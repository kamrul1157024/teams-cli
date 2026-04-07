# Teams Agent — Technical Specification

## Overview

Teams Agent is a context-aware AI assistant that operates across Microsoft Teams and external systems (GitHub, CI/CD, etc.). It reads Teams messages, understands intent, takes action (e.g., reviews PRs, checks CI), and replies on behalf of the user — all while respecting per-contact relationship dynamics, cultural sensitivity, and strict safety rules.

It is NOT a passive auto-responder. It is a **task-executing agent** triggered on-demand that understands:
- **Who** is talking (contact profile, relationship, cultural context)
- **What** they want (intent detection: PR review, status check, question, casual chat)
- **How** to respond (persona-matched tone, relationship-appropriate formality)
- **What actions** to take before responding (review code, check CI, look up info)

---

## Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    User Interaction Layer                        │
│                                                                 │
│   ┌──────────────────────┐    ┌──────────────────────────────┐  │
│   │  Claude Code Skill   │    │  teams-cli agent run         │  │
│   │  /teams              │    │  (standalone CLI mode)        │  │
│   │  (interactive)       │    │  (uses claude -p)             │  │
│   └──────────┬───────────┘    └──────────────┬───────────────┘  │
│              │                                │                  │
└──────────────┼────────────────────────────────┼──────────────────┘
               │                                │
               ▼                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Context Assembly Layer                       │
│                                                                 │
│   ┌───────────┐  ┌───────────┐  ┌──────────┐  ┌─────────────┐  │
│   │ persona.md│  │contacts/  │  │ groups/  │  │ actions/    │  │
│   │           │  │  alice.md │  │ backend  │  │  pr-review  │  │
│   │ user's    │  │  bob.md   │  │  -team.md│  │  ci-status  │  │
│   │ style     │  │  ...      │  │  ...     │  │  ...        │  │
│   └───────────┘  └───────────┘  └──────────┘  └─────────────┘  │
│                                                                 │
│   ┌────────────────────────────────────────────────────────┐    │
│   │  safety-rules.md (ALWAYS loaded, NEVER overridden)     │    │
│   └────────────────────────────────────────────────────────┘    │
│                                                                 │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Execution Layer                              │
│                                                                 │
│   ┌──────────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│   │  teams-cli        │  │  gh (GitHub)  │  │  git             │  │
│   │  - chats list     │  │  - pr view   │  │  - log           │  │
│   │  - messages list  │  │  - pr diff   │  │  - diff          │  │
│   │  - messages send  │  │  - pr review │  │  - status        │  │
│   │  - users search   │  │  - run list  │  │                  │  │
│   └──────────────────┘  └──────────────┘  └──────────────────┘  │
│                                                                 │
│   ┌──────────────────┐  ┌──────────────┐                        │
│   │  Any CLI tool     │  │  Web APIs    │                        │
│   │  (jira, kubectl,  │  │  (via curl)  │                        │
│   │   etc.)           │  │              │                        │
│   └──────────────────┘  └──────────────┘                        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Message Processing Pipeline

```
┌──────────────┐     ┌──────────────┐     ┌──────────────────┐
│  1. FETCH    │     │  2. IDENTIFY │     │  3. DETECT       │
│              │     │              │     │     INTENT       │
│  teams-cli   │────▶│  Match to    │────▶│                  │
│  chats list  │     │  contact/    │     │  PR review?      │
│  --unread    │     │  group       │     │  Status check?   │
│              │     │  profile     │     │  Question?       │
└──────────────┘     └──────────────┘     │  Casual chat?    │
                                          └────────┬─────────┘
                                                   │
                          ┌────────────────────────┐│
                          │                        ││
                          ▼                        ▼▼
               ┌──────────────────┐     ┌──────────────────┐
               │  4a. EXECUTE     │     │  4b. DRAFT       │
               │     ACTION       │     │     REPLY        │
               │                  │     │                  │
               │  gh pr review    │     │  Load persona    │
               │  gh run list     │     │  Load contact    │
               │  git log         │     │  Load patterns   │
               │  ...             │     │  Generate reply  │
               └────────┬─────────┘     └────────┬─────────┘
                        │                         │
                        └────────────┬────────────┘
                                     ▼
                          ┌──────────────────┐
                          │  5. CONFIRM      │
                          │                  │
                          │  Show draft to   │
                          │  user, wait for  │
                          │  approval        │
                          └────────┬─────────┘
                                   │
                                   ▼
                          ┌──────────────────┐     ┌──────────────┐
                          │  6. SEND         │────▶│  7. LOG      │
                          │                  │     │              │
                          │  teams-cli       │     │  Update      │
                          │  messages send   │     │  contact     │
                          │                  │     │  profile     │
                          └──────────────────┘     └──────────────┘
```

### Relationship & Context Model

```
┌─────────────────────────────────────────────────────────────┐
│                    Persona Layer (YOU)                        │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  persona.md                                         │    │
│  │  - Communication style (casual, formal, mixed)      │    │
│  │  - Common phrases ("sure", "sounds good", etc.)     │    │
│  │  - Things you never say                             │    │
│  │  - Work style (timezone, response speed)            │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  Applied to ALL outgoing messages as base tone              │
└─────────────────────────────┬───────────────────────────────┘
                              │ modified by
                              ▼
┌─────────────────────────────────────────────────────────────┐
│               Relationship Layer (PER CONTACT)               │
│                                                             │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────────┐    │
│  │  alice.md    │ │  bob.md      │ │  carol.md        │    │
│  │              │ │              │ │                   │    │
│  │  Closeness:  │ │  Closeness:  │ │  Closeness:      │    │
│  │  close       │ │  formal      │ │  new colleague   │    │
│  │              │ │              │ │                   │    │
│  │  Tone: very  │ │  Tone: semi- │ │  Tone: warm but  │    │
│  │  casual      │ │  formal      │ │  professional    │    │
│  │              │ │              │ │                   │    │
│  │  Culture:    │ │  Culture:    │ │  Culture:        │    │
│  │  same bg,    │ │  different   │ │  unknown yet,    │    │
│  │  humor OK    │ │  bg, stick   │ │  default to      │    │
│  │              │ │  to neutral  │ │  neutral          │    │
│  └──────────────┘ └──────────────┘ └──────────────────┘    │
│                                                             │
│  Each contact modifies the base persona tone                │
└─────────────────────────────┬───────────────────────────────┘
                              │ further modified by
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              Context Layer (SITUATION-SPECIFIC)               │
│                                                             │
│  ┌──────────────────────┐  ┌────────────────────────────┐   │
│  │  1:1 Chat            │  │  Group Chat                │   │
│  │  - Full persona +    │  │  - Most formal member      │   │
│  │    contact style     │  │    sets the floor           │   │
│  │  - Can be more       │  │  - Mention rules apply     │   │
│  │    personal          │  │  - Read room dynamics       │   │
│  │  - Inside refs OK    │  │  - No inside jokes          │   │
│  │    if relationship   │  │  - Be inclusive             │   │
│  │    supports it       │  │                            │   │
│  └──────────────────────┘  └────────────────────────────┘   │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Action Patterns (pr-review.md, ci-status.md, etc.) │   │
│  │  - Override reply template based on what was done    │   │
│  │  - Still respect persona + contact tone              │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Group Chat Relationship Diagram

```
                    ┌─────────────────────┐
                    │  #backend-team      │
                    │  Group Chat         │
                    └──────────┬──────────┘
                               │
          ┌────────────────────┼────────────────────┐
          │                    │                    │
    ┌─────▼─────┐       ┌─────▼─────┐       ┌─────▼─────┐
    │   Alice    │       │    Bob    │       │   Carol   │
    │  (peer)   │       │ (manager) │       │  (new)    │
    └─────┬─────┘       └─────┬─────┘       └─────┬─────┘
          │                   │                    │
          ▼                   ▼                    ▼
    ┌───────────┐       ┌───────────┐       ┌───────────┐
    │ casual    │       │ semi-     │       │ warm,     │
    │ direct    │       │ formal    │       │ welcoming │
    │ humor OK  │       │ no humor  │       │ inclusive │
    └───────────┘       └───────────┘       └───────────┘

    Group tone = max(formality of members present)
    If Bob is present → semi-formal floor
    If just Alice and me → can be casual
    Mention rules:
      @Bob → decisions only
      @Alice → frontend questions
      @Carol → include in discussions, explain jargon
```

---

## Data Model

### Directory Structure

```
~/.teams-agent/
├── config.yaml                    # Agent configuration
├── persona.md                     # User's communication profile
├── safety-rules.md                # Hard safety constraints (NEVER overridden)
│
├── contacts/                      # Per-person relationship profiles
│   ├── alice-smith.md
│   ├── bob-jones.md
│   └── carol-chen.md
│
├── groups/                        # Group chat dynamics
│   ├── backend-team.md
│   └── project-alpha.md
│
├── patterns/                      # Response templates by topic
│   ├── code-review.md
│   ├── meetings.md
│   ├── status-updates.md
│   └── greetings.md
│
├── actions/                       # Executable action definitions
│   ├── pr-review.md               # Review PRs on GitHub
│   ├── ci-status.md               # Check CI/CD pipeline
│   ├── jira-lookup.md             # Look up ticket status
│   └── schedule.md                # Handle meeting requests
│
└── logs/                          # Conversation logs for learning
    ├── 2026-04-08.jsonl
    └── ...
```

### config.yaml

```yaml
# ~/.teams-agent/config.yaml

# Agent behavior
safe_mode: true                    # Enforce all safety rules (cannot be disabled)
confirm_before_send: true          # Always show draft before sending
auto_reply_contacts: []            # Emails of contacts with auto-reply enabled (empty = none)

# Defaults
default_tone: "professional"       # Fallback tone for unknown contacts
default_formality: "medium"        # low | medium | high
max_history_messages: 30           # Messages to fetch for context per chat
puns_allowed: false                # Global default, overridden per contact

# Learning
log_conversations: true            # Save conversation logs for learning
auto_update_profiles: false        # Auto-update contact profiles after conversations

# Integrations
github_enabled: true               # Allow GitHub actions (PR review, CI check)
jira_enabled: false                # Allow JIRA lookups (requires jira CLI)

# Cultural defaults
cultural_default: "neutral"        # Default cultural assumption for new contacts
humor_default: "none"              # none | light-work-only | casual
```

### persona.md Schema

```markdown
# [User's Name]'s Communication Style

## General Tone
- [Overall style description]
- [Formality level]
- [Response length preference]

## Common Phrases
- [Phrases the user actually uses, extracted from chat history]

## Things I Never Say
- [Anti-patterns to avoid]

## Work Context
- Role: [job title/role]
- Timezone: [timezone]
- Typical response time: [fast/moderate/slow]
- Work hours: [hours]

## Cultural Background
- [Relevant cultural context for communication style]
- [Language preferences]
```

### Contact Profile Schema

```markdown
# [Contact Name]
email: [email]
mri: [Teams MRI identifier]
chat_id: [1:1 chat ID if known]

## Relationship
- Type: [teammate | manager | report | cross-team | external | client]
- Closeness: [close | friendly | professional | formal | new]
- Team: [team name]
- Reports to: [if known]
- Known since: [approximate]

## Cultural Context
- Language: [primary language]
- Background: [relevant cultural notes for communication, keep respectful]
- Humor tolerance: [none | light-work-only | casual]
- Formality preference: [low | medium | high]
- Note: [any specific cultural considerations]

## Communication Pattern
- They send: [message style — short/long, direct/verbose, etc.]
- I reply with: [my typical reply style to this person]
- Common topics: [what we usually discuss]
- Response speed: [how quickly I usually respond]

## Mention Behavior (in groups)
- How to @mention: [first name, full name, or don't mention]
- When to mention: [decisions, questions, etc.]

## My Phrases With Them
- [Actual phrases extracted from history]

## Sample Exchanges
[Real or representative examples]

## Preferences
- Puns allowed: [yes | no]
- Auto-reply: [yes | no]
- Emoji usage: [none | minimal | moderate]
```

### Group Profile Schema

```markdown
# [Group Name]
chat_id: [group chat ID]
type: [team-channel | group-chat]

## Members & Relationships
- [Name] — [relationship], [tone] (email: [email])
- [Name] — [relationship], [tone] (email: [email])

## Relationship Diagram
[Freeform text describing interpersonal dynamics]
[Who reports to whom, who works closely with whom]

## Group Dynamics
- Overall tone: [description]
- Tone setter: [who sets the formality floor]
- Things to avoid: [group-specific anti-patterns]
- Cultural mix: [if relevant, note to stay neutral/inclusive]

## Mention Rules
- @[Name]: [when to mention]
- General: [default mention behavior]
- Never over-mention

## My Behavior in This Group
- [How I adjust tone vs 1:1]
- [What I avoid in this group]
- [Message format preferences: bullets, prose, etc.]
```

### Action Definition Schema

```markdown
# [Action Name]

## Triggers
- [Message patterns that activate this action]
- [Keywords or intents]

## Prerequisites
- [CLI tools needed: gh, jira, kubectl, etc.]
- [Permissions needed]

## Steps
1. [Step-by-step execution plan]
2. [Commands to run]
3. [How to interpret results]

## Reply Templates by Relationship
- Close teammate: "[casual response template]"
- Manager: "[professional response template]"
- Cross-team: "[neutral response template]"
- New contact: "[slightly formal response template]"

## Safety
- [What this action must NEVER do]
- [Confirmation requirements]
- [Scope limits]
```

---

## Safety System

### Safety Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    SAFETY LAYERS                             │
│                                                             │
│  Layer 1: HARD RULES (safety-rules.md)                      │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  - No racist/religious/political content             │    │
│  │  - No offensive humor or puns (unless explicit opt-in│)   │
│  │  - No cultural assumptions                           │    │
│  │  - No forwarding messages between people             │    │
│  │  - No sharing personal info across contacts          │    │
│  │  - Always confirm before sending                     │    │
│  │  CANNOT BE OVERRIDDEN BY ANY LAYER BELOW             │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  Layer 2: CULTURAL AWARENESS                                │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  - Default to neutral/professional                   │    │
│  │  - Avoid idioms that don't translate                 │    │
│  │  - Respect formality preferences per culture         │    │
│  │  - No humor with unknown cultural context            │    │
│  │  - Time zone awareness in expectations               │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  Layer 3: RELATIONSHIP BOUNDARIES                           │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  - Tone locked to contact profile                    │    │
│  │  - Cannot escalate beyond defined closeness          │    │
│  │  - Unknown contacts: professional-only               │    │
│  │  - Group chats: formality floor = most formal member │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  Layer 4: ACTION SAFETY                                     │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  - PR review: review + comment only, NEVER merge    │    │
│  │  - CI: read-only, NEVER trigger deploys             │    │
│  │  - Messages: NEVER delete or edit sent messages      │    │
│  │  - Files: NEVER share files without approval         │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  Layer 5: CONFIRMATION GATE                                 │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  ALL outgoing messages shown to user before sending  │    │
│  │  ALL destructive actions require explicit approval    │    │
│  │  Exception: auto_reply_contacts (opt-in per contact) │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Safety Rules (Full)

The following rules are loaded into EVERY agent prompt and cannot be overridden:

**Communication Safety:**
- NEVER make jokes about race, religion, ethnicity, gender, politics, sexuality, disability, or any protected characteristic
- NEVER use sarcasm that could be misread across cultures
- NEVER make puns unless the user has EXPLICITLY enabled puns for a specific contact in their contact profile
- NEVER assume cultural norms — default to professional-neutral
- NEVER discuss politics, religion, or controversial topics even if the other person brings it up — deflect politely
- NEVER use slang, idioms, or colloquialisms that may not translate across cultures
- NEVER use passive-aggressive language
- NEVER make assumptions about someone's background, beliefs, or preferences
- If humor is allowed for a contact, keep it strictly work-related and light

**Operational Safety:**
- NEVER send a message without showing the user the exact text and getting confirmation (unless contact is in auto_reply_contacts)
- NEVER reply to messages from unknown/new contacts without asking the user first
- NEVER forward, quote, or reference messages from one person in a conversation with another person
- NEVER share personal information about one contact with another
- NEVER escalate tone — if someone is angry or upset, respond calmly and neutrally
- NEVER impersonate — the reply represents the user, in their authentic style
- NEVER merge PRs, trigger deployments, delete resources, or take any destructive action
- NEVER access or reference messages older than the fetched history window
- If unsure about ANYTHING, hold the message and ask the user

**Group Chat Safety:**
- NEVER use inside jokes or references from 1:1 chats in group contexts
- NEVER over-mention (@) people — only mention when specifically needed
- NEVER reply on behalf of the user to sensitive topics (HR, performance, compensation)
- In mixed-culture groups, always use the most neutral/formal tone appropriate

---

## CLI Commands

### `teams-cli agent` Subcommands

```
teams-cli agent init                    Bootstrap persona and contact profiles
teams-cli agent run                     One-shot: check unreads, process, confirm, send
teams-cli agent watch                   Continuous polling mode (optional)
teams-cli agent learn                   Analyze recent chats, update profiles
teams-cli agent contacts                List all known contacts
teams-cli agent contacts show <email>   Show a specific contact profile
teams-cli agent contacts edit <email>   Open contact profile in $EDITOR
teams-cli agent groups                  List known group chats
teams-cli agent groups show <chat-id>   Show group profile
teams-cli agent actions                 List available actions
teams-cli agent config                  Show current configuration
teams-cli agent config edit             Open config in $EDITOR
teams-cli agent status                  Show agent health and stats
```

### Command Details

#### `teams-cli agent init`

Bootstraps the entire `~/.teams-agent/` directory:

```
1. Create directory structure
2. Fetch user info:        teams-cli me --format json
3. Fetch all chats:        teams-cli chats list --format json
4. For each chat, fetch last 50 messages
5. Call claude -p with ALL messages to generate:
   - persona.md (extracted from user's own messages)
   - contacts/*.md (one per frequent contact)
   - groups/*.md (one per group chat)
   - patterns/*.md (detected response patterns)
6. Write safety-rules.md (hardcoded, not generated)
7. Write default config.yaml
8. Write default actions/ (pr-review.md, ci-status.md)
```

**Prompt for persona extraction:**
```
Analyze these Teams messages sent by [user]. Extract:
1. Communication style (formal/casual/mixed)
2. Common phrases they actually use (exact quotes)
3. Response patterns by context (quick reply vs detailed)
4. Tone variations by recipient
5. Things they never say or avoid

Output as a structured persona.md file.
```

**Prompt for contact extraction:**
```
Analyze the conversation between [user] and [contact]. Extract:
1. Relationship type and closeness level
2. Communication pattern (who initiates, message length, topics)
3. Tone (casual/formal/mixed)
4. Sample exchanges that capture the dynamic
5. Cultural context if apparent (otherwise mark as "neutral/unknown")

Output as a structured contact profile.
```

#### `teams-cli agent run`

One-shot execution:

```
1. Load config, persona, safety rules
2. Fetch unreads:         teams-cli chats list --unread --format json
3. For each unread chat:
   a. Identify contact/group from contacts/ or groups/
   b. Fetch history:      teams-cli messages list <id> -n 30 --format json
   c. Build context prompt (see Prompt Assembly below)
   d. Call claude -p to:
      - Detect intent
      - Execute actions if needed (returns action commands)
      - Generate reply draft
   e. Display to user:
      - Who sent the message
      - Detected intent
      - Actions taken (if any)
      - Draft reply
   f. Prompt: [S]end / [E]dit / [N]o reply / [Q]uit
4. Log interactions to logs/
5. Optionally update profiles (if auto_update_profiles: true)
```

#### `teams-cli agent learn`

Re-analyzes recent conversations to update profiles:

```
1. For each contact in contacts/:
   a. Fetch recent messages from their chat
   b. Compare with existing profile
   c. Call claude -p to identify:
      - New phrases or patterns
      - Tone shifts
      - New topics
   d. Update contact profile (merge, don't replace)
2. For each group in groups/:
   a. Fetch recent messages
   b. Update dynamics, detect new members
   c. Update relationship diagram
3. Update persona.md if user's own patterns have shifted
```

---

## Prompt Assembly

When processing a message, the agent assembles the prompt in this order:

```
┌─────────────────────────────────────────────┐
│  1. SYSTEM: safety-rules.md (ALWAYS first)  │
├─────────────────────────────────────────────┤
│  2. PERSONA: persona.md                     │
├─────────────────────────────────────────────┤
│  3. CONTACT/GROUP: alice.md or backend.md   │
├─────────────────────────────────────────────┤
│  4. PATTERNS: relevant patterns/*.md        │
├─────────────────────────────────────────────┤
│  5. ACTIONS: relevant actions/*.md          │
│     (only if intent matches an action)      │
├─────────────────────────────────────────────┤
│  6. CONVERSATION HISTORY: last N messages   │
│     from teams-cli messages list            │
├─────────────────────────────────────────────┤
│  7. INSTRUCTION: what to do                 │
│     - Detect intent                         │
│     - Execute action (if applicable)        │
│     - Draft reply matching persona+contact  │
│     - Output structured response            │
└─────────────────────────────────────────────┘
```

### Prompt Template

```
You are acting as [User Name]'s Teams messaging assistant.

=== SAFETY RULES (NON-NEGOTIABLE) ===
[contents of safety-rules.md]

=== MY COMMUNICATION STYLE ===
[contents of persona.md]

=== ABOUT THE SENDER ===
[contents of contacts/[sender].md OR "Unknown contact — use professional-neutral tone"]

=== GROUP CONTEXT (if group chat) ===
[contents of groups/[group].md]

=== RESPONSE PATTERNS ===
[contents of relevant patterns/*.md]

=== AVAILABLE ACTIONS ===
[list of actions with triggers]

=== CONVERSATION HISTORY ===
[last N messages from teams-cli messages list, formatted as:]
[timestamp] [sender]: [message]
[timestamp] [sender]: [message]
...

=== INSTRUCTIONS ===
1. Read the latest unread message(s)
2. Detect the intent:
   - ACTION_NEEDED: sender is asking for something that requires an external action
     (PR review, CI check, status lookup, etc.)
   - REPLY_ONLY: sender is asking a question or making conversation
   - NO_REPLY: message doesn't need a response (acknowledgment, emoji, "ok", etc.)
3. If ACTION_NEEDED, output the action commands to execute
4. Draft a reply that:
   - Matches my communication style exactly
   - Adjusts tone for this specific contact/group
   - Includes action results if applicable
   - Follows all safety rules
5. If in a group chat and responding to a specific person, mention them appropriately

Output format:
INTENT: [ACTION_NEEDED | REPLY_ONLY | NO_REPLY]
ACTION_COMMANDS: [commands to run, one per line, or NONE]
DRAFT_REPLY: [the reply text, or NONE if NO_REPLY]
CONFIDENCE: [high | medium | low]
NOTES: [any concerns or things the user should know]
```

---

## Group Chat Mention Logic

```
┌─────────────────────────────────────────────┐
│           Group Mention Decision Tree        │
│                                             │
│  Is this a reply to a specific person?       │
│  ├── YES                                    │
│  │   └── Mention them: "@Alice ..."         │
│  └── NO                                     │
│      └── Is this a general message?          │
│          ├── YES                             │
│          │   └── No mention, just post       │
│          └── NO (directed at multiple)       │
│              └── Mention relevant people     │
│                                             │
│  Special rules:                              │
│  - Never mention EVERYONE unless critical    │
│  - Mention managers only for decisions       │
│  - Mention new members to include them       │
│  - Follow group-specific mention rules       │
│    from groups/*.md                          │
└─────────────────────────────────────────────┘
```

### Mention Format

Teams uses `<at>` tags for mentions in HTML messages:

```html
<at id="8:orgid:user-object-id">Display Name</at> message text here
```

The agent must:
1. Look up the user's MRI from their contact profile
2. Format the mention as an `<at>` tag with correct ID
3. Include the mention in the HTML message body

---

## Claude Code Skill (`/teams`)

Located at `~/.claude/skills/teams.md`, this skill enables interactive Teams usage within Claude Code sessions.

### Skill Capabilities

```
/teams
  │
  ├── Check unreads → summarize, offer to reply
  ├── Send message → draft, confirm, send
  ├── Search messages → find and summarize
  ├── PR review flow → read PR, review, reply to requester
  ├── Status check → check CI/git, reply with status
  ├── Manage profiles → view/edit contacts, groups, persona
  └── Learn → trigger profile updates from recent history
```

### Skill vs CLI Agent

| Feature | `/teams` Skill | `teams-cli agent run` |
|---------|----------------|----------------------|
| Environment | Claude Code session | Standalone terminal |
| AI Model | Current Claude Code model | `claude -p` (pipe mode) |
| Tools available | All Claude Code tools (Bash, Read, Edit, etc.) | Only CLI tools |
| Interactivity | Fully conversational | Prompt-based (S/E/N/Q) |
| Actions | Can execute ANY tool | Limited to predefined actions |
| Best for | Interactive sessions, complex tasks | Quick batch processing |

---

## Implementation Phases

### Phase 1: Foundation
- [ ] Create `~/.teams-agent/` directory structure
- [ ] Write `safety-rules.md` (hardcoded)
- [ ] Write `config.yaml` template
- [ ] Write persona.md, contact, group, pattern, action templates
- [ ] Create Claude Code skill `~/.claude/skills/teams.md`

### Phase 2: Agent CLI Commands
- [ ] `teams-cli agent init` — bootstrap from chat history
- [ ] `teams-cli agent contacts` — list/show contacts
- [ ] `teams-cli agent groups` — list/show groups
- [ ] `teams-cli agent actions` — list available actions
- [ ] `teams-cli agent config` — show/edit config
- [ ] `teams-cli agent status` — health check

### Phase 3: Agent Runtime
- [ ] `teams-cli agent run` — one-shot processing
- [ ] Prompt assembly pipeline
- [ ] Intent detection
- [ ] Action execution (PR review, CI check)
- [ ] Draft + confirm + send flow
- [ ] Conversation logging

### Phase 4: Learning
- [ ] `teams-cli agent learn` — profile updates
- [ ] Pattern extraction from new conversations
- [ ] Relationship evolution tracking
- [ ] Group dynamics updates

### Phase 5: Watch Mode (Optional)
- [ ] `teams-cli agent watch` — continuous polling
- [ ] Configurable poll interval
- [ ] Desktop notifications for held messages
- [ ] Auto-reply for opted-in contacts

---

## Example Flows

### Flow 1: PR Review Request

```
Alice (Teams, 1:1): "hey can you review github.com/company/api/pull/87?"

Agent detects:
  Intent: ACTION_NEEDED (PR review)
  Contact: Alice (close teammate, casual tone)
  Action: pr-review

Agent executes:
  $ gh pr view 87 --json title,body,changedFiles,additions,deletions
  $ gh pr diff 87

Agent reviews code, then:
  $ gh pr review 87 --comment --body "Looks good overall! A couple of things:
    1. The error handling in auth.go:45 could use a wrapped error
    2. Consider adding a test for the edge case in parser.go
    Otherwise clean and well-structured."

Agent drafts Teams reply:
  "reviewed it, left a couple comments. mostly looks good,
   just some error handling and a test case suggestion"

User confirms → Agent sends via teams-cli messages send
```

### Flow 2: Group Chat Status Request

```
#backend-team:
  Bob: "what's the status on the auth migration? @Kamrul"

Agent detects:
  Intent: ACTION_NEEDED (status check)
  Contact: Bob (manager, semi-formal)
  Group: backend-team (professional-casual, Bob sets tone)
  Action: git status check

Agent executes:
  $ git log --oneline -10 (in the relevant repo)
  $ gh pr list --author kamrul

Agent drafts:
  "@Bob Auth migration is on track. Completed token refresh last week,
   currently working on middleware swap. PR #92 is up for the first
   part. Targeting end of week for the full cutover."

User confirms → Agent sends (with @mention HTML tag for Bob)
```

### Flow 3: Casual Chat (Cross-Cultural)

```
Carol (Teams, 1:1): "hi Kamrul, I was wondering if you could help
me understand how the deployment process works?"

Agent detects:
  Intent: REPLY_ONLY
  Contact: Carol (new colleague, professional tone, cultural context: neutral)
  No action needed

Agent drafts:
  "Hi Carol, sure I can help with that. The deployment process goes
   through our CI pipeline — when a PR merges to main, GitHub Actions
   runs tests and deploys to staging automatically. For prod, we
   trigger a manual workflow. Happy to walk you through it in more
   detail if you'd like."

  (Note: more formal than with Alice, welcoming tone for new colleague,
   no slang, no humor, clear and helpful)

User confirms → Agent sends
```

### Flow 4: Message That Needs No Reply

```
Alice (Teams): "ok cool thanks"

Agent detects:
  Intent: NO_REPLY
  Reason: Acknowledgment message, no question or request

Agent shows:
  "Alice said 'ok cool thanks' — no reply needed. Skip? [Y/n]"
```

---

## Security Considerations

1. **Token Security**: Agent uses `teams-cli` which manages tokens at `~/.config/teams-cli/`. Agent never accesses tokens directly.
2. **API Key Security**: `claude -p` uses `ANTHROPIC_API_KEY` from environment. Never stored in agent config.
3. **Log Security**: Conversation logs stored locally at `~/.teams-agent/logs/`. User's responsibility to secure.
4. **Profile Security**: Contact profiles may contain relationship info. Stored locally, never transmitted beyond `claude -p` prompts.
5. **Action Scope**: All actions are read-only or comment-only. No destructive operations (merge, delete, deploy) are defined.
