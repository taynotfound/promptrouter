# Benchmarks

These numbers come from `route bench` run on the author's machine. They are
reproducible: run `route bench` yourself and you will get the same shape of
result. Nothing here is hand-waved.

## What `route bench` actually measures

`route bench` calls the **real judge model** through Ollama on a set of
hand-labeled coding tasks. For each task it records the tier the judge predicts
(or, in score mode, the 0-100 score mapped onto EASY < 40, HARD 40-74,
EXPERT >= 75) and the wall-clock time the judge call took. It then reports:

- **accuracy** = how often the judge agreed with the human label
- **judge avg** = mean latency of one judge call
- **per label** = correct / total within each tier

With `--execute` it additionally dispatches every task to the real engine chain
and times the end-to-end answer. That path really runs the models; it is slow
and, if a level points at a cloud engine, it spends real credits.

**Honesty note:** accuracy here is *agreement with a small hand-labeled set*,
not a measure of answer quality. A judge that agrees with the labels is routing
the way the author would; it says nothing about whether the downstream model
writes good code. The built-in set is 15 tasks. Bring your own with
`route bench --set yourset.json` (a JSON array of `{"prompt": ..., "expect":
"EASY|HARD|EXPERT"}`).

## Hardware

- CPU: AMD Ryzen 7 3700X
- GPU: NVIDIA GeForce RTX 2070, 8 GB VRAM
- Ollama 0.32.x, models quantized as pulled from the Ollama library

## Judge model comparison (score mode, built-in 15-task set)

Every judge was warmed once before timing so model-load time is excluded.

| Judge model            | Size  | Accuracy | Judge avg | Notes                         |
|------------------------|-------|----------|-----------|-------------------------------|
| qwen2.5:0.5b-instruct  | 0.5B  | 60%      | ~0.8s     | cold; unreliable HARD/EXPERT  |
| llama3.2:1b            | 1B    | 33%      | 0.34s     | too small, misroutes badly    |
| qwen2.5:1.5b-instruct  | 1.5B  | 60%      | 0.26s     | weak on HARD                  |
| gemma2:2b              | 2B    | 53%      | 0.59s     | weak on HARD                  |
| qwen2.5:3b-instruct    | 3B    | 67%      | 0.28s     | collapses on HARD (1/5)       |
| **qwen2.5:7b-instruct**| 7B    | **93%**  | 0.32s     | **recommended default**       |

## The smallest-judge question, answered honestly

The intuition is "use the smallest model so judging is instant." On this GPU
that intuition does not hold. Warm, the 3B judge (0.28s) and the 7B judge
(0.32s) are within noise of each other on latency, because the floor is
per-call inference warmup, not parameter count. But accuracy falls off a cliff
below 7B: the 3B judge gets only 1 of 5 HARD tasks right, and the 1B model is
worse than guessing.

So the recommendation is **qwen2.5:7b-instruct**: it is the smallest model
tested that routes reliably, and going smaller buys no real speed while costing
a lot of accuracy. If you are on weaker hardware where 7B is genuinely slow,
qwen2.5:3b-instruct is the honest fallback, with the understanding that it will
under-route hard tasks to the local model.

The judge is fully user-settable. Set it in `models.yaml`:

```yaml
judge:
  model: qwen2.5:7b-instruct   # any Ollama model you have pulled
  mode: score                  # or omit for categorical tiers
```

or pick it in `route init`. Run `route bench` after changing it to see the
tradeoff on your own hardware and your own task set.

## Score mode vs tier mode

Same judge (qwen2.5:7b-instruct), same set:

| Mode  | Accuracy | Judge avg |
|-------|----------|-----------|
| score | 93%      | 0.32s     |
| tiers | 80%      | 0.28s     |

Score mode edged out categorical tiers here because the numeric prompt gives the
judge a finer target and the EASY/HARD/EXPERT bands are derived from the score.
The difference is small and set-dependent; the real reason to prefer score mode
is customization, not this delta.

## End-to-end dispatch latency (real answers)

These are full runs: judge, then the local model actually answers. Times are
dominated by the answering model (qwen3-coder:30b on an 8 GB card, so it is
partially offloaded to CPU), not the judge.

| Task                                    | Score | Model             | Time  |
|-----------------------------------------|-------|-------------------|-------|
| add a trailing newline to a file        | 25    | qwen3-coder:30b   | 47.7s |
| reverse a linked list (python)          | 25    | qwen3-coder:30b   | 84.1s |
| implement a small LRU cache class       | 50    | qwen3-coder:30b   | 63.1s |

The judge adds about 0.3s. The rest is the answering model. On a card that fits
a 7B-14B coder model fully in VRAM these end-to-end times drop sharply; the
judge cost stays flat.

## Reproduce

```sh
route bench                       # built-in set, current judge
route bench --config other.yaml   # a different judge/mode
route bench --set myset.json      # your own labeled tasks
route bench --execute             # also run the real engines (slow)
route bench --json                # machine-readable
```
