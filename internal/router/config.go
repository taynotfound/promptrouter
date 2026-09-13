package router

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Engine is one backend in a tier's fallback chain.
type Engine struct {
	Kind  string `yaml:"engine"` // ollama, hermes, openrouter
	Model string `yaml:"model"`
}

// Tier is an ordered list of engines. First reachable one wins.
type Tier struct {
	Chain []Engine `yaml:"chain"`
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
}

// Judge is the fast classifier config.
type Judge struct {
	Model string `yaml:"model"`
}

// Config is the whole models.yaml.
type Config struct {
	JudgeCfg Judge           `yaml:"judge"`
	Tiers    map[string]Tier `yaml:"tiers"`
	Defaults Defaults        `yaml:"defaults"`
}

var validKinds = map[string]bool{"ollama": true, "hermes": true, "openrouter": true}

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
	if len(c.Tiers) == 0 {
		return fmt.Errorf("config: at least one tier is required")
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
}
