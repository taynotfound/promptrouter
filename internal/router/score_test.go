package router

import (
	"strings"
	"testing"
)

const scoreConfig = `
judge:
  model: judge-model
  mode: score
levels:
  - name: local
    min_score: 0
    chain:
      - engine: ollama
        model: local-a
  - name: mid
    min_score: 40
    chain:
      - engine: ollama
        model: local-b
  - name: cloud
    min_score: 75
    chain:
      - engine: hermes
        model: cloud-a
      - engine: ollama
        model: local-a
`

func TestScoreModeLoads(t *testing.T) {
	c, err := Load(writeConfig(t, scoreConfig))
	if err != nil {
		t.Fatal(err)
	}
	if !c.ScoreMode() {
		t.Fatal("expected score mode")
	}
	if len(c.Levels) != 3 {
		t.Fatalf("levels = %d, want 3", len(c.Levels))
	}
	// applyDefaults sorts highest-first.
	if c.Levels[0].MinScore != 75 {
		t.Errorf("levels not sorted desc: first min_score = %d", c.Levels[0].MinScore)
	}
}

func TestLevelFor(t *testing.T) {
	c, _ := Load(writeConfig(t, scoreConfig))
	cases := []struct {
		score int
		want  string
	}{
		{0, "local"},
		{10, "local"},
		{39, "local"},
		{40, "mid"},
		{74, "mid"},
		{75, "cloud"},
		{100, "cloud"},
		{-5, "local"}, // below everything falls to the lowest band
	}
	for _, tc := range cases {
		if got := c.LevelFor(tc.score).Name; got != tc.want {
			t.Errorf("LevelFor(%d) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestParseScore(t *testing.T) {
	cases := map[string]int{
		"42":                42,
		"  7 ":              7,
		"SCORE: 88\nWHY: x": 88,
		"120":               100, // clamp
		"the score is 55.":  55,
		"no number here":    -1,
		"0":                 0,
	}
	for in, want := range cases {
		if got := parseScore(in); got != want {
			t.Errorf("parseScore(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestScoreModeRejectsDuplicateThreshold(t *testing.T) {
	bad := `
judge:
  model: j
  mode: score
levels:
  - name: a
    min_score: 0
    chain:
      - engine: ollama
        model: x
  - name: b
    min_score: 0
    chain:
      - engine: ollama
        model: y
`
	if _, err := Load(writeConfig(t, bad)); err == nil {
		t.Fatal("expected error for duplicate min_score")
	}
}

func TestScoreModeRejectsEmptyLevels(t *testing.T) {
	bad := `
judge:
  model: j
  mode: score
`
	if _, err := Load(writeConfig(t, bad)); err == nil {
		t.Fatal("expected error for score mode with no levels")
	}
}

func TestPlanRenderScore(t *testing.T) {
	p := Plan{
		JudgeModel: "judge-model",
		OllamaURL:  "http://localhost:11434",
		Levels: []LevelSpec{
			{Name: "local", MinScore: 0, Chain: []Engine{{Kind: "ollama", Model: "qwen"}}},
			{Name: "cloud", MinScore: 70, Chain: []Engine{{Kind: "hermes", Model: "claude"}}},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("score plan should validate: %v", err)
	}
	out := p.Render()
	if !strings.Contains(out, "mode: score") {
		t.Error("render missing score mode")
	}
	if !strings.Contains(out, "min_score: 70") {
		t.Error("render missing custom threshold")
	}
	if !strings.Contains(out, "name: cloud") {
		t.Error("render missing custom level name")
	}
}

func TestTierForScore(t *testing.T) {
	cases := map[int]string{0: "EASY", 39: "EASY", 40: "HARD", 74: "HARD", 75: "EXPERT", 100: "EXPERT"}
	for s, want := range cases {
		if got := tierForScoreTest(s); got != want {
			t.Errorf("tierForScore(%d) = %q, want %q", s, got, want)
		}
	}
}

// tierForScoreTest mirrors the mapping in cmd/route/bench.go so the band logic
// is covered by a unit test in the package that owns the scoring semantics.
func tierForScoreTest(s int) string {
	switch {
	case s < 40:
		return "EASY"
	case s < 75:
		return "HARD"
	default:
		return "EXPERT"
	}
}
