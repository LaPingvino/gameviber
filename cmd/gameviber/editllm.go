package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// A key on this computer (LOOP item 5d): `gameviber edit` reads a provider's
// key from its environment and forwards the editor's chat requests with it,
// so the key never reaches the page nor the project. Only to the endpoint the
// environment names, never one the page asks for.
//
//	ANTHROPIC_API_KEY  (ANTHROPIC_BASE_URL, GAMEVIBER_ANTHROPIC_MODEL)
//	OPENAI_API_KEY     (OPENAI_BASE_URL — OpenAI, OpenRouter, any OpenAI-compatible —, GAMEVIBER_OPENAI_MODEL)

type llmProvider struct {
	ID    string `json:"id"` // anthropic | openai
	Label string `json:"label"`
	Base  string `json:"base"`
	Model string `json:"model"`
	key   string
}

func envLLMProviders() []llmProvider {
	get := func(name, def string) string {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return v
		}
		return def
	}
	var out []llmProvider
	if k := get("ANTHROPIC_API_KEY", ""); k != "" {
		out = append(out, llmProvider{ID: "anthropic", Label: "Anthropic — the key on this computer",
			Base:  strings.TrimSuffix(get("ANTHROPIC_BASE_URL", "https://api.anthropic.com"), "/") + "/v1",
			Model: get("GAMEVIBER_ANTHROPIC_MODEL", "claude-sonnet-5-5"), key: k})
	}
	if k := get("OPENAI_API_KEY", ""); k != "" {
		out = append(out, llmProvider{ID: "openai", Label: "OpenAI-compatible — the key on this computer",
			Base:  strings.TrimSuffix(get("OPENAI_BASE_URL", "https://api.openai.com/v1"), "/"),
			Model: get("GAMEVIBER_OPENAI_MODEL", "gpt-5"), key: k})
	}
	return out
}

func llmHandlers(mux *http.ServeMux) {
	// GET api/llm: the providers with a key on this computer (never the keys).
	mux.HandleFunc("GET /api/llm", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"providers": envLLMProviders()})
	})
	// POST api/llm?prov=anthropic|openai: the request body (Messages API /
	// chat completions JSON) sent on with the key; the answer passed back.
	mux.HandleFunc("POST /api/llm", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("prov")
		var p *llmProvider
		for _, c := range envLLMProviders() {
			if c.ID == id {
				p = &c
				break
			}
		}
		if p == nil {
			http.Error(w, "no key for "+id+" on this computer (ANTHROPIC_API_KEY / OPENAI_API_KEY where gameviber edit runs)", http.StatusNotFound)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
		if err != nil || !json.Valid(body) {
			http.Error(w, "the request is not JSON", http.StatusBadRequest)
			return
		}
		url := p.Base + "/chat/completions"
		if p.ID == "anthropic" {
			url = p.Base + "/messages"
		}
		req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, url, strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		if p.ID == "anthropic" {
			req.Header.Set("x-api-key", p.key)
			req.Header.Set("anthropic-version", "2023-06-01")
		} else {
			req.Header.Set("Authorization", "Bearer "+p.key)
		}
		res, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
		if err != nil {
			http.Error(w, "the model didn't answer: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer res.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(res.StatusCode)
		io.Copy(w, res.Body)
	})
}
