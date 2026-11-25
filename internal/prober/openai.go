package prober

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"llm-api-uptime/internal/domain"
)

func (p *Prober) discoverOpenAIModels(ctx context.Context, baseURL, apiKey, userAgent string, headers map[string]string) ([]ModelInfo, int64, error) {
	models, lat, _, err := p.fetchOpenAIModelsWithStatus(ctx, baseURL, apiKey, userAgent, headers)
	return models, lat, err
}

func (p *Prober) fetchOpenAIModelsWithStatus(ctx context.Context, baseURL, apiKey, userAgent string, headers map[string]string) ([]ModelInfo, int64, int, error) {
	url := strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, 0, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Accept", "application/json")
	applyRequestHeaders(req, userAgent, headers)

	start := time.Now()
	resp, err := p.client.Do(req)
	lat := time.Since(start).Milliseconds()
	if err != nil {
		return nil, lat, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, lat, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, readErrorBody(resp.Body))
	}

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, lat, resp.StatusCode, fmt.Errorf("decode /models: %w", err)
	}

	var envelope struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && len(envelope.Data) > 0 {
		return envelope.Data, lat, resp.StatusCode, nil
	}

	var direct []ModelInfo
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct, lat, resp.StatusCode, nil
	}
	return envelope.Data, lat, resp.StatusCode, nil
}

func (p *Prober) probeOpenAICompletion(ctx context.Context, m domain.Monitor) (completionResult, error) {
	if m.OpenAIEndpoint == "responses" {
		return p.probeOpenAIResponses(ctx, m)
	}
	if m.StreamEnabled {
		res, bodyText, err := p.doOpenAIStream(ctx, m, true, false)
		if err != nil && res.httpStatus == http.StatusBadRequest {
			low := strings.ToLower(bodyText)
			// Retry without stream_options if provider rejects it.
			if strings.Contains(low, "stream_options") {
				res, bodyText, err = p.doOpenAIStream(ctx, m, false, false)
			}
			// Retry with max_completion_tokens for newer OpenAI reasoning models.
			if err != nil && res.httpStatus == http.StatusBadRequest && strings.Contains(strings.ToLower(bodyText), "max_completion_tokens") {
				res, bodyText, err = p.doOpenAIStream(ctx, m, true, true)
				if err != nil && res.httpStatus == http.StatusBadRequest && strings.Contains(strings.ToLower(bodyText), "stream_options") {
					res, bodyText, err = p.doOpenAIStream(ctx, m, false, true)
				}
			}
			if err != nil && m.FallbackNonStream && isStreamUnsupportedErr(res.httpStatus, bodyText) {
				return p.doOpenAINonStream(ctx, m, false)
			}
		}
		return res, err
	}
	return p.doOpenAINonStream(ctx, m, false)
}

func (p *Prober) doOpenAIStream(ctx context.Context, m domain.Monitor, includeUsage, useMaxCompletionTokens bool) (completionResult, string, error) {
	url := strings.TrimRight(strings.TrimSpace(m.BaseURL), "/") + "/chat/completions"
	payload := map[string]any{
		"model": m.ModelID,
		"messages": []map[string]string{
			{"role": "user", "content": m.Prompt},
		},
		"stream": true,
	}
	if useMaxCompletionTokens {
		payload["max_completion_tokens"] = m.MaxTokens
	} else {
		payload["max_tokens"] = m.MaxTokens
	}
	if includeUsage {
		payload["stream_options"] = map[string]bool{"include_usage": true}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return completionResult{stageFailed: "completion_http"}, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return completionResult{stageFailed: "completion_http"}, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if m.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.APIKey)
	}
	applyRequestHeaders(req, m.UserAgent, m.CustomHeaders)

	tStart := time.Now()
	resp, err := p.client.Do(req)
	if err != nil {
		return completionResult{stageFailed: "completion_http"}, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody := readErrorBody(resp.Body)
		return completionResult{
			httpStatus:  resp.StatusCode,
			stageFailed: "completion_http",
		}, errBody, fmt.Errorf("HTTP %d: %s", resp.StatusCode, errBody)
	}

	var (
		tFirst       time.Time
		usageTokens  int
		nonEmptyHits int
		gotChunk     bool
	)

	err = readSSEEvents(resp.Body, func(ev sseEvent) error {
		if ev.Data == "" {
			return nil
		}
		if ev.Data == "[DONE]" {
			return errStopStream
		}

		var chunk struct {
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(ev.Data), &chunk); err != nil {
			return nil
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return fmt.Errorf("stream error: %s", chunk.Error.Message)
		}
		gotChunk = true
		if len(chunk.Choices) > 0 {
			d := chunk.Choices[0].Delta
			if d.Content != "" || d.ReasoningContent != "" {
				if tFirst.IsZero() {
					tFirst = time.Now()
				}
				nonEmptyHits++
			}
		}
		if chunk.Usage != nil && chunk.Usage.CompletionTokens > 0 {
			usageTokens = chunk.Usage.CompletionTokens
		}
		return nil
	})
	tEnd := time.Now()

	if err != nil {
		return completionResult{
			httpStatus:  resp.StatusCode,
			stageFailed: "completion_stream",
		}, "", err
	}
	if !gotChunk && nonEmptyHits == 0 && usageTokens == 0 {
		return completionResult{
			httpStatus:  resp.StatusCode,
			stageFailed: "completion_stream",
		}, "", fmt.Errorf("empty SSE response from upstream")
	}

	res := completionResult{
		httpStatus:     resp.StatusCode,
		usedStream:     true,
		totalLatencyMs: tEnd.Sub(tStart).Milliseconds(),
	}
	if !tFirst.IsZero() {
		ttft := tFirst.Sub(tStart).Milliseconds()
		res.ttftMs = &ttft
	}

	outTokens := usageTokens
	method := "usage"
	if outTokens == 0 && nonEmptyHits > 0 {
		outTokens = nonEmptyHits
		method = "chunk_estimate"
	}
	if outTokens > 0 {
		res.outputTokens = &outTokens
		if tps, _ := computeTPS(outTokens, tStart, tFirst, tEnd, true); tps != nil {
			res.tokensPerSec = tps
			res.tpsMethod = method
		}
	}
	return res, "", nil
}

