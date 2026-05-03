# Claude Skills

Reusable skills for [Claude Code](https://docs.anthropic.com/en/docs/claude-code).

## Skills

| Skill | Description |
|-------|-------------|
| [notify](notify/) | Telegram notifications with optional inline-button reply listening |

## Usage

Copy a skill directory into `~/.claude/skills/` or symlink it:

```bash
ln -s ~/dev/claude-skills/notify ~/.claude/skills/notify
```

Then invoke via `/notify` in Claude Code.

## Requirements

- `jq` (pre-installed on macOS)
- `MCP_BOT_TOKEN` env var with your Telegram bot token
- `CHAT_ID` configured in the skill (default: hardcoded)

## License

MIT
