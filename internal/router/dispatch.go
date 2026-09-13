package router

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Attempt records one engine try in a route.
type Attempt struct {
	Engine string `json:"engine"`
	Model  string `json:"model"`
	Error  string `json:"error,omitempty"`
}

// Result is the outcome of routing one task.
type Result struct {
	Task    string    `json:"task"`
	Tier    string    `json:"tier"`
	Engine  string    `json:"engine"`
	Model   string    `json:"model"`
	Text    string    `json:"text"`
	OK      bool      `json:"ok"`
	Seconds float64   `json:"seconds"`
	Tried   []Attempt `json:"tried"`
	Err     string    `json:"error,omitempty"`
}

// Summary is a one line header for terminal output.
func (r *Result) Summary() string {
	icon := map[string]string{"EASY": "[EASY]", "HARD": "[HARD]", "EXPERT": "[EXPERT]"}[r.Tier]
	if r.OK {
		return fmt.Sprintf("%s %s/%s  %.1fs", icon, r.Engine, r.Model, r.Seconds)
	}
	return fmt.Sprintf("%s failed: %s", icon, r.Err)
}

// Options tweak a single dispatch.
type Options struct {
	ForceTier   string
	ForceEngine string
}

// Dispatch judges the task (unless a tier is forced), then walks the tier's
// fallback chain until an engine answers. Each engine gets MaxRetries tries
// for empty or transient failures before moving on.
func (c *Config) Dispatch(task string, opt Options) *Result {
	start := time.Now()
	task = strings.TrimSpace(task)
	res := &Result{Task: task}
	if task == "" {
		res.Err = "empty task"
		return res
	}

	tier := strings.ToUpper(opt.ForceTier)
	if tier == "" {
		tier = c.Judge(task)
	}
	if !validTiers[tier] {
		tier = c.Defaults.FallbackTier
	}
	res.Tier = tier

	chain := c.Tiers[tier].Chain
	for _, e := range chain {
		if opt.ForceEngine != "" && e.Kind != opt.ForceEngine {
			continue
		}
		text, err := c.runEngineRetrying(e, task)
		att := Attempt{Engine: e.Kind, Model: e.Model}
		if err != nil {
			att.Error = err.Error()
			res.Tried = append(res.Tried, att)
			continue
		}
		res.Tried = append(res.Tried, att)
		res.Engine, res.Model, res.Text, res.OK = e.Kind, e.Model, text, true
		res.Seconds = time.Since(start).Seconds()
		c.logRoute(res)
		return res
	}

	res.Seconds = time.Since(start).Seconds()
	if len(res.Tried) > 0 {
		last := res.Tried[len(res.Tried)-1]
		res.Err = fmt.Sprintf("%s/%s: %s", last.Engine, last.Model, last.Error)
	} else {
		res.Err = "no engine matched"
	}
	c.logRoute(res)
	return res
}

func (c *Config) runEngineRetrying(e Engine, task string) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Defaults.MaxRetries; attempt++ {
		text, err := c.runEngine(e, task)
		if err == nil && strings.TrimSpace(text) != "" {
			return text, nil
		}
		if err == nil {
			lastErr = fmt.Errorf("%s: empty response", e.Kind)
		} else {
			lastErr = err
		}
		if c.Defaults.RetryBackoff > 0 && attempt < c.Defaults.MaxRetries {
			time.Sleep(time.Duration(c.Defaults.RetryBackoff * float64(time.Second)))
		}
	}
	return "", lastErr
}

func (c *Config) runEngine(e Engine, task string) (string, error) {
	switch e.Kind {
	case "ollama":
		return ollamaChat(e.Model, "", task, c.Defaults.OllamaURL, -1, c.Defaults.AnswerTemp, c.Defaults.OllamaTimeout)
	case "hermes":
		return runHermes(task, e.Model, c.Defaults.HermesTimeout)
	case "openrouter":
		return openRouterChat(task, e.Model, os.Getenv("OPENROUTER_API_KEY"), c.Defaults.AnswerTemp, c.Defaults.RequestTimeout)
	default:
		return "", fmt.Errorf("unknown engine %q", e.Kind)
	}
}

func (c *Config) logRoute(r *Result) {
	path := c.Defaults.LogFile
	if path == "" {
		return
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	entry := map[string]any{
		"ts":      time.Now().UTC().Format(time.RFC3339),
		"tier":    r.Tier,
		"engine":  r.Engine,
		"model":   r.Model,
		"ok":      r.OK,
		"seconds": r.Seconds,
	}
	line, _ := json.Marshal(entry)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(line, '\n'))
}
