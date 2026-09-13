<img src="assets/logo.png" alt="promptrouter" width="300">

![ci](https://github.com/taynotfound/promptrouter/actions/workflows/ci.yml/badge.svg)
![license](https://img.shields.io/badge/license-MIT-blue)
![go](https://img.shields.io/badge/go-1.26%2B-00ADD8)
[![Discord](https://img.shields.io/badge/discord-support-5865F2?logo=discord&logoColor=white)](https://discord.gg/bGx9VetCC4)

Stop paying cloud prices for `rename this variable`.

promptrouter judges each coding task with a fast local model, then runs the easy
work on your own GPU (free, private) and only reaches for the cloud when the task
actually earns it. One static binary, one YAML file, no runtime to install.

```
$ route --explain "rename the variable foo to bar"
[judge] score 5 -> local  trivial rename
would use ollama/qwen3-coder:30b

$ route --explain "design a lock-free MPMC queue with hazard pointers"
[judge] score 85 -> expert  lock-free concurrency, easy to get wrong
would use hermes/claude-opus-4.8
```

## How it works

1. A small local model (default `qwen2.5:7b-instruct`) scores the task, warm in about 0.3s.
2. That score selects a level, and each level is an ordered chain of engines.
   Routing tries the first engine and falls back to the next on failure.
3. Work below your threshold stays local with no cloud bill. Work above it goes to the cloud model you chose.

Two ways to rate a task, both configured in `models.yaml`:

- **Score mode** (most flexible): the judge emits a 0-100 difficulty score and
  you define your own levels with your own thresholds, names, and engine chains.
- **Tier mode**: three fixed buckets, EASY / HARD / EXPERT.

Nothing is hardcoded. See [Customize](#customize).

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
the flake as an input. Full list in [docs/prerequisites.md](docs/prerequisites.md).

## Set up

After installing, run the wizard. It checks prerequisites, lists the Ollama
models you already have, lets you choose tier or score mode, define your levels
and thresholds, pick models, take any API token, and writes a valid config.

```bash
route init      # interactive setup
route doctor    # check prerequisites any time
```

Prefer to edit by hand? See [docs/configuration.md](docs/configuration.md).

## Use

```bash
route "add a null check to this parser"     # judge, route, run
route --dry "design a billing API"          # show the decision only
route --explain "debug this flaky test"     # show the judge reasoning
route --score 90 "..."                      # force a score (score mode)
route --tier EXPERT "..."                   # force a tier or level name
route --engine ollama "..."                 # force an engine in the chain
route --json "..."                          # machine readable
route --stats                               # summarize the routing log
route bench                                 # measure the judge (see below)
```

Config is found at `~/.config/promptrouter/models.yaml`, then
`/etc/promptrouter/models.yaml`, then `./models.yaml`. Override with `--config`
or `PROMPTROUTER_CONFIG`.

## Customize

`models.yaml` is the whole product. Make it as coarse or as fine-grained as you
like.

**Score mode** puts you fully in control of where the handoffs happen:

```yaml
judge:
  model: qwen2.5:7b-instruct
  mode: score

levels:
  - name: local          # 0-39: your GPU handles it, free
    min_score: 0
    chain:
      - engine: ollama
        model: qwen3-coder:30b
  - name: mid            # 40-74: a mid cloud model
    min_score: 40
    chain:
      - engine: openrouter
        model: anthropic/claude-3.5-sonnet
      - engine: ollama   # local fallback if the cloud call fails
        model: qwen3-coder:30b
  - name: expert         # 75+: the strongest model, only when earned
    min_score: 75
    chain:
      - engine: hermes
        model: claude-opus-4.8
```

The highest level whose `min_score` the task clears wins. Add two levels or add
ten; move the thresholds; give each its own chain. Want everything local under
80 and cloud above? Set one threshold at 80. It is your call, per level.

**Tier mode** is the simpler three-bucket form:

```yaml
tiers:
  HARD:
    chain:
      - engine: openrouter
        model: anthropic/claude-3.5-sonnet
      - engine: ollama
        model: qwen3-coder:30b
```

Engines: `ollama` (local, free), `openrouter` (cloud, one API key), `hermes`
(cloud, via the hermes CLI). Full reference with tunables and secrets handling in
[docs/configuration.md](docs/configuration.md).

## Does it actually route? Yes.

Routing is real, not a mock. The dispatcher calls Ollama's `/api/chat` for local
models, shells out to the `hermes` CLI, and POSTs to the OpenRouter API. Each
engine in a chain is tried in order until one answers. You can watch it happen
with `route --json`, which shows every engine attempted.

## Benchmarks

`route bench` calls the **real judge** on a labeled task set and reports how
often it agreed with the labels, plus per-call latency. It does not fake
anything. With `--execute` it also dispatches each task to the real engines.

On the author's RTX 2070, built-in 15-task set:

| Judge model            | Accuracy | Judge avg | Note                     |
|------------------------|----------|-----------|--------------------------|
| **qwen2.5:7b-instruct**| **93%**  | 0.28s     | recommended default      |
| qwen2.5:3b-instruct    | 67%      | 0.26s     | weak on hard tasks       |
| qwen2.5:1.5b-instruct  | 60%      | 0.53s     | only if 7b is too slow   |
| llama3.2:1b            | 33%      | 0.44s     | worse than guessing      |

Going below 7B buys almost no speed on this GPU (the floor is inference warmup,
not model size) while accuracy falls off a cliff. So the recommended judge is the
smallest one that still routes reliably, not the smallest one that runs.

Two important honesty notes:

- Accuracy is *agreement with a hand-labeled set*, not a measure of answer
  quality. It tells you the judge routes the way a human would, nothing more.
- A misjudged hard task still lands on a capable engine if your chain is set up
  that way. The judge picks the level; your chain decides the fallbacks.

Reproduce it yourself, or bring your own tasks:

```bash
route bench                       # built-in set, current judge
route bench --config other.yaml   # a different judge or mode
route bench --set myset.json      # your own labeled tasks
route bench --execute             # also run the real engines (slow)
route bench --json                # machine readable
```

Full methodology and hardware details in [docs/benchmarks.md](docs/benchmarks.md).

## Measuring your own savings

Turn on `log_file` and use `route` for real work. Then:

```
$ route --stats
routes: 214  ok: 214  local (free): 171 (79% of successful)
by tier:   local 171, mid 34, expert 9
by engine: ollama/qwen3-coder:30b 171, hermes/claude-sonnet-5 34, hermes/claude-opus-4.8 9
```

That percentage is the point. It is your workload, not a benchmark.

## Build

```bash
go build -o route ./cmd/route
go test ./...
```

## Compare

There are good routers out there. This one is the local-first, self-hosted corner.

| | promptrouter | OpenRouter Auto | RouteLLM |
| --- | --- | --- | --- |
| Routes to your local GPU | yes | no | DIY |
| Runs fully self hosted | yes | no | yes |
| Code stays on your machine | yes | no | yes |
| Custom levels and thresholds | yes | no | partial |
| Setup | one YAML | one API key | Python plus a trained model |

Point it at any local model through Ollama and any cloud model through OpenRouter
or hermes. The judge decides per task; you decide the thresholds.

## Support

Questions, bug reports, or want to show off your routing config? Open an issue
using one of the forms, or come chat in [Discord](https://discord.gg/bGx9VetCC4).

## License

MIT. See [LICENSE](LICENSE).
