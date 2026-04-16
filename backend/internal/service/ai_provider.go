package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/santapong/KeepSave/backend/internal/models"
)

// AIProvider defines the interface for AI model providers.
type AIProvider interface {
	Name() string
	Chat(systemPrompt, userPrompt string) (string, error)
	Available() bool
	ModelName() string
}

// AIProviderManager manages multiple AI providers and selects the best available one.
type AIProviderManager struct {
	providers []AIProvider
	preferred string
}

// NewAIProviderManager creates a new provider manager, auto-detecting configured providers.
func NewAIProviderManager() *AIProviderManager {
	mgr := &AIProviderManager{}

	// Register providers in priority order
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		model := envOr("ANTHROPIC_MODEL", "claude-sonnet-4-20250514")
		mgr.providers = append(mgr.providers, NewClaudeProvider(key, model))
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		model := envOr("OPENAI_MODEL", "gpt-4o")
		baseURL := os.Getenv("OPENAI_BASE_URL")
		mgr.providers = append(mgr.providers, NewOpenAICompatProvider("openai", key, model, baseURL, "https://api.openai.com/v1"))
	}
	if key := os.Getenv("GOOGLE_API_KEY"); key != "" {
		model := envOr("GOOGLE_MODEL", "gemini-2.0-flash")
		mgr.providers = append(mgr.providers, NewGeminiProvider(key, model))
	}
	if key := os.Getenv("GROQ_API_KEY"); key != "" {
		model := envOr("GROQ_MODEL", "llama-3.3-70b-versatile")
		mgr.providers = append(mgr.providers, NewOpenAICompatProvider("groq", key, model, "", "https://api.groq.com/openai/v1"))
	}
	if key := os.Getenv("MISTRAL_API_KEY"); key != "" {
		model := envOr("MISTRAL_MODEL", "mistral-large-latest")
		mgr.providers = append(mgr.providers, NewOpenAICompatProvider("mistral", key, model, "", "https://api.mistral.ai/v1"))
	}
	if url := os.Getenv("OLLAMA_BASE_URL"); url != "" {
		model := envOr("OLLAMA_MODEL", "llama3.1")
		mgr.providers = append(mgr.providers, NewOllamaProvider(url, model))
	}

	mgr.preferred = os.Getenv("AI_PREFERRED_PROVIDER")
	return mgr
}

// Chat sends a request to the best available provider. Returns response, provider name, model name, error.
func (m *AIProviderManager) Chat(systemPrompt, userPrompt string) (string, string, string, error) {
	if m.preferred != "" {
		for _, p := range m.providers {
			if strings.EqualFold(p.Name(), m.preferred) && p.Available() {
				resp, err := p.Chat(systemPrompt, userPrompt)
				if err == nil {
					return resp, p.Name(), p.ModelName(), nil
				}
			}
		}
	}
	for _, p := range m.providers {
		if p.Available() {
			resp, err := p.Chat(systemPrompt, userPrompt)
			if err == nil {
				return resp, p.Name(), p.ModelName(), nil
			}
		}
	}
	return "", "", "", fmt.Errorf("no AI provider available; set one of: ANTHROPIC_API_KEY, OPENAI_API_KEY, GOOGLE_API_KEY, GROQ_API_KEY, MISTRAL_API_KEY, OLLAMA_BASE_URL")
}

// ListProviders returns status of all configured providers.
func (m *AIProviderManager) ListProviders() []models.AIProviderStatus {
	var out []models.AIProviderStatus
	for _, p := range m.providers {
		out = append(out, models.AIProviderStatus{Provider: p.Name(), Available: p.Available(), Model: p.ModelName()})
	}
	return out
}

