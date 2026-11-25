package prober

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"llm-api-uptime/internal/domain"
)

type Prober struct {
	client *http.Client
}

func New() *Prober {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		DisableCompression:    true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
	}
	return &Prober{
		client: &http.Client{Transport: transport},
	}
}

type ModelInfo struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by,omitempty"`
}

func (p *Prober) DiscoverModels(
	ctx context.Context,
	provider domain.ProviderType,
	baseURL string,
	apiKey string,
	anthropicVersion string,
	userAgent string,
	customHeaders map[string]string,
) ([]ModelInfo, int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	switch provider {
	case domain.ProviderAnthropic:
		return p.discoverAnthropicModels(ctx, baseURL, apiKey, anthropicVersion, userAgent, customHeaders)
	default:
		return p.discoverOpenAIModels(ctx, baseURL, apiKey, userAgent, customHeaders)
	}
}

func (p *Prober) Probe(ctx context.Context, m domain.Monitor) domain.Heartbeat {
	timeout := time.Duration(m.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	hb := domain.Heartbeat{
		MonitorID: m.ID,
		Status:    domain.StatusDown,
		CheckedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}

	// Stage 1: /models pre-flight
	if m.CheckModelsFirst {
		var (
			models    []ModelInfo
			latencyMs int64
			httpCode  int
			err       error
		)
		if m.ProviderType == domain.ProviderAnthropic {
			models, latencyMs, httpCode, err = p.fetchAnthropicModelsWithStatus(ctx, m.BaseURL, m.APIKey, m.AnthropicVersion, m.UserAgent, m.CustomHeaders)
		} else {
			models, latencyMs, httpCode, err = p.fetchOpenAIModelsWithStatus(ctx, m.BaseURL, m.APIKey, m.UserAgent, m.CustomHeaders)
		}
		hb.ModelsLatencyMs = &latencyMs
		if httpCode > 0 {
			hb.HTTPStatus = httpCode
		}
		if err != nil {
			f := false
			hb.ModelsOK = &f
			hb.StageFailed = "models_http"
			hb.ErrorMessage = sanitizeError(err.Error(), m.APIKey)
			return hb
		}

		if !containsModel(models, m.ModelID) {
			f := false
			hb.ModelsOK = &f
			hb.StageFailed = "model_missing"
			hb.ErrorMessage = sanitizeError(fmt.Sprintf("model %q not found in /models (%d models returned)", m.ModelID, len(models)), m.APIKey)
			return hb
		}

		t := true
		hb.ModelsOK = &t
	}

	// Stage 2: Completion check
	var res completionResult
	var err error
	if m.ProviderType == domain.ProviderAnthropic {
		res, err = p.probeAnthropicCompletion(ctx, m)
	} else {
		res, err = p.probeOpenAICompletion(ctx, m)
	}

	if res.httpStatus > 0 {
		hb.HTTPStatus = res.httpStatus
	}
	if err != nil {
		if m.CheckModelsFirst && hb.ModelsOK != nil && *hb.ModelsOK {
			hb.Status = domain.StatusDegraded
		} else {
			hb.Status = domain.StatusDown
		}
		hb.StageFailed = res.stageFailed
		if hb.StageFailed == "" {
			hb.StageFailed = "completion_http"
		}
		hb.ErrorMessage = sanitizeError(err.Error(), m.APIKey)
		return hb
	}

	hb.Status = domain.StatusUp
	hb.CompletionOK = true
	hb.UsedStream = res.usedStream
	hb.TTFTMs = res.ttftMs
	hb.TotalLatencyMs = &res.totalLatencyMs
	hb.OutputTokens = res.outputTokens
	hb.TokensPerSec = res.tokensPerSec
	hb.TPSMethod = res.tpsMethod
	return hb
}

type completionResult struct {
	httpStatus     int
	stageFailed    string
	usedStream     bool
	ttftMs         *int64
	totalLatencyMs int64
	outputTokens   *int
	tokensPerSec   *float64
	tpsMethod      string
}

func containsModel(models []ModelInfo, target string) bool {
	t := strings.TrimSpace(target)
	for _, m := range models {
		if m.ID == t {
			return true
		}
	}
	return false
}

func computeTPS(outputTokens int, tStart, tFirst, tEnd time.Time, streamed bool) (*float64, string) {
	if outputTokens <= 0 {
		return nil, ""
	}
	if streamed && !tFirst.IsZero() && outputTokens > 1 {
		genSec := tEnd.Sub(tFirst).Seconds()
		if genSec > 0.005 {
			v := float64(outputTokens-1) / genSec
			return &v, ""
		}
	}
	totalSec := tEnd.Sub(tStart).Seconds()
	if totalSec > 0.005 {
		v := float64(outputTokens) / totalSec
		return &v, ""
	}
	return nil, ""
}

func applyRequestHeaders(req *http.Request, userAgent string, headers map[string]string) {
	if ua := strings.TrimSpace(userAgent); ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	for k, v := range headers {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		req.Header.Set(key, v)
	}
}

func sanitizeError(msg, secret string) string {
	if secret != "" {
		msg = strings.ReplaceAll(msg, secret, "[REDACTED]")
	}
	msg = strings.TrimSpace(msg)
	if len(msg) > 500 {
		return msg[:497] + "..."
	}
	return msg
}

func readErrorBody(body io.Reader) string {
	b, _ := io.ReadAll(io.LimitReader(body, 1024))
	return strings.TrimSpace(string(b))
}

func isStreamUnsupportedErr(httpStatus int, body string) bool {
	if httpStatus != http.StatusBadRequest && httpStatus != http.StatusUnprocessableEntity {
		return false
	}
	low := strings.ToLower(body)
	return strings.Contains(low, "stream") && (strings.Contains(low, "unsupported") || strings.Contains(low, "not supported") || strings.Contains(low, "invalid"))
}

type sseEvent struct {
	Event string
	Data  string
}

func readSSEEvents(r io.Reader, handle func(sseEvent) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var evName string
	var dataLines []string

	flush := func() error {
		if len(dataLines) == 0 && evName == "" {
			return nil
		}
		ev := sseEvent{
			Event: evName,
			Data:  strings.Join(dataLines, "\n"),
		}
		evName = ""
		dataLines = dataLines[:0]
		return handle(ev)
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				if errors.Is(err, errStopStream) {
					return nil
				}
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			evName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if err := flush(); err != nil && !errors.Is(err, errStopStream) {
		return err
	}
	return nil
}

var errStopStream = errors.New("stop sse stream")
