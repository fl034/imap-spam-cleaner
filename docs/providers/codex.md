# Codex CLI

The `codex` provider runs one non-interactive `codex exec` command per email. It uses an ephemeral session, a read-only sandbox, an isolated temporary working directory, and a JSON output schema.

```yaml
providers:
  local-codex:
    type: codex
    config:
      model: gpt-5.3-codex # optional; omit to use the CLI default
      maxsize: 100000
      timeout: 180s
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | no | Executable name or path; defaults to `codex` |
| `model` | string | no | Codex model override; omitted by default |
| `maxsize` | integer | yes | Maximum message content size in bytes |
| `timeout` | duration | no | Command timeout; defaults to `180s` |

The Docker image includes the Codex CLI. Create a dedicated writable Codex home so refreshed login tokens persist without exposing the rest of your normal Codex configuration:

```console
mkdir -p ~/.codex-imap-spam-cleaner
cp ~/.codex/auth.json ~/.codex-imap-spam-cleaner/auth.json
chmod 600 ~/.codex-imap-spam-cleaner/auth.json
docker compose up -d --build
```

Treat `auth.json` as a password. Never commit it or expose it in logs.

Check the configured provider without connecting to IMAP or moving mail:

```console
docker compose run --rm imap-spam-cleaner --check-provider local-codex
```
