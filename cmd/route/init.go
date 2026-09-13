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
	judge := ask(in, "\nJudge model (small and fast is best)", "qwen2.5:7b-instruct")

	// 4. Cloud provider (optional).
	fmt.Println("\nCloud engine for HARD and EXPERT tasks (optional):")
	fmt.Println("  1) none      local only, fully free and private")
	fmt.Println("  2) openrouter   any cloud model, one API key")
	fmt.Println("  3) hermes    github-copilot models via the hermes CLI")
	cloudChoice := ask(in, "Pick", "1")

	plan := router.Plan{
		JudgeModel: judge,
		LocalModel: local,
		OllamaURL:  ollamaURL,
		LogFile:    "~/.promptrouter/routes.jsonl",
	}

	switch strings.TrimSpace(cloudChoice) {
	case "2", "openrouter":
		plan.CloudEngine = "openrouter"
		plan.CloudHard = ask(in, "OpenRouter model for HARD", "anthropic/claude-3.5-sonnet")
		plan.CloudExpert = ask(in, "OpenRouter model for EXPERT", "anthropic/claude-3-opus")
		key := askSecret(in, "OpenRouter API key (leave blank to set later)")
		if key != "" {
			if err := router.WriteSecret("OPENROUTER_API_KEY", key); err != nil {
				fmt.Println("  could not save key:", err)
			} else {
				fmt.Println("  saved to", router.SecretsPath())
			}
		}
	case "3", "hermes":
		plan.CloudEngine = "hermes"
		plan.CloudHard = ask(in, "hermes model for HARD", "claude-sonnet-5")
		plan.CloudExpert = ask(in, "hermes model for EXPERT", "claude-opus-4.8")
		fmt.Println("  hermes uses your existing github-copilot auth, no token needed here")
	default:
		fmt.Println("  local only. You can rerun `route init` to add a cloud engine later.")
	}

	// 5. Validate and write.
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