func (p *Prober) doOpenAINonStream(ctx context.Context, m domain.Monitor, useMaxCompletionTokens bool) (completionResult, error) {
	url := strings.TrimRight(strings.TrimSpace(m.BaseURL), "/") + "/chat/completions"
	payload := map[string]any{
		"model": m.ModelID,
		"messages": []map[string]string{
			{"role": "user", "content": m.Prompt},
		},
		"stream": false,
	}
	if useMaxCompletionTokens {
		payload["max_completion_tokens"] = m.MaxTokens
	} else {
		payload["max_tokens"] = m.MaxTokens
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return completionResult{stageFailed: "completion_http"}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return completionResult{stageFailed: "completion_http"}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if m.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.APIKey)
	}
	applyRequestHeaders(req, m.UserAgent, m.CustomHeaders)

	tStart := time.Now()
	resp, err := p.client.Do(req)
	if err != nil {
		return completionResult{stageFailed: "completion_http"}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody := readErrorBody(resp.Body)
		if !useMaxCompletionTokens && resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(errBody), "max_completion_tokens") {
			return p.doOpenAINonStream(ctx, m, true)
		}
		return completionResult{
			httpStatus:  resp.StatusCode,
			stageFailed: "completion_http",
		}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, errBody)
	}

	var parsed struct {
		Usage *struct {
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return completionResult{httpStatus: resp.StatusCode, stageFailed: "completion_http"}, err
	}
	tEnd := time.Now()

	res := completionResult{
		httpStatus:     resp.StatusCode,
		usedStream:     false,
		totalLatencyMs: tEnd.Sub(tStart).Milliseconds(),
	}
	if parsed.Usage != nil && parsed.Usage.CompletionTokens > 0 {
		tok := parsed.Usage.CompletionTokens
		res.outputTokens = &tok
		if tps, _ := computeTPS(tok, tStart, time.Time{}, tEnd, false); tps != nil {
			res.tokensPerSec = tps
			res.tpsMethod = "non_stream"
		}
	}
	return res, nil
}

func (p *Prober) probeOpenAIResponses(ctx context.Context, m domain.Monitor) (completionResult, error) {
	if m.StreamEnabled {
		res, bodyText, err := p.doOpenAIResponsesStream(ctx, m)
		if err != nil && m.FallbackNonStream && isStreamUnsupportedErr(res.httpStatus, bodyText) {
			return p.doOpenAIResponsesNonStream(ctx, m)
		}
		return res, err
	}
	return p.doOpenAIResponsesNonStream(ctx, m)
}

