package prober

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"llm-api-uptime/internal/domain"
)

func anthropicBase(baseURL string) string {
	b := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(b, "/v1") {
		return b
	}
	return b + "/v1"
}

func (p *Prober) discoverAnthropicModels(ctx context.Context, baseURL, apiKey, version, userAgent string, headers map[string]string) ([]ModelInfo, int64, error) {
	models, lat, _, err := p.fetchAnthropicModelsWithStatus(ctx, baseURL, apiKey, version, userAgent, headers)
	return models, lat, err
}

func (p *Prober) fetchAnthropicModelsWithStatus(ctx context.Context, baseURL, apiKey, version, userAgent string, headers map[string]string) ([]ModelInfo, int64, int, error) {
	if version == "" {
		version = "2023-06-01"
	}
	base := anthropicBase(baseURL)
	var all []ModelInfo
	var totalLat int64
	var lastCode int
	afterID := ""

	for page := 0; page < 5; page++ {
		u := base + "/models?limit=100"
		if afterID != "" {
			u += "&after_id=" + url.QueryEscape(afterID)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, totalLat, lastCode, err
		}
		if apiKey != "" {
			req.Header.Set("x-api-key", apiKey)
		}
		req.Header.Set("anthropic-version", version)
		req.Header.Set("Accept", "application/json")
		applyRequestHeaders(req, userAgent, headers)

		start := time.Now()
		resp, err := p.client.Do(req)
		totalLat += time.Since(start).Milliseconds()
		if err != nil {
			return nil, totalLat, lastCode, err
		}

		lastCode = resp.StatusCode
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errBody := readErrorBody(resp.Body)
			_ = resp.Body.Close()
			return nil, totalLat, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, errBody)
		}

		var body struct {
			Data    []ModelInfo `json:"data"`
			HasMore bool        `json:"has_more"`
			LastID  string      `json:"last_id"`
		}
		err = json.NewDecoder(resp.Body).Decode(&body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, totalLat, lastCode, fmt.Errorf("decode /v1/models: %w", err)
		}
		all = append(all, body.Data...)
		if !body.HasMore || body.LastID == "" {
			break
		}
		afterID = body.LastID
	}

	return all, totalLat, lastCode, nil
}

func (p *Prober) probeAnthropicCompletion(ctx context.Context, m domain.Monitor) (completionResult, error) {
	if m.StreamEnabled {
		res, bodyText, err := p.doAnthropicStream(ctx, m)
		if err != nil && m.FallbackNonStream && isStreamUnsupportedErr(res.httpStatus, bodyText) {
			return p.doAnthropicNonStream(ctx, m)
		}
		return res, err
	}
	return p.doAnthropicNonStream(ctx, m)
}

func (p *Prober) doAnthropicStream(ctx context.Context, m domain.Monitor) (completionResult, string, error) {
	version := m.AnthropicVersion
	if version == "" {
		version = "2023-06-01"
	}
	endpoint := anthropicBase(m.BaseURL) + "/messages"
	payload := map[string]any{
		"model":      m.ModelID,
		"max_tokens": m.MaxTokens,
		"stream":     true,
		"messages": []map[string]string{
			{"role": "user", "content": m.Prompt},
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return completionResult{stageFailed: "completion_http"}, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return completionResult{stageFailed: "completion_http"}, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("anthropic-version", version)
	if m.APIKey != "" {
		req.Header.Set("x-api-key", m.APIKey)
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
		gotMessage   bool
	)

	err = readSSEEvents(resp.Body, func(ev sseEvent) error {
		if ev.Data == "" {
			return nil
		}
		var data struct {
			Type  string `json:"type"`
			Error *struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
			Delta *struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				Thinking string `json:"thinking"`
			} `json:"delta"`
			Usage *struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(ev.Data), &data); err != nil {
			return nil
		}
		if data.Type == "error" && data.Error != nil {
			return fmt.Errorf("anthropic stream error (%s): %s", data.Error.Type, data.Error.Message)
		}
		if data.Type == "message_start" {
			gotMessage = true
		}
		if data.Type == "content_block_delta" && data.Delta != nil {
			if data.Delta.Text != "" || data.Delta.Thinking != "" {
				if tFirst.IsZero() {
					tFirst = time.Now()
				}
				nonEmptyHits++
			}
		}
		if data.Type == "message_delta" && data.Usage != nil && data.Usage.OutputTokens > 0 {
			usageTokens = data.Usage.OutputTokens
		}
		if data.Type == "message_stop" {
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
	if !gotMessage && nonEmptyHits == 0 && usageTokens == 0 {
		return completionResult{
			httpStatus:  resp.StatusCode,
			stageFailed: "completion_stream",
		}, "", fmt.Errorf("empty SSE response from anthropic endpoint")
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

func (p *Prober) doAnthropicNonStream(ctx context.Context, m domain.Monitor) (completionResult, error) {
	version := m.AnthropicVersion
	if version == "" {
		version = "2023-06-01"
	}
	endpoint := anthropicBase(m.BaseURL) + "/messages"
	payload := map[string]any{
		"model":      m.ModelID,
		"max_tokens": m.MaxTokens,
		"stream":     false,
		"messages": []map[string]string{
			{"role": "user", "content": m.Prompt},
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return completionResult{stageFailed: "completion_http"}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return completionResult{stageFailed: "completion_http"}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", version)
	if m.APIKey != "" {
		req.Header.Set("x-api-key", m.APIKey)
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
