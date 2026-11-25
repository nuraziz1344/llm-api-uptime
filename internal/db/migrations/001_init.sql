CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS monitors (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    group_name TEXT NOT NULL DEFAULT '',
    provider_type TEXT NOT NULL CHECK (provider_type IN ('openai', 'anthropic')),
    base_url TEXT NOT NULL,
    api_key_enc TEXT NOT NULL DEFAULT '',
    api_key_hint TEXT NOT NULL DEFAULT '',
    model_id TEXT NOT NULL,
    check_models_first INTEGER NOT NULL DEFAULT 1,
    stream_enabled INTEGER NOT NULL DEFAULT 1,
    fallback_non_stream INTEGER NOT NULL DEFAULT 1,
    prompt TEXT NOT NULL DEFAULT 'Count from 1 to 15 separated by spaces.',
    max_tokens INTEGER NOT NULL DEFAULT 24,
    interval_sec INTEGER NOT NULL DEFAULT 60,
    timeout_sec INTEGER NOT NULL DEFAULT 30,
    anthropic_version TEXT NOT NULL DEFAULT '2023-06-01',
    custom_headers_enc TEXT NOT NULL DEFAULT '',
    is_active INTEGER NOT NULL DEFAULT 1,
    is_public INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS heartbeats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    monitor_id TEXT NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('up', 'degraded', 'down')),
    stage_failed TEXT NOT NULL DEFAULT '',
    http_status INTEGER NOT NULL DEFAULT 0,
    models_ok INTEGER,
    models_latency_ms INTEGER,
    completion_ok INTEGER NOT NULL DEFAULT 0,
    used_stream INTEGER NOT NULL DEFAULT 0,
    ttft_ms INTEGER,
    total_latency_ms INTEGER,
    output_tokens INTEGER,
    tokens_per_sec REAL,
    tps_method TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    checked_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_heartbeats_monitor_time
    ON heartbeats(monitor_id, checked_at DESC);
CREATE INDEX IF NOT EXISTS idx_heartbeats_checked_at
    ON heartbeats(checked_at);

CREATE TABLE IF NOT EXISTS hourly_stats (
    monitor_id TEXT NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    bucket_hour TEXT NOT NULL,
    total_checks INTEGER NOT NULL DEFAULT 0,
    up_checks INTEGER NOT NULL DEFAULT 0,
    degraded_checks INTEGER NOT NULL DEFAULT 0,
    down_checks INTEGER NOT NULL DEFAULT 0,
    sum_ttft_ms REAL NOT NULL DEFAULT 0,
    ttft_count INTEGER NOT NULL DEFAULT 0,
    sum_total_ms REAL NOT NULL DEFAULT 0,
    total_ms_count INTEGER NOT NULL DEFAULT 0,
    sum_tps REAL NOT NULL DEFAULT 0,
    tps_count INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (monitor_id, bucket_hour)
);
