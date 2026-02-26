---
summary: "Use OpenAcosmi Zen (curated models) with OpenAcosmi"
read_when:
  - You want OpenAcosmi Zen for model access
  - You want a curated list of coding-friendly models
title: "OpenAcosmi Zen"
---

# OpenAcosmi Zen

OpenAcosmi Zen is a **curated list of models** recommended by the OpenAcosmi team for coding agents.
It is an optional, hosted model access path that uses an API key and the `openacosmi` provider.
Zen is currently in beta.

## CLI setup

```bash
openacosmi onboard --auth-choice openacosmi-zen
# or non-interactive
openacosmi onboard --openacosmi-zen-api-key "$OPENACOSMI_API_KEY"
```

## Config snippet

```json5
{
  env: { OPENACOSMI_API_KEY: "sk-..." },
  agents: { defaults: { model: { primary: "openacosmi/claude-opus-4-6" } } },
}
```

## Notes

- `OPENACOSMI_ZEN_API_KEY` is also supported.
- You sign in to Zen, add billing details, and copy your API key.
- OpenAcosmi Zen bills per request; check the OpenAcosmi dashboard for details.
