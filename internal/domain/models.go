package domain

type ProviderType string

const (
	ProviderOpenAI    ProviderType = "openai"
	ProviderAnthropic ProviderType = "anthropic"
)

type Status string

const (
	StatusUp       Status = "up"
	StatusDegraded Status = "degraded"
	StatusDown     Status = "down"
)

// Monitor represents an LLM endpoint monitor.
// APIKey and CustomHeaders are only populated in memory for the prober/scheduler, never serialized to JSON.
type Monitor struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	GroupName         string            `json:"group_name"`
	ProviderType      ProviderType      `json:"provider_type"`
	OpenAIEndpoint    string            `json:"openai_endpoint"`
	BaseURL           string            `json:"base_url"`
	HasAPIKey         bool              `json:"has_api_key"`
	APIKeyHint        string            `json:"api_key_hint"`
	ModelID           string            `json:"model_id"`
	CheckModelsFirst  bool              `json:"check_models_first"`
	StreamEnabled     bool              `json:"stream_enabled"`
	FallbackNonStream bool              `json:"fallback_non_stream"`
	Prompt            string            `json:"prompt"`
	MaxTokens         int               `json:"max_tokens"`
	IntervalSec       int               `json:"interval_sec"`
	TimeoutSec        int               `json:"timeout_sec"`
	AnthropicVersion  string            `json:"anthropic_version"`
	UserAgent         string            `json:"user_agent"`
	HasCustomHeaders  bool              `json:"has_custom_headers"`
	IsActive          bool              `json:"is_active"`
	IsPublic          bool              `json:"is_public"`
	CreatedAt         string            `json:"created_at"`
	UpdatedAt         string            `json:"updated_at"`
	APIKey            string            `json:"-"`
	CustomHeaders     map[string]string `json:"-"`
}

type MonitorInput struct {
	Name              string            `json:"name"`
	GroupName         string            `json:"group_name"`
	ProviderType      ProviderType      `json:"provider_type"`
	OpenAIEndpoint    string            `json:"openai_endpoint"`
	BaseURL           string            `json:"base_url"`
	APIKey            *string           `json:"api_key,omitempty"`
	ModelID           string            `json:"model_id"`
	CheckModelsFirst  *bool             `json:"check_models_first,omitempty"`
	StreamEnabled     *bool             `json:"stream_enabled,omitempty"`
	FallbackNonStream *bool             `json:"fallback_non_stream,omitempty"`
	Prompt            string            `json:"prompt"`
	MaxTokens         int               `json:"max_tokens"`
	IntervalSec       int               `json:"interval_sec"`
	TimeoutSec        int               `json:"timeout_sec"`
	AnthropicVersion  string            `json:"anthropic_version"`
	UserAgent         string            `json:"user_agent"`
	CustomHeaders     map[string]string `json:"custom_headers,omitempty"`
	ClearHeaders      bool              `json:"clear_headers,omitempty"`
	IsActive          *bool             `json:"is_active,omitempty"`
	IsPublic          *bool             `json:"is_public,omitempty"`
}

type Heartbeat struct {
	ID              int64    `json:"id"`
	MonitorID       string   `json:"monitor_id"`
	Status          Status   `json:"status"`
	StageFailed     string   `json:"stage_failed"`
	HTTPStatus      int      `json:"http_status"`
	ModelsOK        *bool    `json:"models_ok"`
	ModelsLatencyMs *int64   `json:"models_latency_ms"`
	CompletionOK    bool     `json:"completion_ok"`
	UsedStream      bool     `json:"used_stream"`
	TTFTMs          *int64   `json:"ttft_ms"`
	TotalLatencyMs  *int64   `json:"total_latency_ms"`
	OutputTokens    *int     `json:"output_tokens"`
	TokensPerSec    *float64 `json:"tokens_per_sec"`
	TPSMethod       string   `json:"tps_method"`
	ErrorMessage    string   `json:"error_message"`
	CheckedAt       string   `json:"checked_at"`
}

type HourlyStat struct {
	MonitorID      string   `json:"monitor_id"`
	BucketHour     string   `json:"bucket_hour"`
	TotalChecks    int      `json:"total_checks"`
	UpChecks       int      `json:"up_checks"`
	DegradedChecks int      `json:"degraded_checks"`
	DownChecks     int      `json:"down_checks"`
	AvgTTFTMs      *float64 `json:"avg_ttft_ms"`
	AvgTotalMs     *float64 `json:"avg_total_ms"`
	AvgTPS         *float64 `json:"avg_tps"`
	UptimePct      float64  `json:"uptime_pct"`
}

type MonitorOverview struct {
	Monitor          Monitor     `json:"monitor"`
	Latest           *Heartbeat  `json:"latest"`
	RecentHeartbeats []Heartbeat `json:"recent_heartbeats"`
	Uptime24h        float64     `json:"uptime_24h"`
	AvgTPS24h        *float64    `json:"avg_tps_24h"`
	AvgTTFT24h       *float64    `json:"avg_ttft_24h"`
}

type AppSettings struct {
	RetentionDays         int    `json:"retention_days"`
	PublicPageTitle       string `json:"public_page_title"`
	PublicPageDescription string `json:"public_page_description"`
}

type PublicHeartbeat struct {
	Status          Status   `json:"status"`
	ModelsLatencyMs *int64   `json:"models_latency_ms"`
	TTFTMs          *int64   `json:"ttft_ms"`
	TotalLatencyMs  *int64   `json:"total_latency_ms"`
	TokensPerSec    *float64 `json:"tokens_per_sec"`
	CheckedAt       string   `json:"checked_at"`
}

type PublicMonitor struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	GroupName        string            `json:"group_name"`
	ProviderType     ProviderType      `json:"provider_type"`
	ModelID          string            `json:"model_id"`
	IntervalSec      int               `json:"interval_sec"`
	IsActive         bool              `json:"is_active"`
	Latest           *PublicHeartbeat  `json:"latest"`
	RecentHeartbeats []PublicHeartbeat `json:"recent_heartbeats"`
	Uptime24h        float64           `json:"uptime_24h"`
	Uptime30d        float64           `json:"uptime_30d"`
	AvgTPS24h        *float64          `json:"avg_tps_24h"`
	AvgTTFT24h       *float64          `json:"avg_ttft_24h"`
}

type PublicStatusResponse struct {
	Title         string          `json:"title"`
	Description   string          `json:"description"`
	OverallStatus Status          `json:"overall_status"`
	Monitors      []PublicMonitor `json:"monitors"`
	GeneratedAt   string          `json:"generated_at"`
}

type PublicHeartbeatEvent struct {
	MonitorID string          `json:"monitor_id"`
	Heartbeat PublicHeartbeat `json:"heartbeat"`
}
