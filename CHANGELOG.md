# Changelog

## v0.2.0

Customization and honest benchmarking.

### Added
- **Score-based routing.** Set `judge.mode: score` and the judge emits a 0-100
  difficulty score. Define your own `levels` with custom names, thresholds, and
  engine chains, from two levels to ten. The highest level a task clears wins.
- **`route bench`.** Runs the real judge over a labeled task set and reports
  accuracy (agreement with labels) and per-call latency. `--execute` also
  dispatches to the real engines; `--set FILE.json` uses your own tasks;
  `--json` for machine output.
- **`--score N`** flag to force a difficulty score, alongside the existing
  `--tier` which now also accepts a custom level name.
- **Wizard score mode.** `route init` offers tiers vs score mode and walks you
  through defining levels and thresholds. The judge prompt now shows benchmarked
  accuracy and latency so you pick the smallest viable judge.
- **docs/benchmarks.md** with reproducible RTX 2070 numbers and methodology.

### Changed
- README rewritten around score mode, customization, and real benchmark numbers.
  Removed the previous unverified accuracy claim.
- Routing log now records the numeric score alongside the tier.

### Notes
- Routing has always been real: the dispatcher calls Ollama's `/api/chat`,
  shells to the `hermes` CLI, and POSTs to the OpenRouter API. `route bench`
  exercises that same path.
- Fully backward compatible. Existing tier-mode `models.yaml` files work
  unchanged.

## v0.1.0

Initial release: local-first tier routing, `route init` wizard, `route doctor`,
secrets handling, cross-distro packaging (deb, rpm, AUR, Nix flake), one static
binary.
