package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/taynotfound/promptrouter/internal/router"
)

// runInit is the interactive setup wizard: it checks prerequisites, lets the
// user pick a local model and an optional cloud provider, collects any token,
// and writes models.yaml plus a secrets file.
func runInit(args []string) int {
	in := bufio.NewReader(os.Stdin)

	fmt.Println("promptrouter setup")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println("This writes a config to", router.DefaultConfigPath())
	fmt.Println()

	// 1. Prerequisites.
	ollamaURL := ask(in, "Ollama URL", "http://localhost:11434")
	fmt.Println("\nChecking prerequisites...")
	ok, models := router.CheckOllama(ollamaURL)
	if !ok {
		fmt.Println("  [x] Ollama is not reachable at", ollamaURL)
		fmt.Println("      Install it from https://ollama.com and run: ollama serve")
		if !confirm(in, "Continue anyway?", false) {
			return 1
		}
	} else {
		fmt.Printf("  [ok] Ollama reachable, %d model(s) available\n", len(models))
	}

	// 2. Local model for EASY tier.
	local := ""
	if len(models) > 0 {
		fmt.Println("\nLocal models found:")
		for i, m := range models {
			fmt.Printf("  %d) %s\n", i+1, m)
		}
		choice := ask(in, "Pick a number for the local (free) model, or type a name", "1")
		local = pickFromList(choice, models)
	} else {
		local = ask(in, "Local Ollama model to use", "qwen3-coder:30b")
		fmt.Printf("  note: pull it later with: ollama pull %s\n", local)
	}

	// 3. Judge model.
	fmt.Println("\nJudge model: a small, fast model that scores each task.")
	fmt.Println("  Benchmarked on the author's RTX 2070 (see docs/benchmarks.md):")
	fmt.Println("    qwen2.5:7b-instruct   93% agreement, ~0.3s   <- recommended")
	fmt.Println("    qwen2.5:3b-instruct   67% agreement, ~0.3s   smaller, weaker on hard tasks")
	fmt.Println("    qwen2.5:1.5b-instruct 60% agreement          only if 7b is too slow for you")
	fmt.Println("  Going below 7b buys little speed (the floor is warmup, not size) and")
	fmt.Println("  loses accuracy. Any Ollama model works; run `route bench` to compare.")
	judge := ask(in, "Judge model", "qwen2.5:7b-instruct")

	plan := router.Plan{
		JudgeModel: judge,
		LocalModel: local,
		OllamaURL:  ollamaURL,
		LogFile:    "~/.promptrouter/routes.jsonl",
	}

	// 4. Routing mode.
	fmt.Println("\nRouting mode:")
	fmt.Println("  1) tiers   three fixed buckets: EASY, HARD, EXPERT")
	fmt.Println("  2) score   custom levels with your own 0-100 thresholds (most flexible)")
	modeChoice := strings.TrimSpace(ask(in, "Pick", "1"))
	if modeChoice == "2" || strings.EqualFold(modeChoice, "score") {
		if buildScorePlan(in, &plan, models) != 0 {
			return 1
		}
		return writePlan(in, plan)
	}

	// 5. Cloud provider (optional), tier mode.
	fmt.Println("\nCloud engine for HARD and EXPERT tasks (optional):")
	fmt.Println("  1) none        local only, fully free and private")
	fmt.Println("  2) openai      OpenAI or any OpenAI-compatible API (Groq, Together, vLLM, ...)")
	fmt.Println("  3) anthropic   Claude models, Anthropic API key")
	fmt.Println("  4) openrouter  any cloud model behind one OpenRouter key")
	fmt.Println("  5) hermes      github-copilot models via the hermes CLI")
	cloudChoice := ask(in, "Pick", "1")

	switch strings.TrimSpace(cloudChoice) {
	case "2", "openai":
		plan.CloudEngine = "openai"
		plan.CloudHard = ask(in, "OpenAI model for HARD", "gpt-4o-mini")
		plan.CloudExpert = ask(in, "OpenAI model for EXPERT", "gpt-4o")
		base := ask(in, "API base URL (blank for OpenAI; set for Groq/Together/vLLM/etc.)", "")
		plan.CloudBaseURL = base
		keyEnv := "OPENAI_API_KEY"
		if base != "" {
			keyEnv = ask(in, "Env var that holds the API key", "OPENAI_API_KEY")
		}
		plan.CloudKeyEnv = keyEnv
		saveKey(in, keyEnv, "API key (leave blank to set later)")
	case "3", "anthropic":
		plan.CloudEngine = "anthropic"
		plan.CloudHard = ask(in, "Claude model for HARD", "claude-3-5-sonnet-latest")
		plan.CloudExpert = ask(in, "Claude model for EXPERT", "claude-opus-4-20250514")
		saveKey(in, "ANTHROPIC_API_KEY", "Anthropic API key (leave blank to set later)")
	case "4", "openrouter":
		plan.CloudEngine = "openrouter"
		plan.CloudHard = ask(in, "OpenRouter model for HARD", "anthropic/claude-3.5-sonnet")
		plan.CloudExpert = ask(in, "OpenRouter model for EXPERT", "anthropic/claude-3-opus")
		saveKey(in, "OPENROUTER_API_KEY", "OpenRouter API key (leave blank to set later)")
	case "5", "hermes":
		plan.CloudEngine = "hermes"
		plan.CloudHard = ask(in, "hermes model for HARD", "claude-sonnet-5")
		plan.CloudExpert = ask(in, "hermes model for EXPERT", "claude-opus-4.8")
		fmt.Println("  hermes uses your existing github-copilot auth, no token needed here")
	default:
		fmt.Println("  local only. You can rerun `route init` to add a cloud engine later.")
	}

	return writePlan(in, plan)
}

