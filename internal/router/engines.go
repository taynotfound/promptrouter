package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// decodeJSON reads all of r and unmarshals it into v.
func decodeJSON(r io.Reader, v any) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

// ollamaChat calls a local Ollama model via the chat API. Uses the chat
// endpoint (not generate) because some coder models early-stop on generate.
func ollamaChat(model, system, user, baseURL string, numPredict int, temp float64, timeoutSec int) (string, error) {
	body := map[string]any{
		"model":  model,
		"stream": false,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"options": map[string]any{
			"temperature": temp,
			"num_predict": numPredict,
		},
	}
	raw, err := postJSON(baseURL+"/api/chat", body, timeoutSec)
	if err != nil {
		return "", err
	}
	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("ollama: bad response: %w", err)
	}
	if out.Error != "" {
		return "", fmt.Errorf("ollama: %s", out.Error)
	}
	return strings.TrimSpace(out.Message.Content), nil
}

// runHermes shells out to the hermes headless CLI for a cloud model.
func runHermes(task, model string, timeoutSec int) (string, error) {
	if model == "" {
		return "", fmt.Errorf("hermes: model is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "hermes", "-z", task, "-m", model, "--provider", "github-copilot")
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("hermes: %s", msg)
	}
	return strings.TrimSpace(out.String()), nil
}

// openAICompatChat calls any OpenAI-compatible /chat/completions endpoint.
// This one function backs OpenAI, OpenRouter, Groq, Together, DeepSeek, a local
// vLLM or LM Studio server, and anything else that speaks the same shape: only
// the base URL and API key differ. baseURL is the API root (no trailing slash),
// for example https://api.openai.com/v1. label is used in error messages.
func openAICompatChat(label, baseURL, apiKey, model, task string, temp float64, timeoutSec int) (string, error) {
	if model == "" {
		return "", fmt.Errorf("%s: model is required", label)
	}
	url := strings.TrimRight(baseURL, "/") + "/chat/completions"
	body := map[string]any{
		"model":       model,
		"temperature": temp,
		"messages":    []map[string]string{{"role": "user", "content": task}},
	}
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("%s: HTTP %d: %s", label, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("%s: bad response: %w", label, err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("%s: empty response", label)
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

// openRouterChat calls the OpenRouter chat completions API.
func openRouterChat(task, model, apiKey string, temp float64, timeoutSec int) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("openrouter: OPENROUTER_API_KEY not set")
	}
	return openAICompatChat("openrouter", "https://openrouter.ai/api/v1", apiKey, model, task, temp, timeoutSec)
}

// anthropicChat calls the Anthropic Messages API for Claude models. Anthropic
// does not speak the OpenAI shape: it uses x-api-key, an anthropic-version
// header, a required max_tokens, and a content array in the response.
func anthropicChat(task, model, apiKey string, temp float64, timeoutSec int) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("anthropic: ANTHROPIC_API_KEY not set")
	}
	if model == "" {
		return "", fmt.Errorf("anthropic: model is required")
	}
	body := map[string]any{
		"model":       model,
		"max_tokens":  4096,
		"temperature": temp,
		"messages":    []map[string]string{{"role": "user", "content": task}},
	}
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("anthropic: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("anthropic: bad response: %w", err)
	}
	if len(out.Content) == 0 {
		return "", fmt.Errorf("anthropic: empty response")
	}
	return strings.TrimSpace(out.Content[0].Text), nil
}

func postJSON(url string, body any, timeoutSec int) ([]byte, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, url, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}
