package prober

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"llm-api-uptime/internal/domain"
)

func TestOpenAIProbe_ModelsThenStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"}]}`))
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)
			time.Sleep(15 * time.Millisecond)
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"1\"}}]}\n\n")
			flusher.Flush()
			time.Sleep(40 * time.Millisecond)
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\" 2 3\"}}],\"usage\":{\"completion_tokens\":11}}\n\n")
			flusher.Flush()
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := New()
	hb := p.Probe(context.Background(), domain.Monitor{
		ID:               "m1",
		ProviderType:     domain.ProviderOpenAI,
		BaseURL:          srv.URL + "/v1",
		APIKey:           "sk-secret-key",
		ModelID:          "gpt-4o-mini",
		CheckModelsFirst: true,
		StreamEnabled:    true,
		MaxTokens:        24,
		TimeoutSec:       10,
		Prompt:           "Count 1 to 3",
	})

	if hb.Status != domain.StatusUp {
		t.Fatalf("expected status up, got %s (err=%s)", hb.Status, hb.ErrorMessage)
	}
	if hb.ModelsOK == nil || !*hb.ModelsOK {
		t.Fatalf("expected models_ok=true")
	}
	if hb.TTFTMs == nil || *hb.TTFTMs < 10 {
		t.Fatalf("expected ttft_ms >= 10, got %v", hb.TTFTMs)
	}
	if hb.OutputTokens == nil || *hb.OutputTokens != 11 {
		t.Fatalf("expected output_tokens=11, got %v", hb.OutputTokens)
	}
	if hb.TokensPerSec == nil || *hb.TokensPerSec <= 0 {
		t.Fatalf("expected positive tokens_per_sec, got %v", hb.TokensPerSec)
	}
	if hb.TPSMethod != "usage" {
		t.Fatalf("expected tps_method=usage, got %s", hb.TPSMethod)
	}
}

func TestOpenAIProbe_ModelMissingSkipsCompletion(t *testing.T) {
	var completionCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"other-model"}]}`))
		case "/v1/chat/completions":
			completionCalls.Add(1)
		}
	}))
	defer srv.Close()

	p := New()
	hb := p.Probe(context.Background(), domain.Monitor{
		ID:               "m2",
		ProviderType:     domain.ProviderOpenAI,
		BaseURL:          srv.URL + "/v1",
		ModelID:          "gpt-4o-mini",
		CheckModelsFirst: true,
		StreamEnabled:    true,
		TimeoutSec:       5,
	})

	if hb.Status != domain.StatusDown {
		t.Fatalf("expected status down, got %s", hb.Status)
	}
	if hb.StageFailed != "model_missing" {
		t.Fatalf("expected stage model_missing, got %s", hb.StageFailed)
	}
	if completionCalls.Load() != 0 {
		t.Fatalf("expected completion not to be called, got %d calls", completionCalls.Load())
	}
}

func TestOpenAIProbe_ModelsOKButCompletion429IsDegraded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"}]}`))
		case "/v1/chat/completions":
			http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
		}
	}))
	defer srv.Close()

	p := New()
	hb := p.Probe(context.Background(), domain.Monitor{
		ID:               "m3",
		ProviderType:     domain.ProviderOpenAI,
		BaseURL:          srv.URL + "/v1",
		APIKey:           "sk-super-secret-value",
		ModelID:          "gpt-4o-mini",
		CheckModelsFirst: true,
		StreamEnabled:    true,
		TimeoutSec:       5,
	})

	if hb.Status != domain.StatusDegraded {
		t.Fatalf("expected status degraded, got %s", hb.Status)
	}
	if hb.HTTPStatus != http.StatusTooManyRequests {
		t.Fatalf("expected http_status 429, got %d", hb.HTTPStatus)
	}
}