func (p *Prober) doOpenAIResponsesStream(ctx context.Context, m domain.Monitor) (completionResult, string, error) {
	url := strings.TrimRight(strings.TrimSpace(m.BaseURL), "/") + "/responses"
	payload := map[string]any{
		"model":             m.ModelID,
		"input":             m.Prompt,
		"max_output_tokens": m.MaxTokens,
		"stream":            true,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return completionResult{stageFailed: "completion_http"}, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return completionResult{stageFailed: "completion_http"}, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if m.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.APIKey)
	}
	applyRequestHeaders(req, m.UserAgent, m.CustomHeaders)

	tStart := time.Now()
	resp, err := p.client.Do(req)
	if err != nil {
		return completionResult{stageFailed: "completion_http"}, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody := readErrorBody(resp.Body)
		return completionResult{
			httpStatus:  resp.StatusCode,
			stageFailed: "completion_http",
		}, errBody, fmt.Errorf("HTTP %d: %s", resp.StatusCode, errBody)
	}

	var (
		tFirst       time.Time
		usageTokens  int
		nonEmptyHits int
		gotEvent     bool
	)

	err = readSSEEvents(resp.Body, func(ev sseEvent) error {
		if ev.Data == "" {
			return nil
		}
		if ev.Data == "[DONE]" {
			return errStopStream
		}

		var data struct {
			Type     string `json:"type"`
			Delta    string `json:"delta"`
			Error    *struct {
				Message string `json:"message"`
			} `json:"error"`
			Response *struct {
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
				Usage *struct {
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			} `json:"response"`
		}
		if err := json.Unmarshal([]byte(ev.Data), &data); err != nil {
			return nil
		}
		evType := data.Type
		if evType == "" {
			evType = ev.Event
		}
		if evType == "error" && data.Error != nil && data.Error.Message != "" {
			return fmt.Errorf("responses stream error: %s", data.Error.Message)
		}
		if evType == "response.failed" && data.Response != nil && data.Response.Error != nil {
			return fmt.Errorf("responses failed: %s", data.Response.Error.Message)
		}

		gotEvent = true
		if (evType == "response.output_text.delta" ||
			evType == "response.reasoning_text.delta" ||
			evType == "response.reasoning_summary_text.delta") && data.Delta != "" {
			if tFirst.IsZero() {
				tFirst = time.Now()
			}
			nonEmptyHits++
		}
		if evType == "response.completed" {
			if data.Response != nil && data.Response.Usage != nil && data.Response.Usage.OutputTokens > 0 {
				usageTokens = data.Response.Usage.OutputTokens
			}
			return errStopStream
		}
		return nil
	})
	tEnd := time.Now()

	if err != nil {
		return completionResult{
			httpStatus:  resp.StatusCode,
			stageFailed: "completion_stream",
		}, "", err
	}
	if !gotEvent && nonEmptyHits == 0 && usageTokens == 0 {
		return completionResult{
			httpStatus:  resp.StatusCode,
			stageFailed: "completion_stream",
		}, "", fmt.Errorf("empty SSE response from /responses")
	}

	res := completionResult{
		httpStatus:     resp.StatusCode,
		usedStream:     true,
		totalLatencyMs: tEnd.Sub(tStart).Milliseconds(),
	}
	if !tFirst.IsZero() {
		ttft := tFirst.Sub(tStart).Milliseconds()
		res.ttftMs = &ttft
	}

	outTokens := usageTokens
	method := "usage"
	if outTokens == 0 && nonEmptyHits > 0 {
		outTokens = nonEmptyHits
		method = "chunk_estimate"
	}
	if outTokens > 0 {
		res.outputTokens = &outTokens
		if tps, _ := computeTPS(outTokens, tStart, tFirst, tEnd, true); tps != nil {
			res.tokensPerSec = tps
			res.tpsMethod = method
		}
	}
	return res, "", nil
}

func (p *Prober) doOpenAIResponsesNonStream(ctx context.Context, m domain.Monitor) (completionResult, error) {
	url := strings.TrimRight(strings.TrimSpace(m.BaseURL), "/") + "/responses"
	payload := map[string]any{
		"model":             m.ModelID,
		"input":             m.Prompt,
		"max_output_tokens": m.MaxTokens,
		"stream":            false,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return completionResult{stageFailed: "completion_http"}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return completionResult{stageFailed: "completion_http"}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if m.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.APIKey)
	}
	applyRequestHeaders(req, m.UserAgent, m.CustomHeaders)

	tStart := time.Now()
	resp, err := p.client.Do(req)
	if err != nil {
		return completionResult{stageFailed: "completion_http"}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody := readErrorBody(resp.Body)
		return completionResult{
			httpStatus:  resp.StatusCode,
			stageFailed: "completion_http",
		}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, errBody)
	}

	var parsed struct {
		Usage *struct {
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return completionResult{httpStatus: resp.StatusCode, stageFailed: "completion_http"}, err
	}
	tEnd := time.Now()

	res := completionResult{
		httpStatus:     resp.StatusCode,
		usedStream:     false,
		totalLatencyMs: tEnd.Sub(tStart).Milliseconds(),
	}
	if parsed.Usage != nil && parsed.Usage.OutputTokens > 0 {
		tok := parsed.Usage.OutputTokens
		res.outputTokens = &tok
		if tps, _ := computeTPS(tok, tStart, time.Time{}, tEnd, false); tps != nil {
			res.tokensPerSec = tps
			res.tpsMethod = "non_stream"
		}
	}
	return res, nil
}
