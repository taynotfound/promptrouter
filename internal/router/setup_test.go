package router

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanRenderValidLocalOnly(t *testing.T) {
	p := Plan{
		JudgeModel: "qwen2.5:7b-instruct",
		LocalModel: "qwen3-coder:30b",
		OllamaURL:  "http://localhost:11434",
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("local-only plan should validate: %v", err)
	}
	out := p.Render()
	if !strings.Contains(out, "engine: ollama") {
		t.Error("render missing ollama engine")
	}
	if strings.Contains(out, "openrouter") {
		t.Error("local-only plan should not mention openrouter")
	}
}

func TestPlanRenderCloudFirst(t *testing.T) {
	p := Plan{
		JudgeModel:  "j",
		LocalModel:  "local",
		CloudEngine: "openrouter",
		CloudHard:   "vendor/hard",
		CloudExpert: "vendor/expert",
		OllamaURL:   "http://localhost:11434",
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("cloud plan should validate: %v", err)
	}
	out := p.Render()
	hardIdx := strings.Index(out, "HARD:")
	orIdx := strings.Index(out[hardIdx:], "openrouter")
	ollamaIdx := strings.Index(out[hardIdx:], "ollama")
	if orIdx == -1 || ollamaIdx == -1 || orIdx > ollamaIdx {
		t.Error("HARD tier should list openrouter before the ollama fallback")
	}
}

func TestSecretsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.env")
	t.Setenv("PROMPTROUTER_SECRETS", path)

	if err := WriteSecret("OPENROUTER_API_KEY", "sk-1"); err != nil {
		t.Fatal(err)
	}
	if err := WriteSecret("OTHER", "two"); err != nil {
		t.Fatal(err)
	}
	// replace existing key, keep the other
	if err := WriteSecret("OPENROUTER_API_KEY", "sk-2"); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("secrets file perms = %v, want 0600", info.Mode().Perm())
	}

	os.Unsetenv("OPENROUTER_API_KEY")
	os.Unsetenv("OTHER")
	LoadSecrets()
	if got := os.Getenv("OPENROUTER_API_KEY"); got != "sk-2" {
		t.Errorf("OPENROUTER_API_KEY = %q, want sk-2", got)
	}
	if got := os.Getenv("OTHER"); got != "two" {
		t.Errorf("OTHER = %q, want two", got)
	}
}

func TestLoadSecretsDoesNotOverrideEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.env")
	t.Setenv("PROMPTROUTER_SECRETS", path)
	if err := WriteSecret("OPENROUTER_API_KEY", "from-file"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENROUTER_API_KEY", "from-env")
	LoadSecrets()
	if got := os.Getenv("OPENROUTER_API_KEY"); got != "from-env" {
		t.Errorf("real env should win, got %q", got)
	}
}