// HasProvider returns true if at least one provider is configured.
func (m *AIProviderManager) HasProvider() bool {
	for _, p := range m.providers {
		if p.Available() {
			return true
		}
	}
	return false
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ---------------------------------------------------------------------------
// Claude (Anthropic)
// ---------------------------------------------------------------------------

type ClaudeProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewClaudeProvider(apiKey, model string) *ClaudeProvider {
	return &ClaudeProvider{apiKey: apiKey, model: model, client: &http.Client{Timeout: 60 * time.Second}}
}

func (p *ClaudeProvider) Name() string     { return "claude" }
func (p *ClaudeProvider) ModelName() string { return p.model }
func (p *ClaudeProvider) Available() bool   { return p.apiKey != "" }

func (p *ClaudeProvider) Chat(systemPrompt, userPrompt string) (string, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"model": p.model, "max_tokens": 4096, "system": systemPrompt,
		"messages": []map[string]string{{"role": "user", "content": userPrompt}},
	})
	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("claude API error %d: %s", resp.StatusCode, string(data))
	}
	var result struct {
		Content []struct{ Text string `json:"text"` } `json:"content"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response from Claude")
	}
	return result.Content[0].Text, nil
}

// ---------------------------------------------------------------------------
// OpenAI-compatible provider (OpenAI, Groq, Mistral)
// ---------------------------------------------------------------------------

type OpenAICompatProvider struct {
	name    string
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

func NewOpenAICompatProvider(name, apiKey, model, customURL, defaultURL string) *OpenAICompatProvider {
	base := defaultURL
	if customURL != "" {
		base = customURL
	}
	return &OpenAICompatProvider{name: name, apiKey: apiKey, model: model, baseURL: base, client: &http.Client{Timeout: 60 * time.Second}}
}

func (p *OpenAICompatProvider) Name() string     { return p.name }
func (p *OpenAICompatProvider) ModelName() string { return p.model }
func (p *OpenAICompatProvider) Available() bool   { return p.apiKey != "" }

func (p *OpenAICompatProvider) Chat(systemPrompt, userPrompt string) (string, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"model": p.model, "max_tokens": 4096,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	})
	req, err := http.NewRequest("POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("%s API error %d: %s", p.name, resp.StatusCode, string(data))
	}
	var result struct {
		Choices []struct {
			Message struct{ Content string `json:"content"` } `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response from %s", p.name)
	}
	return result.Choices[0].Message.Content, nil
}

// ---------------------------------------------------------------------------
// Google Gemini
// ---------------------------------------------------------------------------

type GeminiProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewGeminiProvider(apiKey, model string) *GeminiProvider {
	return &GeminiProvider{apiKey: apiKey, model: model, client: &http.Client{Timeout: 60 * time.Second}}
}

func (p *GeminiProvider) Name() string     { return "gemini" }
func (p *GeminiProvider) ModelName() string { return p.model }
func (p *GeminiProvider) Available() bool   { return p.apiKey != "" }

func (p *GeminiProvider) Chat(systemPrompt, userPrompt string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.model, p.apiKey)
	body, _ := json.Marshal(map[string]interface{}{
		"system_instruction": map[string]interface{}{"parts": []map[string]string{{"text": systemPrompt}}},
		"contents":          []map[string]interface{}{{"parts": []map[string]string{{"text": userPrompt}}}},
		"generationConfig":  map[string]interface{}{"maxOutputTokens": 4096},
	})
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("gemini API error %d: %s", resp.StatusCode, string(data))
	}
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct{ Text string `json:"text"` } `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}
	return result.Candidates[0].Content.Parts[0].Text, nil
}

// ---------------------------------------------------------------------------
// Ollama (local)
// ---------------------------------------------------------------------------

type OllamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	return &OllamaProvider{baseURL: strings.TrimRight(baseURL, "/"), model: model, client: &http.Client{Timeout: 120 * time.Second}}
}

func (p *OllamaProvider) Name() string     { return "ollama" }
func (p *OllamaProvider) ModelName() string { return p.model }
func (p *OllamaProvider) Available() bool {
	resp, err := p.client.Get(p.baseURL + "/api/tags")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func (p *OllamaProvider) Chat(systemPrompt, userPrompt string) (string, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"model": p.model, "stream": false,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	})
	req, err := http.NewRequest("POST", p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("ollama API error %d: %s", resp.StatusCode, string(data))
	}
	var result struct {
		Message struct{ Content string `json:"content"` } `json:"message"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	return result.Message.Content, nil
}
