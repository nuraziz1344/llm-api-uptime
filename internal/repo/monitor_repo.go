package repo

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"llm-api-uptime/internal/crypto"
	"llm-api-uptime/internal/domain"
)

type MonitorRepo struct {
	writer *sql.DB
	reader *sql.DB
	vault  *crypto.Vault
}

func NewMonitorRepo(writer, reader *sql.DB, vault *crypto.Vault) *MonitorRepo {
	return &MonitorRepo{writer: writer, reader: reader, vault: vault}
}

func newID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func normalizeOpenAIEndpoint(ep string) string {
	if strings.TrimSpace(ep) == "responses" {
		return "responses"
	}
	return "chat_completions"
}

func (r *MonitorRepo) Create(ctx context.Context, in domain.MonitorInput) (domain.Monitor, error) {
	id, err := newID()
	if err != nil {
		return domain.Monitor{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	m := domain.Monitor{
		ID:                id,
		Name:              strings.TrimSpace(in.Name),
		GroupName:         strings.TrimSpace(in.GroupName),
		ProviderType:      in.ProviderType,
		OpenAIEndpoint:    normalizeOpenAIEndpoint(in.OpenAIEndpoint),
		BaseURL:           strings.TrimRight(strings.TrimSpace(in.BaseURL), "/"),
		ModelID:           strings.TrimSpace(in.ModelID),
		CheckModelsFirst:  true,
		StreamEnabled:     true,
		FallbackNonStream: true,
		Prompt:            strings.TrimSpace(in.Prompt),
		MaxTokens:         in.MaxTokens,
		IntervalSec:       in.IntervalSec,
		TimeoutSec:        in.TimeoutSec,
		AnthropicVersion:  strings.TrimSpace(in.AnthropicVersion),
		UserAgent:         strings.TrimSpace(in.UserAgent),
		IsActive:          true,
		IsPublic:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
		CustomHeaders:     in.CustomHeaders,
	}

	if in.CheckModelsFirst != nil {
		m.CheckModelsFirst = *in.CheckModelsFirst
	}
	if in.StreamEnabled != nil {
		m.StreamEnabled = *in.StreamEnabled
	}
	if in.FallbackNonStream != nil {
		m.FallbackNonStream = *in.FallbackNonStream
	}
	if in.IsActive != nil {
		m.IsActive = *in.IsActive
	}
	if in.IsPublic != nil {
		m.IsPublic = *in.IsPublic
	}
	if m.Prompt == "" {
		m.Prompt = "Count from 1 to 15 separated by spaces."
	}
	if m.MaxTokens <= 0 {
		m.MaxTokens = 24
	}
	if m.IntervalSec < 10 {
		m.IntervalSec = 60
	}
	if m.TimeoutSec < 3 {
		m.TimeoutSec = 30
	}
	if m.AnthropicVersion == "" {
		m.AnthropicVersion = "2023-06-01"
	}

	var apiKeyEnc string
	if in.APIKey != nil && strings.TrimSpace(*in.APIKey) != "" {
		m.APIKey = strings.TrimSpace(*in.APIKey)
		m.HasAPIKey = true
		m.APIKeyHint = crypto.MaskKey(m.APIKey)
		apiKeyEnc, err = r.vault.Encrypt(m.APIKey)
		if err != nil {
			return domain.Monitor{}, fmt.Errorf("encrypt api key: %w", err)
		}
	}

	var headersEnc string
	if len(in.CustomHeaders) > 0 {
		raw, err := json.Marshal(in.CustomHeaders)
		if err != nil {
			return domain.Monitor{}, err
		}
		headersEnc, err = r.vault.Encrypt(string(raw))
		if err != nil {
			return domain.Monitor{}, fmt.Errorf("encrypt headers: %w", err)
		}
		m.HasCustomHeaders = true
	}

	_, err = r.writer.ExecContext(ctx, `
		INSERT INTO monitors (
			id, name, group_name, provider_type, openai_endpoint, base_url, api_key_enc, api_key_hint,
			model_id, check_models_first, stream_enabled, fallback_non_stream,
			prompt, max_tokens, interval_sec, timeout_sec, anthropic_version, user_agent,
			custom_headers_enc, is_active, is_public, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		m.ID, m.Name, m.GroupName, string(m.ProviderType), m.OpenAIEndpoint, m.BaseURL, apiKeyEnc, m.APIKeyHint,
		m.ModelID, boolToInt(m.CheckModelsFirst), boolToInt(m.StreamEnabled), boolToInt(m.FallbackNonStream),
		m.Prompt, m.MaxTokens, m.IntervalSec, m.TimeoutSec, m.AnthropicVersion, m.UserAgent,
		headersEnc, boolToInt(m.IsActive), boolToInt(m.IsPublic), m.CreatedAt, m.UpdatedAt,
	)
	if err != nil {
		return domain.Monitor{}, err
	}
	return m, nil
}

func (r *MonitorRepo) GetByID(ctx context.Context, id string) (domain.Monitor, error) {
	row := r.reader.QueryRowContext(ctx, `
		SELECT id, name, group_name, provider_type, openai_endpoint, base_url, api_key_enc, api_key_hint,
			model_id, check_models_first, stream_enabled, fallback_non_stream,
			prompt, max_tokens, interval_sec, timeout_sec, anthropic_version, user_agent,
			custom_headers_enc, is_active, is_public, created_at, updated_at
		FROM monitors WHERE id = ?
	`, id)
	return r.scanMonitor(row)
}

func (r *MonitorRepo) ListAll(ctx context.Context) ([]domain.Monitor, error) {
	rows, err := r.reader.QueryContext(ctx, `
		SELECT id, name, group_name, provider_type, openai_endpoint, base_url, api_key_enc, api_key_hint,
			model_id, check_models_first, stream_enabled, fallback_non_stream,
			prompt, max_tokens, interval_sec, timeout_sec, anthropic_version, user_agent,
			custom_headers_enc, is_active, is_public, created_at, updated_at
		FROM monitors ORDER BY group_name ASC, name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]domain.Monitor, 0)
	for rows.Next() {
		m, err := r.scanMonitor(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *MonitorRepo) ListPublic(ctx context.Context) ([]domain.Monitor, error) {
	rows, err := r.reader.QueryContext(ctx, `
		SELECT id, name, group_name, provider_type, openai_endpoint, base_url, api_key_enc, api_key_hint,
			model_id, check_models_first, stream_enabled, fallback_non_stream,
			prompt, max_tokens, interval_sec, timeout_sec, anthropic_version, user_agent,
			custom_headers_enc, is_active, is_public, created_at, updated_at
		FROM monitors WHERE is_public = 1 ORDER BY group_name ASC, name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]domain.Monitor, 0)
	for rows.Next() {
		m, err := r.scanMonitor(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *MonitorRepo) Update(ctx context.Context, id string, in domain.MonitorInput) (domain.Monitor, error) {
	existing, err := r.GetByID(ctx, id)
	if err != nil {
		return domain.Monitor{}, err
	}

	var apiKeyEnc string
	err = r.reader.QueryRowContext(ctx, `SELECT api_key_enc FROM monitors WHERE id = ?`, id).Scan(&apiKeyEnc)
	if err != nil {
		return domain.Monitor{}, err
	}
	var headersEnc string
	err = r.reader.QueryRowContext(ctx, `SELECT custom_headers_enc FROM monitors WHERE id = ?`, id).Scan(&headersEnc)
	if err != nil {
		return domain.Monitor{}, err
	}

	existing.Name = strings.TrimSpace(in.Name)
	existing.GroupName = strings.TrimSpace(in.GroupName)
	existing.ProviderType = in.ProviderType
	existing.OpenAIEndpoint = normalizeOpenAIEndpoint(in.OpenAIEndpoint)
	existing.BaseURL = strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	existing.ModelID = strings.TrimSpace(in.ModelID)
	if in.CheckModelsFirst != nil {
		existing.CheckModelsFirst = *in.CheckModelsFirst
	}
	if in.StreamEnabled != nil {
		existing.StreamEnabled = *in.StreamEnabled
	}
	if in.FallbackNonStream != nil {
		existing.FallbackNonStream = *in.FallbackNonStream
	}
	if strings.TrimSpace(in.Prompt) != "" {
		existing.Prompt = strings.TrimSpace(in.Prompt)
	}
	if in.MaxTokens > 0 {
		existing.MaxTokens = in.MaxTokens
	}
	if in.IntervalSec >= 10 {
		existing.IntervalSec = in.IntervalSec
	}
	if in.TimeoutSec >= 3 {
		existing.TimeoutSec = in.TimeoutSec
	}
	if strings.TrimSpace(in.AnthropicVersion) != "" {
		existing.AnthropicVersion = strings.TrimSpace(in.AnthropicVersion)
	}
	existing.UserAgent = strings.TrimSpace(in.UserAgent)
	if in.IsActive != nil {
		existing.IsActive = *in.IsActive
	}
	if in.IsPublic != nil {
		existing.IsPublic = *in.IsPublic
	}

	if in.APIKey != nil {
		trimmed := strings.TrimSpace(*in.APIKey)
		if trimmed == "" {
			apiKeyEnc = ""
			existing.APIKey = ""
			existing.APIKeyHint = ""
			existing.HasAPIKey = false
		} else {
			apiKeyEnc, err = r.vault.Encrypt(trimmed)
			if err != nil {
				return domain.Monitor{}, err
			}
			existing.APIKey = trimmed
			existing.APIKeyHint = crypto.MaskKey(trimmed)
			existing.HasAPIKey = true
		}
	}

	if in.ClearHeaders {
		headersEnc = ""
		existing.CustomHeaders = nil
		existing.HasCustomHeaders = false
	} else if in.CustomHeaders != nil {
		if len(in.CustomHeaders) == 0 {
			headersEnc = ""
			existing.CustomHeaders = nil
			existing.HasCustomHeaders = false
		} else {
			raw, err := json.Marshal(in.CustomHeaders)
			if err != nil {
				return domain.Monitor{}, err
			}
			headersEnc, err = r.vault.Encrypt(string(raw))
			if err != nil {
				return domain.Monitor{}, err
			}
			existing.CustomHeaders = in.CustomHeaders
			existing.HasCustomHeaders = true
		}
	}

	existing.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_, err = r.writer.ExecContext(ctx, `
		UPDATE monitors SET
			name = ?, group_name = ?, provider_type = ?, openai_endpoint = ?, base_url = ?,
			api_key_enc = ?, api_key_hint = ?, model_id = ?,
			check_models_first = ?, stream_enabled = ?, fallback_non_stream = ?,
			prompt = ?, max_tokens = ?, interval_sec = ?, timeout_sec = ?,
			anthropic_version = ?, user_agent = ?, custom_headers_enc = ?, is_active = ?, is_public = ?,
			updated_at = ?
		WHERE id = ?
	`,
		existing.Name, existing.GroupName, string(existing.ProviderType), existing.OpenAIEndpoint, existing.BaseURL,
		apiKeyEnc, existing.APIKeyHint, existing.ModelID,
		boolToInt(existing.CheckModelsFirst), boolToInt(existing.StreamEnabled), boolToInt(existing.FallbackNonStream),
		existing.Prompt, existing.MaxTokens, existing.IntervalSec, existing.TimeoutSec,
		existing.AnthropicVersion, existing.UserAgent, headersEnc, boolToInt(existing.IsActive), boolToInt(existing.IsPublic),
		existing.UpdatedAt, id,
	)
	if err != nil {
		return domain.Monitor{}, err
	}
	return existing, nil
}

func (r *MonitorRepo) SetActive(ctx context.Context, id string, active bool) (domain.Monitor, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := r.writer.ExecContext(ctx, `UPDATE monitors SET is_active = ?, updated_at = ? WHERE id = ?`, boolToInt(active), now, id)
	if err != nil {
		return domain.Monitor{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.Monitor{}, sql.ErrNoRows
	}
	return r.GetByID(ctx, id)
}

func (r *MonitorRepo) Delete(ctx context.Context, id string) error {
	res, err := r.writer.ExecContext(ctx, `DELETE FROM monitors WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func (r *MonitorRepo) scanMonitor(s scanner) (domain.Monitor, error) {
	var m domain.Monitor
	var providerStr, apiKeyEnc, headersEnc string
	var checkModels, streamEnabled, fallbackNonStream, isActive, isPublic int

	err := s.Scan(
		&m.ID, &m.Name, &m.GroupName, &providerStr, &m.OpenAIEndpoint, &m.BaseURL, &apiKeyEnc, &m.APIKeyHint,
		&m.ModelID, &checkModels, &streamEnabled, &fallbackNonStream,
		&m.Prompt, &m.MaxTokens, &m.IntervalSec, &m.TimeoutSec, &m.AnthropicVersion, &m.UserAgent,
		&headersEnc, &isActive, &isPublic, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return domain.Monitor{}, err
	}

	m.ProviderType = domain.ProviderType(providerStr)
	m.OpenAIEndpoint = normalizeOpenAIEndpoint(m.OpenAIEndpoint)
	m.CheckModelsFirst = checkModels == 1
	m.StreamEnabled = streamEnabled == 1
	m.FallbackNonStream = fallbackNonStream == 1
	m.IsActive = isActive == 1
	m.IsPublic = isPublic == 1

	if apiKeyEnc != "" {
		dec, err := r.vault.Decrypt(apiKeyEnc)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return domain.Monitor{}, fmt.Errorf("decrypt api_key for %s: %w", m.ID, err)
		}
		m.APIKey = dec
		m.HasAPIKey = dec != ""
	}
	if headersEnc != "" {
		dec, err := r.vault.Decrypt(headersEnc)
		if err != nil {
			return domain.Monitor{}, fmt.Errorf("decrypt headers for %s: %w", m.ID, err)
		}
		if dec != "" {
			var h map[string]string
			if err := json.Unmarshal([]byte(dec), &h); err == nil {
				m.CustomHeaders = h
				m.HasCustomHeaders = len(h) > 0
			}
		}
	}
	return m, nil
}
