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

const scoreSys = `You are a difficulty scorer for coding tasks. Output a single integer from 0 to 100 and nothing else.
0 = utterly trivial (rename a variable, fix a typo).
25 = routine edit or boilerplate a junior could do quickly.
50 = real work: debugging, an API or a standard refactor.
75 = hard: subtle concurrency, security sensitive code, tricky system design.
100 = expert: lock-free or wait-free structures, formal proofs, distributed consensus, constant-time crypto, novel algorithms.
Score the difficulty of the task. Output only the number.`

const scoreExplainSys = `You are a difficulty scorer for coding tasks. Rate difficulty from 0 (trivial) to 100 (expert research).
Respond with exactly two lines:
SCORE: <0-100>
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

// parseScore pulls the first integer 0-100 out of a model response, clamping
// to that range. Returns -1 when no number is found.
func parseScore(s string) int {
	digits := ""
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits += string(r)
			continue
		}
		if digits != "" {
			break
		}
	}
	if digits == "" {
		return -1
	}
	n := 0
	for _, r := range digits {
		n = n*10 + int(r-'0')
		if n > 100 {
			return 100
		}
	}
	return n
}

// JudgeScore returns a 0-100 difficulty score. On any failure it returns a
// middle-of-the-road 50 so routing still resolves to a sensible level.
func (c *Config) JudgeScore(task string) int {
	out, err := ollamaChat(c.JudgeCfg.Model, scoreSys, task, c.Defaults.OllamaURL, 8, 0, c.Defaults.OllamaTimeout)
	if err == nil {
		if n := parseScore(out); n >= 0 {
			return n
		}
	}
	return 50
}

// JudgeScoreExplain returns the score plus a one line reason. Degrades to 50.
func (c *Config) JudgeScoreExplain(task string) (score int, why string) {
	out, err := ollamaChat(c.JudgeCfg.Model, scoreExplainSys, task, c.Defaults.OllamaURL, 60, 0, c.Defaults.OllamaTimeout)
	if err != nil {
		return 50, "judge unavailable: " + err.Error()
	}
	score = parseScore(out)
	if score < 0 {
		score = 50
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
	return score, why
}
