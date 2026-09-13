# Contributing

Thanks for looking. promptrouter is small and meant to stay that way.

## Ground rules

- Keep it dependency light. The only third party import is a YAML parser.
- The judge benchmark stays honest. If a change moves accuracy, say so in the PR.
- Run `gofmt`, `go vet`, and `go test ./...` before you push. CI runs all three.

## Add an engine

Engines live in `internal/router/engines.go`. Add a function with the same shape
as `ollamaChat` or `runHermes`, then wire it into `runEngine` in `dispatch.go` and
add the name to `validKinds` in `config.go`. Keep errors specific: include the HTTP
status and body, or the process stderr.

## Build

```bash
go build -o route ./cmd/route
go test ./...
```

## Good first issues

- Add an engine for llama.cpp server or vLLM (OpenAI compatible endpoint).
- Add a `--budget` flag that caps how many cloud calls a session may make.
- Add cost accounting to `route --stats` (tokens times a per model rate).
- Ship a shell completion file for bash, zsh, and fish.
