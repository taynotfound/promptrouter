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

## Score mode: your own levels and thresholds

Tier mode is three fixed buckets. Score mode lets the judge emit a 0-100
difficulty score and hands the task to a level *you* define. Set
`judge.mode: score` and list `levels` instead of `tiers`.

```yaml
judge:
  model: qwen2.5:7b-instruct
  mode: score

levels:
  - name: local          # 0-39
    min_score: 0
    chain:
      - engine: ollama
        model: qwen3-coder:30b
  - name: mid            # 40-74
    min_score: 40
    chain:
      - engine: openrouter
        model: anthropic/claude-3.5-sonnet
      - engine: ollama
        model: qwen3-coder:30b
  - name: expert         # 75-100
    min_score: 75
    chain:
      - engine: hermes
        model: claude-opus-4.8

defaults:
  ollama_url: "http://localhost:11434"
  log_file: "~/.promptrouter/routes.jsonl"
```

Rules:

- The highest level whose `min_score` the score clears wins. A score below every
  threshold falls to the lowest level, so routing always resolves.
- Give the first level `min_score: 0`. Thresholds must be unique and in 0-100.
- Any number of levels, one to ten. Two is fine (local vs cloud); more gives you
  finer control over where money starts being spent.
- Each level has its own engine chain with its own fallbacks, exactly like tiers.

Force a decision to test a level: `route --score 90 "..."`, or by level name
with `route --tier expert "..."`. Measure how your judge scores your tasks with
`route bench` (see [benchmarks.md](benchmarks.md)).

## Engines

| Engine | Where it runs | Auth |
| --- | --- | --- |
| `ollama` | local | none |
| `openai` | cloud or self-hosted; OpenAI and any OpenAI-compatible API | `OPENAI_API_KEY` by default, or a per-engine `key_env`; none needed for a local server |
| `anthropic` | cloud | `ANTHROPIC_API_KEY` (from `secrets.env` or env) |
| `openrouter` | cloud | `OPENROUTER_API_KEY` (from `secrets.env` or env) |
| `hermes` | cloud, via the hermes CLI | your existing hermes login |

### The `openai` engine covers most providers

The `openai` kind speaks the standard `/chat/completions` protocol, so a single
engine type reaches OpenAI, Groq, Together, DeepSeek, Fireworks, a local vLLM or
LM Studio server, and anything else that implements the same API. Two optional
per-engine fields point it at the right place:

```yaml
- engine: openai
  model: llama-3.3-70b-versatile
  base_url: "https://api.groq.com/openai/v1"   # omit for OpenAI itself
  key_env: GROQ_API_KEY                          # omit to use OPENAI_API_KEY
```

`base_url` sets the API root; `key_env` names the environment variable that
holds the key, so several `openai` engines can each use their own token. A local
server (a `base_url` on `localhost` or `127.0.0.1`) needs no key at all.

## Seeing where your tasks go

Set `log_file` and every route appends one JSON line. Summarize it:

```sh
route --stats
```
