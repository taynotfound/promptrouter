# promptrouter

![ci](https://github.com/taynotfound/promptrouter/actions/workflows/ci.yml/badge.svg)
![license](https://img.shields.io/badge/license-MIT-blue)
![go](https://img.shields.io/badge/go-1.26%2B-00ADD8)

Stop paying cloud prices for `rename this variable`.

promptrouter judges each coding task with a fast local model, then runs the easy
work on your own GPU (free, private) and only reaches for the cloud when the task
actually earns it. One static binary, one YAML file, no runtime to install.

```
$ route --explain "rename the variable foo to bar"
[judge] EASY  trivial rename
would use ollama/qwen3-coder:30b

$ route --explain "design a lock-free MPMC queue with hazard pointers"
[judge] EXPERT  lock-free concurrency, easy to get wrong
would use hermes/claude-opus-4.8
```

## How it works

1. A small local model (default `qwen2.5:7b-instruct`) scores the task EASY, HARD, or EXPERT in about 0.3s.
2. Each tier is an ordered chain of engines. Routing tries the first, falls back on failure.
3. EASY stays local and free. HARD and EXPERT go to the cloud model you picked.

Everything lives in `models.yaml`. Nothing is hardcoded.

## Install

```bash
# one line, picks the right prebuilt binary
curl -fsSL https://raw.githubusercontent.com/taynotfound/promptrouter/main/install.sh | bash
```

Per distro:

```bash
yay -S promptrouter                                    # Arch (AUR)
sudo dpkg -i promptrouter_*_amd64.deb                  # Debian, Ubuntu
sudo rpm -i promptrouter_*_amd64.rpm                   # Fedora, RHEL
nix run github:taynotfound/promptrouter -- --version   # NixOS
go install github.com/taynotfound/promptrouter/cmd/route@latest  # from source
```

The `.deb` and `.rpm` are built on every tagged release. NixOS users can also add
the flake as an input.

## Use

```bash
route "add a null check to this parser"     # judge, route, run
route --dry "design a billing API"          # show the decision only
route --explain "debug this flaky test"     # show the judge reasoning
route --tier EXPERT "..."                   # force a tier
route --engine ollama "..."                 # force an engine in the tier
route --json "..."                          # machine readable
route --stats                               # summarize the routing log
```

Config is found at `~/.config/promptrouter/models.yaml`, then `/etc/promptrouter/models.yaml`,
then `./models.yaml`. Override with `--config` or `PROMPTROUTER_CONFIG`.

## Configure

`models.yaml` is the whole product. Tiers map to ordered engine chains:

```yaml
judge:
  model: qwen2.5:7b-instruct

tiers:
  EASY:
    chain:
      - engine: ollama
        model: qwen3-coder:30b
  HARD:
    chain:
      - engine: hermes
        model: claude-sonnet-5
      - engine: ollama        # fallback if the cloud call fails
        model: qwen3-coder:30b

defaults:
  fallback_tier: HARD          # used when the judge is unsure
  ollama_url: "http://localhost:11434"
  max_retries: 1               # retry an engine on an empty or transient failure
  log_file: "~/.promptrouter/routes.jsonl"
```

Engines:

- `ollama` local models over the Ollama HTTP API. Free.
- `hermes` cloud models through the `hermes -z` CLI.
- `openrouter` cloud models over the OpenRouter API (`OPENROUTER_API_KEY`).

## Measuring the savings

Turn on `log_file` and use `route` for real work. Then:

```
$ route --stats
routes: 214  ok: 214  local (free): 171 (79% of successful)
by tier:   EASY 171, HARD 34, EXPERT 9
by engine: ollama/qwen3-coder:30b 171, hermes/claude-sonnet-5 34, hermes/claude-opus-4.8 9
```

That percentage is the point. It is your workload, not a benchmark.

## Judge accuracy, honestly

On a 24 task hand labeled set the judge is right about 92% of the time:

- EASY and HARD: near perfect. The split that saves money is reliable.
- EXPERT: about 4 of 6. A 7B judge sometimes calls a genuinely expert task HARD.

The key point: a misjudged EXPERT still routes to the cloud, so answer quality
holds. The only cost is picking Sonnet over Opus on a hard call. It never drops
hard work onto a weak local model. Want better EXPERT recall? Point `judge.model`
at a 14B model and pay a little more latency.

## Build

```bash
go build -o route ./cmd/route
go test ./...
```

## Compare

There are good routers out there. This one is the local first, self hosted corner.

| | promptrouter | OpenRouter Auto | RouteLLM |
| --- | --- | --- | --- |
| Routes to your local GPU | yes | no | DIY |
| Runs fully self hosted | yes | no | yes |
| Code stays on your machine | yes | no | yes |
| Setup | one YAML | one API key | Python plus a trained model |

If you do not run local models, OpenRouter is easier and that is fine.
promptrouter is for the case they do not serve: keep the easy work on your own box.

## License

MIT. See [LICENSE](LICENSE).
