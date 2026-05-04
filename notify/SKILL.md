---
name: notify
description: Send Telegram notification and wait for user reply. Always attaches a Reply button and polls for response.
argument-hint: [message] [--buttons "Label:data,..." --timeout N]
allowed-tools: Bash
---

# Notify Skill

Send a Telegram message with inline buttons and wait for the user's reply.

## Usage

```
go run ~/.claude/skills/notify/notify.go [flags] "message text"
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--buttons` | *(none)* | Comma-separated `Label:data` pairs |
| `--timeout` | `120` | Seconds to wait for reply |
| `--chat` | `$CLAUDE_CODE_NOTIFY_CHAT_ID` | Override chat ID |

Requires `$CLAUDE_CODE_NOTIFY_BOT_TOKEN` and `$CLAUDE_CODE_NOTIFY_CHAT_ID` in the environment.

`✍️ Reply` button is always appended automatically.

## Stdout

```
SENT: ok                              # message delivered
REPLY: <value>                        # button callback_data or typed text
TIMEOUT: no response within 120s      # no reply received
ERROR: <msg>                          # fatal error, exit 1
```

## When to Use

1. **After every deploy** — summarize what was deployed
2. **When stuck or blocked** — need user input
3. **When you need confirmation** — use `--buttons`
4. **Long task finished** — notify completion

## Message Formatting

Use HTML: `<b>bold</b>`, `<code>mono</code>`. Prefix with project name (from git). Emoji: 🚀 deploy, 🚨 stuck, ✅ done, ❌ failed. Bullet points with •.

Optionally add a fun/humorous extra button (🐐, 🫡, 🔥) when the vibe fits — use sparingly.

## Examples

```bash
# Simple notification (Reply button auto-added)
go run ~/.claude/skills/notify/notify.go "✅ <b>myapp</b> — deploy complete"

# With action buttons
go run ~/.claude/skills/notify/notify.go --buttons "👍 Yes:yes,👎 No:no" "Deploy to prod?"

# Custom timeout
go run ~/.claude/skills/notify/notify.go --timeout 300 "🚨 <b>myapp</b> — stuck, need input"
```
