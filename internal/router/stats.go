package router

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Stats summarizes the routing log for `route stats`.
func (c *Config) Stats() (string, error) {
	path := c.Defaults.LogFile
	if path == "" {
		return "", fmt.Errorf("no log_file set in models.yaml (defaults.log_file)")
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("no routes logged yet at %s", path)
	}
	defer f.Close()

	var total, ok, local int
	tiers := map[string]int{}
	engines := map[string]int{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e struct {
			Tier   string `json:"tier"`
			Engine string `json:"engine"`
			Model  string `json:"model"`
			OK     bool   `json:"ok"`
		}
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		total++
		tiers[e.Tier]++
		if e.OK {
			ok++
			engines[e.Engine+"/"+e.Model]++
			if e.Engine == "ollama" {
				local++
			}
		}
	}
	if total == 0 {
		return "log is empty", nil
	}
	pct := 0
	if ok > 0 {
		pct = local * 100 / ok
	}
	var b strings.Builder
	fmt.Fprintf(&b, "routes: %d  ok: %d  local (free): %d (%d%% of successful)\n", total, ok, local, pct)
	fmt.Fprintf(&b, "by tier:   %s\n", topCounts(tiers))
	fmt.Fprintf(&b, "by engine: %s", topCounts(engines))
	return b.String(), nil
}

func topCounts(m map[string]int) string {
	type kv struct {
		k string
		v int
	}
	var s []kv
	for k, v := range m {
		s = append(s, kv{k, v})
	}
	sort.Slice(s, func(i, j int) bool { return s[i].v > s[j].v })
	var parts []string
	for _, e := range s {
		parts = append(parts, fmt.Sprintf("%s %d", e.k, e.v))
	}
	return strings.Join(parts, ", ")
}
