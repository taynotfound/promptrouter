package router

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// Prereq is one checked dependency with a human readable status.
type Prereq struct {
	Name   string
	OK     bool
	Detail string
	Hint   string
}

// CheckOllama reports whether an Ollama server answers at the given base URL,
// and lists the models it has pulled.
func CheckOllama(baseURL string) (bool, []string) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(strings.TrimRight(baseURL, "/") + "/api/tags")
	if err != nil || resp == nil {
		return false, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false, nil
	}
	var out struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := decodeJSON(resp.Body, &out); err != nil {
		return true, nil
	}
	names := make([]string, 0, len(out.Models))
	for _, m := range out.Models {
		names = append(names, m.Name)
	}
	return true, names
}

// Doctor runs every prerequisite check and returns the results. The baseURL is
// the Ollama endpoint to probe; pass the config default or a chosen value.
func Doctor(baseURL string) []Prereq {
	var out []Prereq

	// Ollama server
	ok, models := CheckOllama(baseURL)
	p := Prereq{Name: "Ollama server", OK: ok}
	if ok {
		p.Detail = fmt.Sprintf("reachable at %s, %d model(s) pulled", baseURL, len(models))
	} else {
		p.Detail = "not reachable at " + baseURL
		p.Hint = "install from https://ollama.com, then: ollama serve"
	}
	out = append(out, p)

	// hermes CLI (optional cloud engine)
	hp := Prereq{Name: "hermes CLI (cloud engine)"}
	if path, err := exec.LookPath("hermes"); err == nil {
		hp.OK = true
		hp.Detail = "found at " + path
	} else {
		hp.Detail = "not found on PATH"
		hp.Hint = "optional: only needed for the hermes cloud engine"
	}
	out = append(out, hp)

	// network (for cloud engines)
	np := Prereq{Name: "Network"}
	if hasNetwork() {
		np.OK = true
		np.Detail = "online"
	} else {
		np.Detail = "offline"
		np.Hint = "cloud tiers need a connection; local tier still works"
	}
	out = append(out, np)

	return out
}

func hasNetwork() bool {
	c, err := net.DialTimeout("tcp", "1.1.1.1:443", 2*time.Second)
	if err != nil {
		return false
	}
	c.Close()
	return true
}
