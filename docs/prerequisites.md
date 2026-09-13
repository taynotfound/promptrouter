# Prerequisites

promptrouter routes coding tasks to language models. What you need depends on
which tiers you want to use.

## Always required

- **A 64-bit Linux or macOS machine** (amd64 or arm64). The binary is static,
  so there is no runtime to install.

## For the local tier (free, private)

- **[Ollama](https://ollama.com)** running locally. Install it, start the
  server, and pull at least one coding model:

  ```sh
  ollama serve &
  ollama pull qwen2.5-coder:7b      # a small judge / fallback model
  ollama pull qwen3-coder:30b       # a stronger local model if you have the VRAM
  ```

- **A GPU helps but is not required.** A 7B model runs on CPU; a 30B MoE model
  wants roughly 16 GB of VRAM or unified memory to be quick.

## For the cloud tiers (optional)

Pick one:

- **OpenRouter**: one API key reaches most hosted models. Create a key at
  <https://openrouter.ai/keys>. `route init` stores it for you.

- **hermes CLI**: routes to github-copilot models through your existing
  hermes login. Install hermes separately and make sure `hermes` is on your
  PATH. No extra token is needed.

## Checking your setup

Run the built-in checker at any time:

```sh
route doctor
```

It reports whether Ollama is reachable, how many models you have pulled,
whether the hermes CLI is present, and whether you are online.
