package router

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Engine is one backend in a tier's fallback chain.
type Engine struct {
	Kind  string `yaml:"engine"` // ollama, hermes, openrouter, openai, anthropic
	Model string `yaml:"model"`
	// BaseURL overrides the API root for the openai kind, so one engine type
	// covers OpenAI, Groq, Together, DeepSeek, a local vLLM or LM Studio
	// server, and any other OpenAI-compatible API. Ignored by other kinds.
	BaseURL string `yaml:"base_url,omitempty"`
	// KeyEnv overrides which environment variable holds the API key. Defaults
	// per kind (OPENAI_API_KEY, ANTHROPIC_API_KEY, OPENROUTER_API_KEY).
	KeyEnv string `yaml:"key_env,omitempty"`
}

// Tier is an ordered list of engines. First reachable one wins.
type Tier struct {
	Chain []Engine `yaml:"chain"`
}

// Level is a user-defined difficulty band for score-based routing. When the
// judge returns a 0-100 score, the level with the highest MinScore that is
// still <= the score wins. Names, thresholds, and chains are all user-defined,
// so a config can have as many or as few levels as it likes.
type Level struct {
	Name     string   `yaml:"name"`
	MinScore int      `yaml:"min_score"`
	Chain    []Engine `yaml:"chain"`
}

// Defaults holds tunables that apply across engines.
type Defaults struct {
	FallbackTier   string  `yaml:"fallback_tier"`
	OllamaURL      string  `yaml:"ollama_url"`
	MaxRetries     int     `yaml:"max_retries"`
	RetryBackoff   float64 `yaml:"retry_backoff"`
	OllamaTimeout  int     `yaml:"ollama_timeout"`
	HermesTimeout  int     `yaml:"hermes_timeout"`
	RequestTimeout int     `yaml:"request_timeout"`
	AnswerTemp     float64 `yaml:"answer_temperature"`
	LogFile        string  `yaml:"log_file"`
	// OpenAIBaseURL and OpenAIKeyEnv are global defaults for every openai
	// engine that does not set its own base_url or key_env. Point them once at
	// a provider (Groq, Together, a local vLLM server) and every openai engine
	// inherits it; a per-engine value still wins over these.
	OpenAIBaseURL string `yaml:"openai_base_url"`
	OpenAIKeyEnv  string `yaml:"openai_key_env"`
}

// Judge is the fast classifier config. Mode is "tier" (categorical, the
// default) or "score" (numeric 0-100 mapped onto user-defined levels).
type Judge struct {
	Model string `yaml:"model"`
	Mode  string `yaml:"mode"`
}

// Config is the whole models.yaml.
type Config struct {
	JudgeCfg Judge           `yaml:"judge"`
	Tiers    map[string]Tier `yaml:"tiers"`
	Levels   []Level         `yaml:"levels"`
	Defaults Defaults        `yaml:"defaults"`
}

// ScoreMode reports whether this config routes by numeric score and levels
// instead of by categorical tiers.
func (c *Config) ScoreMode() bool {
	return strings.EqualFold(c.JudgeCfg.Mode, "score") && len(c.Levels) > 0
}

var validKinds = map[string]bool{"ollama": true, "hermes": true, "openrouter": true, "openai": true, "anthropic": true}

// Load reads and validates a config file. A bad file fails here with a clear
// message instead of a nil-map panic deep in a route.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	c.applyDefaults()
	return &c, nil
}

func (c *Config) validate() error {
	if c.JudgeCfg.Model == "" {
		return fmt.Errorf("config: judge.model is required")
	}
	scoreMode := strings.EqualFold(c.JudgeCfg.Mode, "score")
	if scoreMode {
		return c.validateLevels()
	}
	if len(c.Tiers) == 0 {
		return fmt.Errorf("config: at least one tier is required (or use judge.mode: score with levels)")
	}
	for name, t := range c.Tiers {
		if len(t.Chain) == 0 {
			return fmt.Errorf("config: tier %q has an empty chain", name)
		}
		for i, e := range t.Chain {
			if !validKinds[e.Kind] {
				return fmt.Errorf("config: tier %q step %d: unknown engine %q", name, i, e.Kind)
			}
		}
	}
	ft := c.Defaults.FallbackTier
	if ft != "" {
		if _, ok := c.Tiers[ft]; !ok {
			return fmt.Errorf("config: fallback_tier %q is not a defined tier", ft)
		}
	}
	return nil
}

// validateLevels checks a score-mode config: at least one level, unique
// non-negative thresholds, non-empty chains, and known engines.
func (c *Config) validateLevels() error {
	if len(c.Levels) == 0 {
		return fmt.Errorf("config: judge.mode is score but no levels are defined")
	}
	seen := map[int]bool{}
	for i, l := range c.Levels {
		if l.Name == "" {
			return fmt.Errorf("config: level %d is missing a name", i)
		}
		if l.MinScore < 0 || l.MinScore > 100 {
			return fmt.Errorf("config: level %q min_score %d out of range 0-100", l.Name, l.MinScore)
		}
		if seen[l.MinScore] {
			return fmt.Errorf("config: two levels share min_score %d", l.MinScore)
		}
		seen[l.MinScore] = true
		if len(l.Chain) == 0 {
			return fmt.Errorf("config: level %q has an empty chain", l.Name)
		}
		for j, e := range l.Chain {
			if !validKinds[e.Kind] {
				return fmt.Errorf("config: level %q step %d: unknown engine %q", l.Name, j, e.Kind)
			}
		}
	}
	return nil
}

func (c *Config) applyDefaults() {
	d := &c.Defaults
	if d.FallbackTier == "" {
		d.FallbackTier = "HARD"
	}
	if d.OllamaURL == "" {
		d.OllamaURL = "http://localhost:11434"
	}
	if d.MaxRetries == 0 {
		d.MaxRetries = 1
	}
	if d.OllamaTimeout == 0 {
		d.OllamaTimeout = 600
	}
	if d.HermesTimeout == 0 {
		d.HermesTimeout = 600
	}
	if d.RequestTimeout == 0 {
		d.RequestTimeout = 600
	}
	// Keep levels ordered by threshold, highest first, so LevelFor can pick
	// the first band the score clears.
	if len(c.Levels) > 0 {
		sort.SliceStable(c.Levels, func(i, j int) bool {
			return c.Levels[i].MinScore > c.Levels[j].MinScore
		})
	}
}

// LevelFor returns the level a numeric score falls into: the highest band whose
// min_score the score clears. If nothing matches (score below every band) the
// lowest band is used, so routing always resolves.
func (c *Config) LevelFor(score int) Level {
	for _, l := range c.Levels {
		if score >= l.MinScore {
			return l
		}
	}
	return c.Levels[len(c.Levels)-1]
}
