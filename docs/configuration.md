# Configuration

## The fastest path: `route init`

```sh
route init
```

The wizard checks your prerequisites, lists the Ollama models you already have,
lets you pick a local model and an optional cloud provider, collects any API
token, and writes a valid config. Rerun it any time to change providers.

## Where files live

| File | Purpose | Default location |
| --- | --- | --- |
| `models.yaml` | tiers, model chains, tunables. Safe to share. | `~/.config/promptrouter/models.yaml` |
| `secrets.env` | API tokens, `KEY=value`, chmod 600. Never commit. | `~/.config/promptrouter/secrets.env` |

Override with `PROMPTROUTER_CONFIG` and `PROMPTROUTER_SECRETS`, or pass
`--config PATH` on the command line. Real environment variables always win over
the secrets file.

## How routing works

A task is judged into a tier (`EASY`, `HARD`, `EXPERT`). Each tier has an
ordered chain of engines. promptrouter tries them top to bottom and stops at the
first one that answers, so you put your preferred engine first and a fallback
below it.

```yaml
judge:
  model: qwen2.5:7b-instruct        # small, fast classifier

tiers:
  EASY:
    chain:
      - engine: ollama              # free, local, handles the bulk
        model: qwen3-coder:30b
  HARD:
    chain:
      - engine: openrouter          # cloud first for the harder work
        model: anthropic/claude-3.5-sonnet
      - engine: ollama              # local fallback if the cloud is down
        model: qwen3-coder:30b
  EXPERT:
    chain:
      - engine: hermes
        model: claude-opus-4.8

defaults:
  fallback_tier: HARD               # used if the judge returns junk
  ollama_url: "http://localhost:11434"
  max_retries: 1
  retry_backoff: 0.5
  log_file: "~/.promptrouter/routes.jsonl"
```

## Engines

| Engine | Where it runs | Auth |
| --- | --- | --- |
| `ollama` | local | none |
| `openrouter` | cloud | `OPENROUTER_API_KEY` (from `secrets.env` or env) |
| `hermes` | cloud, via the hermes CLI | your existing hermes login |

## Seeing where your tasks go

Set `log_file` and every route appends one JSON line. Summarize it:

```sh
route --stats
```
