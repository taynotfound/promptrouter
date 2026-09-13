// Command route judges a coding task and sends it to the right model:
// local GPU for the easy stuff, cloud only when the task earns it.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/taynotfound/promptrouter/internal/router"
)

var version = "dev"

const usage = `route: judge a coding task and send it to the right model.

Usage:
  route [flags] "your task"

Flags:
  --dry           Judge only, show which model would run.
  --explain       Judge only, show the reason for the tier.
  --tier T        Force a tier (EASY, HARD, EXPERT).
  --engine E      Force an engine within the tier (ollama, hermes, openrouter).
  --json          Machine readable output.
  --config PATH   Path to models.yaml (default: search standard locations).
  --stats         Summarize the routing log and exit.
  --version       Print version.
  -h, --help      Show this help.`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	var (
		dry, explain, jsonOut, stats bool
		tier, engine, configPath     string
		words                        []string
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--dry":
			dry = true
		case a == "--explain":
			explain = true
		case a == "--json":
			jsonOut = true
		case a == "--stats":
			stats = true
		case a == "--version":
			fmt.Println("route", version)
			return 0
		case a == "-h" || a == "--help":
			fmt.Println(usage)
			return 0
		case a == "--tier":
			i++
			if i < len(args) {
				tier = strings.ToUpper(args[i])
			}
		case a == "--engine":
			i++
			if i < len(args) {
				engine = args[i]
			}
		case a == "--config":
			i++
			if i < len(args) {
				configPath = args[i]
			}
		case strings.HasPrefix(a, "--"):
			fmt.Fprintln(os.Stderr, "unknown flag:", a)
			return 1
		default:
			words = append(words, a)
		}
	}

	cfgFile, err := resolveConfig(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	cfg, err := router.Load(cfgFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if stats {
		out, err := cfg.Stats()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(out)
		return 0
	}

	task := strings.TrimSpace(strings.Join(words, " "))
	if task == "" {
		fmt.Println(usage)
		return 1
	}

	if explain {
		t, why := cfg.JudgeExplain(task)
		first := cfg.Tiers[t].Chain[0]
		if jsonOut {
			b, _ := json.Marshal(map[string]string{"tier": t, "why": why, "engine": first.Kind, "model": first.Model})
			fmt.Println(string(b))
		} else {
			fmt.Printf("[judge] %s  %s\n", t, why)
			fmt.Printf("would use %s/%s\n", first.Kind, first.Model)
		}
		return 0
	}

	if dry {
		t := tier
		if t == "" {
			t = cfg.Judge(task)
		}
		first := cfg.Tiers[t].Chain[0]
		if jsonOut {
			b, _ := json.Marshal(map[string]string{"tier": t, "engine": first.Kind, "model": first.Model})
			fmt.Println(string(b))
		} else {
			fmt.Printf("[judge] %s: %s\n", t, truncate(task, 70))
			fmt.Printf("would use %s/%s\n", first.Kind, first.Model)
		}
		return 0
	}

	res := cfg.Dispatch(task, router.Options{ForceTier: tier, ForceEngine: engine})
	if jsonOut {
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(b))
	} else {
		fmt.Println(res.Summary())
		fmt.Println(strings.Repeat("-", 60))
		if res.OK {
			fmt.Println(res.Text)
		} else {
			fmt.Println("ERROR:", res.Err)
		}
	}
	if res.OK {
		return 0
	}
	return 2
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// resolveConfig finds models.yaml: explicit flag, env, XDG, then next to binary.
func resolveConfig(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if env := os.Getenv("PROMPTROUTER_CONFIG"); env != "" {
		return env, nil
	}
	var candidates []string
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".config", "promptrouter", "models.yaml"))
	}
	candidates = append(candidates,
		"/etc/promptrouter/models.yaml",
		"models.yaml",
	)
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "models.yaml"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("no models.yaml found (looked in ~/.config/promptrouter, /etc/promptrouter, cwd). Pass --config or set PROMPTROUTER_CONFIG")
}
