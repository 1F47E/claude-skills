---
name: notify
description: Send Telegram notification with optional reply listening. Use --reply to wait for button press or text response.
argument-hint: [message] [--reply ["Label:data" ...]]
allowed-tools: Bash
---

# Notify Skill — Telegram Notifications

## Config

```
BOT_TOKEN=$MCP_BOT_TOKEN
CHAT_ID=127933519
API=https://api.telegram.org/bot$BOT_TOKEN
```

`MCP_BOT_TOKEN` must be set in the environment (e.g., `~/.zshenv` or `~/.claude/.env`).

Detect project name from the current git repo: `basename $(git rev-parse --show-toplevel)`

## When to Use

1. **After every deploy** — summarize what was deployed
2. **When stuck** — can't continue, need user input, blocked by an error you can't resolve
3. **When you need user confirmation** — use `--reply` to wait for their response

## Argument Parsing

1. If `$ARGUMENTS` contains `--reply`, split on it:
   - Everything **before** `--reply` = message text
   - Everything **after** `--reply` = button definitions (optional)
2. If no `--reply` → fire-and-forget mode (Mode 1)
3. If `--reply` present → reply listening mode (Mode 2)

Button definitions are space-separated `"Label:callback_data"` pairs, e.g. `"staging:stg" "production:prod"`.

## Mode 1: Fire-and-Forget (no --reply)

If `$ARGUMENTS` is provided (without `--reply`), use it as the message text (prepend project prefix).

If no arguments at all, auto-generate a deploy message:
1. Get the project name: `basename $(git rev-parse --show-toplevel)`
2. Get the latest git tag: `git tag --sort=-v:refname | head -1`
3. Get the commit message for HEAD: `git log -1 --format=%s`
4. Format as deploy notification

### Command

```bash
curl -s -X POST "$API/sendMessage" \
  -d chat_id=$CHAT_ID \
  -d parse_mode=HTML \
  --data-urlencode text="<message>"
```

Display the result: confirm success or show error.

## Mode 2: Send + Wait for Reply (--reply present)

### Step 1: Determine buttons

If button definitions were provided after `--reply`, parse them. Otherwise use defaults.

**Defaults** (when no buttons specified):
```
Row 1: 👍 Yes (data: yes), 👎 No (data: no)
Row 2: ⏭ Skip (data: skip), ✍️ Custom (data: __custom__)
```

**Custom buttons** (when provided):
- Parse each `"Label:data"` pair
- Always append `✍️ Custom` (`__custom__`) as the last button
- Arrange in rows of 2-3 buttons

### Step 2: Build keyboard JSON

Construct the `inline_keyboard` array. Example for defaults:

```json
[[{"text":"👍 Yes","callback_data":"yes"},{"text":"👎 No","callback_data":"no"}],[{"text":"⏭ Skip","callback_data":"skip"},{"text":"✍️ Custom","callback_data":"__custom__"}]]
```

### Step 3: Delete webhook and send message

```bash
# Prevent 409 conflict with any active webhook
curl -s "$API/deleteWebhook" > /dev/null

# JSON-escape the message text
ESCAPED_TEXT=$(printf '%s' "$MESSAGE" | jq -Rs '.')

# Send with inline keyboard (must use JSON body for reply_markup)
MSG_RESULT=$(curl -s -X POST "$API/sendMessage" \
  -H "Content-Type: application/json" \
  -d "{
    \"chat_id\": \"$CHAT_ID\",
    \"text\": $ESCAPED_TEXT,
    \"parse_mode\": \"HTML\",
    \"reply_markup\": {
      \"inline_keyboard\": $KEYBOARD_JSON
    }
  }")

echo "$MSG_RESULT" | jq -r '.ok'
```

### Step 4: Flush stale updates

```bash
ALLOWED='allowed_updates=%5B%22message%22%2C%22callback_query%22%5D'
FLUSH=$(curl -s "$API/getUpdates?offset=-1&limit=1&$ALLOWED")
LAST_ID=$(echo "$FLUSH" | jq -r '.result[-1].update_id // empty')
if [ -n "$LAST_ID" ]; then
  OFFSET=$((LAST_ID + 1))
else
  OFFSET=0
fi
```

### Step 5: Poll for reply

