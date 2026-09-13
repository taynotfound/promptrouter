package router

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "models.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const goodConfig = `
judge:
  model: judge-model
tiers:
  EASY:
    chain:
      - engine: ollama
        model: local-a
      - engine: ollama
        model: local-b
  HARD:
    chain:
      - engine: hermes
        model: cloud-a
defaults:
  fallback_tier: HARD
`

func TestLoadValid(t *testing.T) {
	c, err := Load(writeConfig(t, goodConfig))
	if err != nil {
		t.Fatal(err)
	}
	if c.JudgeCfg.Model != "judge-model" {
		t.Errorf("judge model = %q", c.JudgeCfg.Model)
	}
	if len(c.Tiers["EASY"].Chain) != 2 {
		t.Errorf("EASY chain len = %d", len(c.Tiers["EASY"].Chain))
	}
	if c.Defaults.OllamaURL == "" {
		t.Error("defaults not applied")
	}
}

func TestLoadRejectsBadEngine(t *testing.T) {
	bad := `
judge:
  model: j
tiers:
  EASY:
    chain:
      - engine: notreal
        model: x
`
	if _, err := Load(writeConfig(t, bad)); err == nil {
		t.Fatal("expected error for unknown engine")
	}
}

func TestLoadRejectsMissingJudge(t *testing.T) {
	bad := `
tiers:
  EASY:
    chain:
      - engine: ollama
        model: x
`
	if _, err := Load(writeConfig(t, bad)); err == nil {
		t.Fatal("expected error for missing judge")
	}
}

func TestLoadRejectsBadFallbackTier(t *testing.T) {
	bad := `
judge:
  model: j
tiers:
  EASY:
    chain:
      - engine: ollama
        model: x
defaults:
  fallback_tier: NOPE
`
	if _, err := Load(writeConfig(t, bad)); err == nil {
		t.Fatal("expected error for bad fallback_tier")
	}
}

func TestNormalizeTier(t *testing.T) {
	if got := normalizeTier("EASY"); got != "EASY" {
		t.Errorf("EASY -> %q", got)
	}
	if got := normalizeTier("this needs EXPERT care"); got != "EXPERT" {
		t.Errorf("expert -> %q", got)
	}
	if got := normalizeTier("just HARD"); got != "HARD" {
		t.Errorf("hard -> %q", got)
	}
	if got := normalizeTier("nothing here"); got != "" {
		t.Errorf("empty -> %q", got)
	}
}

func TestDispatchEmptyTask(t *testing.T) {
	c, _ := Load(writeConfig(t, goodConfig))
	res := c.Dispatch("   ", Options{})
	if res.OK || res.Err != "empty task" {
		t.Errorf("expected empty task rejection, got %+v", res)
	}
}

func TestStatsReadsLog(t *testing.T) {
	c, _ := Load(writeConfig(t, goodConfig))
	dir := t.TempDir()
	logp := filepath.Join(dir, "routes.jsonl")
	c.Defaults.LogFile = logp
	lines := `{"tier":"EASY","engine":"ollama","model":"local-a","ok":true}
{"tier":"HARD","engine":"hermes","model":"cloud-a","ok":true}
{"tier":"EASY","engine":"ollama","model":"local-a","ok":true}
`
	os.WriteFile(logp, []byte(lines), 0o644)
	out, err := c.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if !contains(out, "routes: 3") || !contains(out, "local (free): 2") {
		t.Errorf("stats output wrong:\n%s", out)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
