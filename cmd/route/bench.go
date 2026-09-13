package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/taynotfound/promptrouter/internal/router"
)

// benchTask is one labeled item: a prompt and the tier a human expects it to
// land in. The score bands map EASY<40, HARD 40-74, EXPERT>=75 so the same set
// scores both tier-mode and score-mode judges.
type benchTask struct {
	Prompt string `json:"prompt"`
	Expect string `json:"expect"` // EASY | HARD | EXPERT
}

// defaultBenchSet is a small hand-labeled set covering the three tiers. It is
// deliberately modest and honest: 15 tasks, not a research benchmark. Users can
// supply their own with --set FILE.
var defaultBenchSet = []benchTask{
	{"rename the variable userId to accountId across this file", "EASY"},
	{"add a trailing newline to the end of config.yaml", "EASY"},
	{"format this JSON file with two-space indentation", "EASY"},
	{"write a one-line function that returns the square of an integer", "EASY"},
	{"add a docstring to this Python function", "EASY"},
	{"fix the off-by-one error in this for loop", "HARD"},
	{"add retry with exponential backoff to this HTTP client", "HARD"},
	{"design a REST API for a todo app with auth and pagination", "HARD"},
	{"debug why this goroutine leaks under load and fix it", "HARD"},
	{"refactor this 200-line function into testable units", "HARD"},
	{"implement a lock-free single-producer single-consumer ring buffer", "EXPERT"},
	{"prove this concurrent stack is linearizable and write the invariant", "EXPERT"},
	{"design a Raft-based distributed consensus layer with membership changes", "EXPERT"},
	{"write a constant-time AES key comparison resistant to timing attacks", "EXPERT"},
	{"derive and implement a wait-free multi-word compare-and-swap", "EXPERT"},
}

// tierForScore maps a 0-100 score onto the three-way label so a score-mode
// judge can be measured against the same expected tiers.
func tierForScore(s int) string {
	switch {
	case s < 40:
		return "EASY"
	case s < 75:
		return "HARD"
	default:
		return "EXPERT"
	}
}