// saveKey prompts for a secret and writes it to the secrets file under keyEnv,
// unless the value is already present. Blank input is left for the user to set
// later. Used by both the tier and score wizards.
func saveKey(in *bufio.Reader, keyEnv, prompt string) {
	if router.SecretValue(keyEnv) != "" {
		return
	}
	key := askSecret(in, "  "+prompt)
	if key == "" {
		return
	}
	if err := router.WriteSecret(keyEnv, key); err != nil {
		fmt.Println("  could not save key:", err)
		return
	}
	fmt.Println("  saved to", router.SecretsPath())
}

// buildScorePlan drives the score-mode level builder: the user names each level,
// sets the score it starts at, and picks an engine + model for it. Returns
// nonzero on abort.
func buildScorePlan(in *bufio.Reader, plan *router.Plan, models []string) int {
	fmt.Println("\nScore mode: define your levels from easiest to hardest.")
	fmt.Println("The judge scores each task 0-100. The highest level whose")
	fmt.Println("threshold the score clears wins. The first level must start at 0.")

	n := 0
	if _, err := fmt.Sscanf(strings.TrimSpace(ask(in, "\nHow many levels?", "3")), "%d", &n); err != nil || n < 1 {
		n = 3
	}
	if n > 10 {
		n = 10
	}

	for i := 0; i < n; i++ {
		fmt.Printf("\n--- level %d of %d ---\n", i+1, n)
		defName := fmt.Sprintf("level%d", i+1)
		switch {
		case i == 0:
			defName = "local"
		case i == n-1:
			defName = "expert"
		}
		name := ask(in, "Name", defName)

		min := 0
		if i == 0 {
			fmt.Println("  (first level starts at 0)")
		} else {
			defMin := fmt.Sprintf("%d", i*(100/n))
			if _, err := fmt.Sscanf(strings.TrimSpace(ask(in, "Starts at score", defMin)), "%d", &min); err != nil {
				min = i * (100 / n)
			}
		}

		fmt.Println("  engine for this level:")
		fmt.Println("    1) ollama      local, free")
		fmt.Println("    2) openai      OpenAI or any OpenAI-compatible API")
		fmt.Println("    3) anthropic   Claude via Anthropic API key")
		fmt.Println("    4) openrouter  cloud via OpenRouter key")
		fmt.Println("    5) hermes      github-copilot via hermes CLI")
		var eng router.Engine
		switch strings.TrimSpace(ask(in, "  Pick", "1")) {
		case "2", "openai":
			eng.Kind = "openai"
			eng.Model = ask(in, "  OpenAI model", "gpt-4o-mini")
			base := ask(in, "  API base URL (blank for OpenAI; set for Groq/Together/vLLM/etc.)", "")
			eng.BaseURL = base
			eng.KeyEnv = "OPENAI_API_KEY"
			if base != "" {
				eng.KeyEnv = ask(in, "  Env var that holds the API key", "OPENAI_API_KEY")
			}
			saveKey(in, eng.KeyEnv, "API key (leave blank to set later)")
		case "3", "anthropic":
			eng.Kind = "anthropic"
			eng.Model = ask(in, "  Claude model", "claude-3-5-sonnet-latest")
			saveKey(in, "ANTHROPIC_API_KEY", "Anthropic API key (leave blank to set later)")
		case "4", "openrouter":
			eng.Kind = "openrouter"
			eng.Model = ask(in, "  OpenRouter model", "anthropic/claude-3.5-sonnet")
			saveKey(in, "OPENROUTER_API_KEY", "OpenRouter API key (leave blank to set later)")
		case "5", "hermes":
			eng.Kind = "hermes"
			eng.Model = ask(in, "  hermes model", "claude-sonnet-5")
		default:
			eng.Kind = "ollama"
			def := plan.LocalModel
			if len(models) > 0 && def == "" {
				def = models[0]
			}
			eng.Model = ask(in, "  ollama model", def)
		}

		spec := router.LevelSpec{Name: name, MinScore: min, Chain: []router.Engine{eng}}
		if eng.Kind != "ollama" && plan.LocalModel != "" {
			if confirm(in, "  Add a local ollama fallback if this engine fails?", true) {
				spec.Chain = append(spec.Chain, router.Engine{Kind: "ollama", Model: plan.LocalModel})
			}
		}
		plan.Levels = append(plan.Levels, spec)
	}
	return 0
}