func TestAnthropicProbe_ModelsAndStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			if r.Header.Get("x-api-key") != "ant-key" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"claude-3-5-sonnet-latest"}],"has_more":false}`))
		case "/v1/messages":
			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)
			fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\"}\n\n")
			flusher.Flush()
			time.Sleep(12 * time.Millisecond)
			fmt.Fprint(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"1 2\"}}\n\n")
			flusher.Flush()
			time.Sleep(30 * time.Millisecond)
			fmt.Fprint(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":9}}\n\n")
			flusher.Flush()
			fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
			flusher.Flush()
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := New()
	hb := p.Probe(context.Background(), domain.Monitor{
		ID:               "m4",
		ProviderType:     domain.ProviderAnthropic,
		BaseURL:          srv.URL,
		APIKey:           "ant-key",
		ModelID:          "claude-3-5-sonnet-latest",
		CheckModelsFirst: true,
		StreamEnabled:    true,
		MaxTokens:        24,
		TimeoutSec:       10,
	})

	if hb.Status != domain.StatusUp {
		t.Fatalf("expected status up, got %s (err=%s)", hb.Status, hb.ErrorMessage)
	}
	if hb.OutputTokens == nil || *hb.OutputTokens != 9 {
		t.Fatalf("expected output_tokens=9, got %v", hb.OutputTokens)
	}
	if hb.TokensPerSec == nil || *hb.TokensPerSec <= 0 {
		t.Fatalf("expected positive tps, got %v", hb.TokensPerSec)
	}
}

func TestUserAgentForwardedOnBothStages(t *testing.T) {
	const wantUA = "claude-cli/1.0.0 (external, cli)"
	var modelsUA, completionUA string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			modelsUA = r.Header.Get("User-Agent")
			_, _ = w.Write([]byte(`{"data":[{"id":"claude-3-5-sonnet-latest"}]}`))
		case "/v1/messages":
			completionUA = r.Header.Get("User-Agent")
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\"}\n\n")
			fmt.Fprint(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"ok\"}}\n\n")
			fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		}
	}))
	defer srv.Close()

	p := New()
	hb := p.Probe(context.Background(), domain.Monitor{
		ID:               "m-ua",
		ProviderType:     domain.ProviderAnthropic,
		BaseURL:          srv.URL,
		ModelID:          "claude-3-5-sonnet-latest",
		UserAgent:        wantUA,
		CheckModelsFirst: true,
		StreamEnabled:    true,
		MaxTokens:        16,
		TimeoutSec:       5,
	})
	if hb.Status != domain.StatusUp {
		t.Fatalf("expected up, got %s (%s)", hb.Status, hb.ErrorMessage)
	}
	if modelsUA != wantUA {
		t.Fatalf("expected /models User-Agent %q, got %q", wantUA, modelsUA)
	}
	if completionUA != wantUA {
		t.Fatalf("expected completion User-Agent %q, got %q", wantUA, completionUA)
	}
}

func TestOpenAIProbe_ResponsesAPIStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5-mini"}]}`))
		case "/v1/responses":
			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)
			time.Sleep(15 * time.Millisecond)
			fmt.Fprint(w, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"1 2\"}\n\n")
			flusher.Flush()
			time.Sleep(35 * time.Millisecond)
			fmt.Fprint(w, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"output_tokens\":10}}}\n\n")
			flusher.Flush()
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := New()
	hb := p.Probe(context.Background(), domain.Monitor{
		ID:               "m-resp",
		ProviderType:     domain.ProviderOpenAI,
		OpenAIEndpoint:   "responses",
		BaseURL:          srv.URL + "/v1",
		ModelID:          "gpt-5-mini",
		CheckModelsFirst: true,
		StreamEnabled:    true,
		MaxTokens:        24,
		TimeoutSec:       10,
	})
	if hb.Status != domain.StatusUp {
		t.Fatalf("expected status up, got %s (err=%s)", hb.Status, hb.ErrorMessage)
	}
	if hb.OutputTokens == nil || *hb.OutputTokens != 10 {
		t.Fatalf("expected output_tokens=10, got %v", hb.OutputTokens)
	}
	if hb.TPSMethod != "usage" {
		t.Fatalf("expected tps_method=usage, got %s", hb.TPSMethod)
	}
	if hb.TokensPerSec == nil || *hb.TokensPerSec <= 0 {
		t.Fatalf("expected positive tps, got %v", hb.TokensPerSec)
	}
}