// runBench measures the judge honestly: it calls the real judge model on each
// labeled task, records the predicted tier (or score, mapped to a tier), and
// reports accuracy and per-call latency. With --execute it also dispatches each
// task to the real engine chain and times the end-to-end answer.
func runBench(args []string) int {
	var (
		configPath, setPath string
		execute, jsonOut    bool
	)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			i++
			if i < len(args) {
				configPath = args[i]
			}
		case "--set":
			i++
			if i < len(args) {
				setPath = args[i]
			}
		case "--execute":
			execute = true
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Println("route bench [--config PATH] [--set FILE.json] [--execute] [--json]")
			fmt.Println("  Runs the real judge on a labeled task set and reports accuracy + latency.")
			fmt.Println("  --execute also dispatches each task to the real engines (slow, uses cloud credits).")
			return 0
		}
	}

	router.LoadSecrets()
	cfgFile, err := resolveConfig(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	cfg, err := router.Load(cfgFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	set := defaultBenchSet
	setName := "built-in (15 tasks)"
	if setPath != "" {
		raw, err := os.ReadFile(setPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error reading set:", err)
			return 1
		}
		var custom []benchTask
		if err := json.Unmarshal(raw, &custom); err != nil || len(custom) == 0 {
			fmt.Fprintln(os.Stderr, "error: set must be a non-empty JSON array of {prompt, expect}")
			return 1
		}
		set = custom
		setName = fmt.Sprintf("%s (%d tasks)", setPath, len(custom))
	}

	scoreMode := cfg.ScoreMode()
	type row struct {
		Prompt    string  `json:"prompt"`
		Expect    string  `json:"expect"`
		Predicted string  `json:"predicted"`
		Score     int     `json:"score,omitempty"`
		Correct   bool    `json:"correct"`
		JudgeSec  float64 `json:"judge_seconds"`
		RunEngine string  `json:"engine,omitempty"`
		RunModel  string  `json:"model,omitempty"`
		RunSec    float64 `json:"run_seconds,omitempty"`
		RunOK     bool    `json:"run_ok,omitempty"`
	}

	var rows []row
	correct := 0
	var judgeTotal, runTotal float64
	byExpect := map[string]int{}
	correctByExpect := map[string]int{}

	if !jsonOut {
		fmt.Println("promptrouter bench")
		fmt.Println(strings.Repeat("-", 60))
		fmt.Println("judge:   ", cfg.JudgeCfg.Model)
		fmt.Println("mode:    ", modeLabel(scoreMode))
		fmt.Println("task set:", setName)
		if execute {
			fmt.Println("execute:  ON (dispatching to real engines)")
		}
		fmt.Println()
	}

	for _, t := range set {
		byExpect[t.Expect]++
		var predicted string
		var score int
		jt0 := time.Now()
		if scoreMode {
			score = cfg.JudgeScore(t.Prompt)
			predicted = tierForScore(score)
		} else {
			predicted = cfg.Judge(t.Prompt)
		}
		jsec := time.Since(jt0).Seconds()
		judgeTotal += jsec
		ok := strings.EqualFold(predicted, t.Expect)
		if ok {
			correct++
			correctByExpect[t.Expect]++
		}
		r := row{Prompt: t.Prompt, Expect: t.Expect, Predicted: predicted, Score: score, Correct: ok, JudgeSec: jsec}

		if execute {
			res := cfg.Dispatch(t.Prompt, router.Options{ForceScore: -1})
			r.RunEngine, r.RunModel, r.RunSec, r.RunOK = res.Engine, res.Model, res.Seconds, res.OK
			runTotal += res.Seconds
		}
		rows = append(rows, r)

		if !jsonOut {
			mark := "MISS"
			if ok {
				mark = " ok "
			}
			label := predicted
			if scoreMode {
				label = fmt.Sprintf("%s(%d)", predicted, score)
			}
			line := fmt.Sprintf("[%s] want %-6s got %-9s %4.1fs  %s", mark, t.Expect, label, jsec, truncate(t.Prompt, 42))
			if execute {
				line += fmt.Sprintf("  -> %s/%s %.1fs", r.RunEngine, r.RunModel, r.RunSec)
			}
			fmt.Println(line)
		}
	}

	n := len(set)
	acc := float64(correct) / float64(n) * 100

	if jsonOut {
		out := map[string]any{
			"judge":         cfg.JudgeCfg.Model,
			"mode":          modeLabel(scoreMode),
			"tasks":         n,
			"correct":       correct,
			"accuracy_pct":  acc,
			"judge_avg_sec": judgeTotal / float64(n),
			"rows":          rows,
		}
		if execute {
			out["run_avg_sec"] = runTotal / float64(n)
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		return 0
	}

	fmt.Println()
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("accuracy:     %d/%d  (%.0f%%)\n", correct, n, acc)
	fmt.Printf("judge avg:    %.2fs per task\n", judgeTotal/float64(n))
	if execute {
		fmt.Printf("dispatch avg: %.2fs per task\n", runTotal/float64(n))
	}
	var tiers []string
	for k := range byExpect {
		tiers = append(tiers, k)
	}
	sort.Strings(tiers)
	fmt.Print("per label:    ")
	var parts []string
	for _, k := range tiers {
		parts = append(parts, fmt.Sprintf("%s %d/%d", k, correctByExpect[k], byExpect[k]))
	}
	fmt.Println(strings.Join(parts, ", "))
	fmt.Println()
	fmt.Println("Note: accuracy is agreement with a small hand-labeled set, not a")
	fmt.Println("statement of answer quality. Bring your own set with --set FILE.json.")
	return 0
}

func modeLabel(scoreMode bool) string {
	if scoreMode {
		return "score"
	}
	return "tiers"
}
