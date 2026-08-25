# NVIDIA

The `nvidia` provider uses NVIDIA's OpenAI-compatible chat completions endpoint.

```yaml
providers:
  kimi:
    type: nvidia
    config:
      apikey: your-nvidia-api-key
      model: moonshotai/kimi-k2.5
      maxsize: 100000
      maxtokens: 16384
      temperature: 1.0
      topp: 1.0
      thinking: "true"
      timeout: 180s
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `apikey` | string | yes | NVIDIA API key |
| `url` | string | no | API endpoint override |
| `model` | string | no | Model name; defaults to `moonshotai/kimi-k2.5` |
| `maxsize` | integer | yes | Maximum message content size in bytes |
| `maxtokens` | integer | no | Maximum response tokens; defaults to `16384` |
| `temperature` | number | no | Sampling temperature; defaults to `1.0` |
| `topp` | number | no | Top-p sampling value; defaults to `1.0` |
| `thinking` | boolean | no | Thinking mode; defaults to `true` |
| `timeout` | duration or seconds | no | Request timeout; defaults to `180s` |
