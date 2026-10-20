# Claude Code CLI

The `claude` provider runs one non-interactive `claude --print` command per email. It disables all tools, MCP servers and user/project settings, does not persist the session, and requests the score as structured JSON output.

```yaml
providers:
  local-claude:
    type: claude
    config:
      model: haiku # optional; omit to use the CLI default
      maxsize: 100000
      timeout: 180s
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | no | Executable name or path; defaults to `claude` |
| `model` | string | no | Claude model alias or ID; omitted by default |
| `maxsize` | integer | yes | Maximum message content size in bytes |
| `timeout` | duration | no | Command timeout; defaults to `180s` |

The Docker image includes the Claude Code CLI. To use a Claude subscription, create a long-lived token on a machine with a browser and pass it to the container through the `CLAUDE_CODE_OAUTH_TOKEN` environment variable, for example via a `.env` file next to `docker-compose.yml`:

```console
claude setup-token
echo "CLAUDE_CODE_OAUTH_TOKEN=<token>" > .env
chmod 600 .env
docker compose up -d --build
```

Treat the token as a password. Never commit it or expose it in logs.

Check the configured provider without connecting to IMAP or moving mail:

```console
docker compose run --rm imap-spam-cleaner --check-provider local-claude
```
