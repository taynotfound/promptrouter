package router

import (
	"strings"
)

const judgeSys = `You are a routing classifier for coding tasks. Output exactly one token: EASY, HARD, or EXPERT.
EASY = trivial edits, boilerplate, renames, formatting, obvious fixes.
HARD = real debugging, API or system design, refactors, standard concurrency, security sensitive code, tricky logic.
EXPERT = lock-free or wait-free structures, formal proofs, distributed consensus or consistency, constant-time crypto, novel algorithm design.
When unsure between two tiers, pick the higher one. Output only the word.`

const explainSys = `You are a routing classifier for coding tasks. Decide the tier (EASY, HARD, or EXPERT).
EASY = trivial edits, boilerplate, renames, formatting, obvious fixes.
HARD = real debugging, API or system design, refactors, standard concurrency, security sensitive code, tricky logic.
EXPERT = lock-free structures, formal proofs, distributed consensus, constant-time crypto, novel algorithm design.
Respond with exactly two lines:
TIER: <EASY|HARD|EXPERT>
WHY: <one short sentence>`

var validTiers = map[string]bool{"EASY": true, "HARD": true, "EXPERT": true}

// normalizeTier pulls the first valid tier token out of a model response.
func normalizeTier(s string) string {
	up := strings.ToUpper(s)
	// Order matters: check EXPERT before EASY/HARD to avoid substring hits.
	for _, t := range []string{"EXPERT", "EASY", "HARD"} {
		if strings.Contains(up, t) {
			return t
		}
	}
	return ""
}

// Judge classifies a task into a tier. On any failure it returns the
// configured fallback tier, so routing never dies on a flaky judge.
func (c *Config) Judge(task string) string {
	out, err := ollamaChat(c.JudgeCfg.Model, judgeSys, task, c.Defaults.OllamaURL, 4, 0, c.Defaults.OllamaTimeout)
	if err == nil {
		if t := normalizeTier(out); t != "" {
			return t
		}
	}
	return c.Defaults.FallbackTier
}

// JudgeExplain returns the tier plus a one line reason. Degrades gracefully.
func (c *Config) JudgeExplain(task string) (tier, why string) {
	out, err := ollamaChat(c.JudgeCfg.Model, explainSys, task, c.Defaults.OllamaURL, 60, 0, c.Defaults.OllamaTimeout)
	if err != nil {
		return c.Defaults.FallbackTier, "judge unavailable: " + err.Error()
	}
	tier = normalizeTier(out)
	if tier == "" {
		tier = c.Defaults.FallbackTier
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "WHY") {
			if i := strings.Index(line, ":"); i >= 0 {
				why = strings.TrimSpace(line[i+1:])
			}
		}
	}
	if why == "" {
		why = "no reason given"
	}
	return tier, why
}