Poll with long-polling. Maximum 4 iterations × 30s = 120s timeout.

**IMPORTANT:** Do NOT use `for row in $(jq -c '.result[]')` — emojis and special chars in JSON break bash word-splitting. Use index-based access instead.

```bash
CUSTOM_MODE=false
REPLY=""

for i in 1 2 3 4; do
  UPDATES=$(curl -s "$API/getUpdates?offset=$OFFSET&timeout=30&$ALLOWED")
  COUNT=$(echo "$UPDATES" | jq '.result | length')

  if [ "$COUNT" = "0" ] || [ "$COUNT" = "null" ]; then
    continue
  fi

  idx=0
  while [ $idx -lt $COUNT ]; do
    UPD_ID=$(echo "$UPDATES" | jq -r ".result[$idx].update_id")
    OFFSET=$((UPD_ID + 1))

    # Check callback_query (button press)
    CB_CHAT=$(echo "$UPDATES" | jq -r ".result[$idx].callback_query.message.chat.id // empty")
    CB_ID=$(echo "$UPDATES" | jq -r ".result[$idx].callback_query.id // empty")
    CB_DATA=$(echo "$UPDATES" | jq -r ".result[$idx].callback_query.data // empty")

    if [ -n "$CB_ID" ] && [ "$CB_CHAT" = "$CHAT_ID" ]; then
      curl -s "$API/answerCallbackQuery?callback_query_id=$CB_ID" > /dev/null

      if [ "$CB_DATA" = "__custom__" ]; then
        curl -s -X POST "$API/sendMessage" \
          -d chat_id=$CHAT_ID \
          --data-urlencode text="✍️ Type your reply..." > /dev/null
        CUSTOM_MODE=true
        idx=$((idx + 1))
        continue
      fi

      REPLY="$CB_DATA"
      break 2
    fi

    # Check text message (only in custom mode)
    MSG_CHAT=$(echo "$UPDATES" | jq -r ".result[$idx].message.chat.id // empty")
    TEXT=$(echo "$UPDATES" | jq -r ".result[$idx].message.text // empty")
    if [ -n "$TEXT" ] && [ "$MSG_CHAT" = "$CHAT_ID" ] && [ "$CUSTOM_MODE" = true ]; then
      REPLY="$TEXT"
      break 2
    fi

    idx=$((idx + 1))
  done
done

if [ -n "$REPLY" ]; then
  # Acknowledge receipt in the chat so user knows it landed
  curl -s -X POST "$API/sendMessage" \
    -d chat_id=$CHAT_ID \
    --data-urlencode text="👌 Got it: $REPLY" > /dev/null
  echo "REPLY: $REPLY"
else
  echo "REPLY: TIMEOUT (no response within 120s)"
fi
```

### Step 6: Use the reply

After the poll completes, the reply value is printed as `REPLY: <value>`.
Use this to make decisions:
- `yes` / `no` / `skip` — act accordingly
- `TIMEOUT` — inform the user or retry
- Any custom text — use as free-form input

## Message Formats

Use **HTML** parse mode. Use `--data-urlencode text=` (not `-d text=`) for fire-and-forget mode.

**Deploy notification:**
```
🚀 <b>project</b> · <code>tag</code>

• change 1
• change 2
```

**Stuck notification:**
```
🚨 <b>project</b> — stuck

description of what's blocking
```

**Success (long task done):**
```
✅ <b>project</b> — done

• what was completed
```

**Failure:**
```
❌ <b>project</b> — failed

what went wrong
```

## Button Layout Rules

- 2 buttons → single row
- 3-4 buttons → single row or 2+2
- 5+ buttons → rows of 2-3
- `✍️ Custom` always goes last

## Notes

- Always prefix messages with project name (auto-detected from git repo)
- Keep messages concise: version + 1-2 line summary
- Use HTML parse mode: `<b>bold</b>`, `<i>italic</i>`, `<code>mono</code>`
- Use emoji: 🚀 deploy, 🚨 stuck, ✅ success, ❌ failure
- Use bullet points (•) for multiple items
- In reply mode, the `__custom__` callback is reserved — never use it as a regular button data value
- The poll loop requires `jq` (pre-installed on macOS)
- If another Telegram bot process is polling, `deleteWebhook` alone may not fix 409 — kill the other process first
