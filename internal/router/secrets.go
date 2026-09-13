package router

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// SecretsPath is where init writes provider tokens. It is a simple KEY=value
// file (chmod 0600), kept separate from models.yaml so the config stays
// shareable and the secrets never land in a repo.
func SecretsPath() string {
	if p := os.Getenv("PROMPTROUTER_SECRETS"); p != "" {
		return p
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "promptrouter", "secrets.env")
	}
	return "secrets.env"
}

// LoadSecrets reads the secrets file into the process environment without
// overwriting anything already set. A missing file is not an error: a user may
// export the tokens themselves. Real environment always wins.
func LoadSecrets() {
	f, err := os.Open(SecretsPath())
	if err != nil {
		return
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if key != "" && os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

// SecretValue returns the value of a key from the environment or the secrets
// file, or "" if unset. Used by the wizard to avoid re-prompting for a token
// that is already stored.
func SecretValue(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	f, err := os.Open(SecretsPath())
	if err != nil {
		return ""
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if k, v, ok := strings.Cut(line, "="); ok && strings.TrimSpace(k) == key {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}

// WriteSecret sets or replaces a single KEY in the secrets file, creating it
// with 0600 permissions. It preserves other keys and comments.
func WriteSecret(key, value string) error {
	path := SecretsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lines := []string{}
	found := false
	if raw, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, key+"=") {
				lines = append(lines, key+"="+value)
				found = true
			} else if trimmed != "" {
				lines = append(lines, line)
			}
		}
	}
	if !found {
		lines = append(lines, key+"="+value)
	}
	out := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(out), 0o600)
}