// writePlan validates the plan, confirms an overwrite, and saves it.
func writePlan(in *bufio.Reader, plan router.Plan) int {
	if err := plan.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, "\nerror:", err)
		return 1
	}
	path := router.DefaultConfigPath()
	if _, err := os.Stat(path); err == nil {
		if !confirm(in, fmt.Sprintf("\n%s exists. Overwrite?", path), false) {
			fmt.Println("aborted, nothing written")
			return 1
		}
	}
	if err := plan.Save(path); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Println("\nDone. Wrote", path)
	fmt.Println("Try it:")
	fmt.Println(`  route --explain "rename a variable"`)
	return 0
}

// runDoctor prints prerequisite status and exits nonzero if a required one fails.
func runDoctor(args []string) int {
	url := "http://localhost:11434"
	if len(args) > 0 {
		url = args[0]
	}
	fmt.Println("promptrouter doctor")
	fmt.Println(strings.Repeat("-", 40))
	allOK := true
	for _, p := range router.Doctor(url) {
		mark := "[ok]"
		if !p.OK {
			mark = "[x]"
		}
		fmt.Printf("%-5s %s: %s\n", mark, p.Name, p.Detail)
		if !p.OK && p.Hint != "" {
			fmt.Println("       ", p.Hint)
		}
		// only the ollama check is required
		if !p.OK && strings.HasPrefix(p.Name, "Ollama") {
			allOK = false
		}
	}
	if !allOK {
		fmt.Println("\nOllama is required for the local tier. See hints above.")
		return 1
	}
	fmt.Println("\nAll required prerequisites met.")
	return 0
}

func ask(in *bufio.Reader, prompt, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", strings.TrimLeft(prompt, "\n"), def)
	} else {
		fmt.Printf("%s: ", strings.TrimLeft(prompt, "\n"))
	}
	line, _ := in.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func askSecret(in *bufio.Reader, prompt string) string {
	fmt.Printf("%s: ", prompt)
	line, _ := in.ReadString('\n')
	return strings.TrimSpace(line)
}

func confirm(in *bufio.Reader, prompt string, def bool) bool {
	d := "y/N"
	if def {
		d = "Y/n"
	}
	fmt.Printf("%s [%s]: ", strings.TrimLeft(prompt, "\n"), d)
	line, _ := in.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return def
	}
	return line == "y" || line == "yes"
}

func pickFromList(choice string, list []string) string {
	choice = strings.TrimSpace(choice)
	// numeric pick
	var n int
	if _, err := fmt.Sscanf(choice, "%d", &n); err == nil && n >= 1 && n <= len(list) {
		return list[n-1]
	}
	return choice
}
