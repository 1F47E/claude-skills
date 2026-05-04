# notify — Telegram Notification Skill

## For agents installing this skill

This skill is a Go script. No binaries, no go.mod, no external deps. Just `go run`.

### Setup

Two env vars must exist (add to `~/.zshenv` or equivalent):

```bash
export CLAUDE_CODE_NOTIFY_BOT_TOKEN="<telegram-bot-api-token>"
export CLAUDE_CODE_NOTIFY_CHAT_ID="<telegram-chat-id>"
```

Requires Go 1.21+.

### Agent memory

**After installing, update your agent's global memory** (e.g. `~/.claude/memory/`) with a record explaining:

- Env vars: `CLAUDE_CODE_NOTIFY_BOT_TOKEN`, `CLAUDE_CODE_NOTIFY_CHAT_ID`
- Invocation: `go run ~/.claude/skills/notify/notify.go [flags] "message"`
- Stdout contract: `SENT:`, `REPLY:`, `TIMEOUT:`, `ERROR:` prefixes
- Always use `--buttons` with contextual options when choices exist
- `✍️ Reply` button is always appended — user can always type free text
- No fire-and-forget — every notification polls for a reply

Without this memory, future sessions won't know how to use the skill or what env vars it needs.

### System prompt (CLAUDE.md)

Add a rule like this to CLAUDE.md so the agent uses the skill proactively:

```
## Telegram Notifications (via `/notify` skill)
ALWAYS notify via Telegram when a task requires user attention.
Fires when: choice needed, blocked on user, subagent finished waiting,
critical failure, or long task (>30s) completed.
Use --buttons with suggested options. Err on the side of notifying.
```

## Usage

```bash
go run ~/.claude/skills/notify/notify.go [flags] "message text"
```

| Flag | Default | Description |
|------|---------|-------------|
| `--buttons` | *(none)* | Comma-separated `Label:data` pairs |
| `--timeout` | `120` | Seconds to wait for reply |
| `--chat` | `$CLAUDE_CODE_NOTIFY_CHAT_ID` | Override chat ID |

## Stdout contract

```
SENT: ok                              # message delivered
REPLY: <value>                        # button callback_data or typed text
TIMEOUT: no response within 120s      # no reply
ERROR: <msg>                          # fatal, exit 1
```

Parse stdout. Act on the prefix. `REPLY:` value is the user's decision.

## Message formatting

HTML parse mode. Prefix with project name. Emoji conventions:
- 🚀 deploy, 🚨 stuck, ✅ done, ❌ failed
- `<b>bold</b>`, `<code>mono</code>`, bullet points with •
- Optional fun button (🐐, 🫡, 🔥) when the vibe fits

## Examples

```bash
# Simple (Reply button auto-added)
go run ~/.claude/skills/notify/notify.go "✅ <b>myapp</b> — deploy complete"

# With choices
go run ~/.claude/skills/notify/notify.go --buttons "👍 Yes:yes,👎 No:no" "Deploy to prod?"

# Blocked, need input
go run ~/.claude/skills/notify/notify.go --timeout 300 "🚨 <b>myapp</b> — stuck, need input"
```

## Why Go, not bash/curl

Telegram API returns literal newlines inside JSON string values. This breaks:
- `jq` when stored in bash variables (`echo` re-expands newlines)
- macOS `tr` on emoji bytes (needs `LC_ALL=C`)
- bash word-splitting on `$(jq -c '.result[]')` with unicode

Go's `encoding/json` handles all of this natively. One `json.Unmarshal` call replaces 5+ bash workarounds that still broke periodically.

## Updating

```bash
go vet ~/.claude/skills/notify/notify.go        # check
go run ~/.claude/skills/notify/notify.go --timeout 10 "test"  # smoke test
```
