export type ProviderType = 'openai' | 'anthropic';
export type OpenAIEndpoint = 'chat_completions' | 'responses';
export type Status = 'up' | 'degraded' | 'down';

export interface AuthStatus {
  setup_completed: boolean;
  authenticated: boolean;
  username: string;
}

export interface Monitor {
  id: string;
  name: string;
  group_name: string;
  provider_type: ProviderType;
  openai_endpoint: OpenAIEndpoint;
  base_url: string;
  has_api_key: boolean;
  api_key_hint: string;
  model_id: string;
  check_models_first: boolean;
  stream_enabled: boolean;
  fallback_non_stream: boolean;
  prompt: string;
  max_tokens: number;
  interval_sec: number;
  timeout_sec: number;
  anthropic_version: string;
  user_agent: string;
  has_custom_headers: boolean;
  is_active: boolean;
  is_public: boolean;
  created_at: string;
  updated_at: string;
}

export interface MonitorInput {
  name: string;
  group_name: string;
  provider_type: ProviderType;
  openai_endpoint: OpenAIEndpoint;
  base_url: string;
  api_key?: string;
  model_id: string;
  check_models_first: boolean;
  stream_enabled: boolean;
  fallback_non_stream: boolean;
  prompt: string;
  max_tokens: number;
  interval_sec: number;
  timeout_sec: number;
  anthropic_version: string;
  user_agent: string;
  is_active: boolean;
  is_public: boolean;
}

export interface Heartbeat {
  id: number;
  monitor_id: string;
  status: Status;
  stage_failed: string;
  http_status: number;
  models_ok: boolean | null;
  models_latency_ms: number | null;
  completion_ok: boolean;
  used_stream: boolean;
  ttft_ms: number | null;
  total_latency_ms: number | null;
  output_tokens: number | null;
  tokens_per_sec: number | null;
  tps_method: string;
  error_message: string;
  checked_at: string;
}

export interface MonitorOverview {
  monitor: Monitor;
  latest: Heartbeat | null;
  recent_heartbeats: Heartbeat[];
  uptime_24h: number;
  avg_tps_24h: number | null;
  avg_ttft_24h: number | null;
}

export interface HourlyStat {
  monitor_id: string;
  bucket_hour: string;
  total_checks: number;
  up_checks: number;
  degraded_checks: number;
  down_checks: number;
  avg_ttft_ms: number | null;
  avg_total_ms: number | null;
  avg_tps: number | null;
  uptime_pct: number;
}

export interface HeartbeatsResponse {
  mode: 'raw' | 'hourly';
  heartbeats: Heartbeat[];
  hourly_stats?: HourlyStat[];
}

export interface DiscoveredModel {
  id: string;
  owned_by?: string;
}

export interface AppSettings {
  retention_days: number;
  public_page_title: string;
  public_page_description: string;
}

export interface PublicHeartbeat {
  status: Status;
  models_latency_ms: number | null;
  ttft_ms: number | null;
  total_latency_ms: number | null;
  tokens_per_sec: number | null;
  checked_at: string;
}

export interface PublicMonitor {
  id: string;
  name: string;
  group_name: string;
  provider_type: ProviderType;
  model_id: string;
  interval_sec: number;
  is_active: boolean;
  latest: PublicHeartbeat | null;
  recent_heartbeats: PublicHeartbeat[];
  uptime_24h: number;
  uptime_30d: number;
  avg_tps_24h: number | null;
  avg_ttft_24h: number | null;
}

export interface PublicStatusResponse {
  title: string;
  description: string;
  overall_status: Status;
  monitors: PublicMonitor[];
  generated_at: string;
}
