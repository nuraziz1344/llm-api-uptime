import { useState } from 'react';
import { Loader2, RefreshCw, Sparkles, X } from 'lucide-react';
import { api } from '../api/client';
import type {
  DiscoveredModel,
  Heartbeat,
  Monitor,
  MonitorInput,
  OpenAIEndpoint,
  ProviderType,
} from '../types';
import { formatMs, formatTPS, StatusBadge } from './StatusBadge';

interface MonitorFormModalProps {
  initial?: Monitor | null;
  onClose: () => void;
  onSaved: (monitor: Monitor) => void;
}

const PRESETS: { label: string; provider: ProviderType; baseUrl: string; model: string }[] = [
  { label: 'OpenAI', provider: 'openai', baseUrl: 'https://api.openai.com/v1', model: 'gpt-4o-mini' },
  { label: 'Anthropic', provider: 'anthropic', baseUrl: 'https://api.anthropic.com', model: 'claude-3-5-sonnet-latest' },
  { label: 'OpenRouter', provider: 'openai', baseUrl: 'https://openrouter.ai/api/v1', model: 'openai/gpt-4o-mini' },
  { label: 'DeepSeek', provider: 'openai', baseUrl: 'https://api.deepseek.com/v1', model: 'deepseek-chat' },
  { label: 'Groq', provider: 'openai', baseUrl: 'https://api.groq.com/openai/v1', model: 'llama-3.1-8b-instant' },
  { label: 'Ollama (Local)', provider: 'openai', baseUrl: 'http://localhost:11434/v1', model: 'llama3.2' },
];

const USER_AGENT_PRESETS: { label: string; value: string }[] = [
  { label: 'Default (Go HTTP Client)', value: '' },
  { label: 'Claude Code', value: 'claude-cli/1.0.0 (external, cli)' },
  { label: 'OpenAI Codex CLI', value: 'codex_cli_rs/0.1.0' },
  { label: 'OpenCode', value: 'opencode/0.1.0' },
  { label: 'Cursor', value: 'Cursor/1.0' },
  { label: 'Cline', value: 'Cline/3.0' },
  { label: 'OpenAI Python SDK', value: 'OpenAI/Python 1.65.0' },
  { label: 'Anthropic JS SDK', value: 'Anthropic/JS 0.39.0' },
];

export function MonitorFormModal({ initial, onClose, onSaved }: MonitorFormModalProps) {
  const isEdit = Boolean(initial);
  const [name, setName] = useState(initial?.name ?? '');
  const [groupName, setGroupName] = useState(initial?.group_name ?? '');
  const [providerType, setProviderType] = useState<ProviderType>(initial?.provider_type ?? 'openai');
  const [openaiEndpoint, setOpenaiEndpoint] = useState<OpenAIEndpoint>(
    initial?.openai_endpoint ?? 'chat_completions'
  );
  const [baseUrl, setBaseUrl] = useState(initial?.base_url ?? 'https://api.openai.com/v1');
  const [apiKey, setApiKey] = useState('');
  const [modelId, setModelId] = useState(initial?.model_id ?? '');
  const [checkModelsFirst, setCheckModelsFirst] = useState(initial?.check_models_first ?? true);
  const [streamEnabled, setStreamEnabled] = useState(initial?.stream_enabled ?? true);
  const [fallbackNonStream, setFallbackNonStream] = useState(initial?.fallback_non_stream ?? true);
  const [prompt, setPrompt] = useState(initial?.prompt ?? 'Count from 1 to 15 separated by spaces.');
  const [maxTokens, setMaxTokens] = useState(initial?.max_tokens ?? 24);
  const [intervalMin, setIntervalMin] = useState(
    initial?.interval_sec ? Math.max(1, Math.round(initial.interval_sec / 60)) : 1
  );
  const [timeoutSec, setTimeoutSec] = useState(initial?.timeout_sec ?? 30);
  const [anthropicVersion, setAnthropicVersion] = useState(initial?.anthropic_version ?? '2023-06-01');
  const [userAgent, setUserAgent] = useState(initial?.user_agent ?? '');
  const [customUA, setCustomUA] = useState(
    Boolean(initial?.user_agent && !USER_AGENT_PRESETS.some((p) => p.value === initial.user_agent))
  );
  const [isPublic, setIsPublic] = useState(initial?.is_public ?? true);

  const [discovered, setDiscovered] = useState<DiscoveredModel[]>([]);
  const [discovering, setDiscovering] = useState(false);
  const [discoverError, setDiscoverError] = useState<string | null>(null);

  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<Heartbeat | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const buildPayload = (): MonitorInput => {
    const payload: MonitorInput = {
      name,
      group_name: groupName,
      provider_type: providerType,
      openai_endpoint: openaiEndpoint,
      base_url: baseUrl,
      model_id: modelId,
      check_models_first: checkModelsFirst,
      stream_enabled: streamEnabled,
      fallback_non_stream: fallbackNonStream,
      prompt,
      max_tokens: Number(maxTokens) || 24,
      interval_sec: Math.max(1, Number(intervalMin) || 1) * 60,
      timeout_sec: Number(timeoutSec) || 30,
      anthropic_version: anthropicVersion,
      user_agent: userAgent.trim(),
      is_active: initial?.is_active ?? true,
      is_public: isPublic,
    };
    if (apiKey.trim() !== '') {
      payload.api_key = apiKey.trim();
    }
    return payload;
  };

  const handleDiscover = async () => {
    setDiscovering(true);
    setDiscoverError(null);
    try {
      const res = await api.discoverModels({
        monitor_id: initial?.id,
        provider_type: providerType,
        base_url: baseUrl,
        api_key: apiKey.trim() || undefined,
        anthropic_version: anthropicVersion,
        user_agent: userAgent.trim() || undefined,
      });
      setDiscovered(res.models ?? []);
      if (res.models?.length && !modelId) {
        setModelId(res.models[0].id);
      }
    } catch (err) {
      setDiscoverError(err instanceof Error ? err.message : 'Failed to fetch /models');
    } finally {
      setDiscovering(false);
    }
  };

  const handleTest = async () => {
    setTesting(true);
    setTestResult(null);
    setError(null);
    try {
      const hb = await api.testProbe({
        ...buildPayload(),
        monitor_id: initial?.id,
      });
      setTestResult(hb);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Test check failed');
    } finally {
      setTesting(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      const payload = buildPayload();
      const saved = isEdit && initial
        ? await api.updateMonitor(initial.id, payload)
        : await api.createMonitor(payload);
      onSaved(saved);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save monitor');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-4 backdrop-blur-sm">
      <div className="w-full max-w-2xl max-h-[90vh] overflow-y-auto rounded-2xl border border-zinc-800 bg-zinc-950 shadow-2xl">
        <div className="sticky top-0 z-10 flex items-center justify-between border-b border-zinc-800 bg-zinc-950/95 px-6 py-4 backdrop-blur">
          <h2 className="text-base font-semibold text-zinc-100">
            {isEdit ? `Edit Monitor — ${initial?.name}` : 'Add LLM Monitor'}
          </h2>
          <button onClick={onClose} className="text-zinc-400 hover:text-zinc-200 cursor-pointer">
            <X className="w-5 h-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-5">
          {!isEdit && (
            <div>
              <div className="text-xs font-medium text-zinc-400 mb-2">Quick Presets</div>
              <div className="flex flex-wrap gap-1.5">
                {PRESETS.map((p) => (
                  <button
                    type="button"
                    key={p.label}
                    onClick={() => {
                      setProviderType(p.provider);
                      setBaseUrl(p.baseUrl);
                      if (!modelId) setModelId(p.model);
                      if (!name) setName(`${p.label} · ${p.model}`);
                    }}
                    className="rounded-md border border-zinc-800 bg-zinc-900 hover:border-emerald-500/40 px-2.5 py-1 text-xs text-zinc-300 transition-colors cursor-pointer"
                  >
                    {p.label}
                  </button>
                ))}
              </div>
            </div>
          )}

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-zinc-300 mb-1">Display Name *</label>
              <input
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="OpenAI Production"
                className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 focus:border-emerald-500 focus:outline-none"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-zinc-300 mb-1">Group (optional)</label>
              <input
                value={groupName}
                onChange={(e) => setGroupName(e.target.value)}
                placeholder="Production Gateways"
                className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 focus:border-emerald-500 focus:outline-none"
              />
            </div>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div>
              <label className="block text-xs font-medium text-zinc-300 mb-1">Provider Protocol *</label>
              <select
                value={providerType}
                onChange={(e) => setProviderType(e.target.value as ProviderType)}
                className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 focus:border-emerald-500 focus:outline-none"
              >
                <option value="openai">OpenAI-compatible</option>
                <option value="anthropic">Anthropic-compatible</option>
              </select>
            </div>
            <div className="sm:col-span-2">
              <label className="block text-xs font-medium text-zinc-300 mb-1">Base URL *</label>
              <input
                required
                value={baseUrl}
                onChange={(e) => setBaseUrl(e.target.value)}
                placeholder="https://api.openai.com/v1"
                className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm font-mono text-zinc-100 focus:border-emerald-500 focus:outline-none"
              />
            </div>
          </div>

          {providerType === 'openai' && (
            <div>
              <label className="block text-xs font-medium text-zinc-300 mb-1">
                OpenAI Inference Endpoint
              </label>
              <select
                value={openaiEndpoint}
                onChange={(e) => setOpenaiEndpoint(e.target.value as OpenAIEndpoint)}
                className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 focus:border-emerald-500 focus:outline-none"
              >
                <option value="chat_completions">Chat Completions (/chat/completions)</option>
                <option value="responses">Responses API (/responses)</option>
              </select>
            </div>
          )}

          <div>
            <label className="block text-xs font-medium text-zinc-300 mb-1">
              API Key {isEdit && initial?.has_api_key ? `(saved: ${initial.api_key_hint} — leave blank to keep)` : ''}
            </label>
            <input
              type="password"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder={isEdit && initial?.has_api_key ? '••••••••••••' : 'sk-...'}
              className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm font-mono text-zinc-100 focus:border-emerald-500 focus:outline-none"
            />
          </div>

          <div>
            <div className="flex items-center justify-between mb-1">
              <label className="text-xs font-medium text-zinc-300">Model ID *</label>
              <button
                type="button"
                onClick={handleDiscover}
                disabled={discovering || !baseUrl}
                className="inline-flex items-center gap-1 text-xs text-emerald-400 hover:text-emerald-300 disabled:opacity-50 cursor-pointer"
              >
                {discovering ? <Loader2 className="w-3 h-3 animate-spin" /> : <RefreshCw className="w-3 h-3" />}
                Fetch /models
              </button>
            </div>
            <div className="flex gap-2">
              <input
                required
                value={modelId}
                onChange={(e) => setModelId(e.target.value)}
                placeholder="gpt-4o-mini"
                className="flex-1 rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm font-mono text-zinc-100 focus:border-emerald-500 focus:outline-none"
              />
              {discovered.length > 0 && (
                <select
                  value={modelId}
                  onChange={(e) => setModelId(e.target.value)}
                  className="max-w-56 rounded-lg border border-zinc-800 bg-zinc-900 px-2.5 py-2 text-xs font-mono text-zinc-200 focus:border-emerald-500 focus:outline-none"
                >
                  {discovered.map((m) => (
                    <option key={m.id} value={m.id}>
                      {m.id}
                    </option>
                  ))}
                </select>
              )}
            </div>
            {discoverError && <p className="mt-1 text-xs text-rose-400">{discoverError}</p>}
          </div>

          <div className="rounded-xl border border-zinc-800 bg-zinc-900/40 p-4 space-y-3">
            <div className="text-xs font-medium text-zinc-300">Probing Behavior</div>
            <label className="flex items-center gap-2.5 text-xs text-zinc-300 cursor-pointer">
              <input
                type="checkbox"
                checked={checkModelsFirst}
                onChange={(e) => setCheckModelsFirst(e.target.checked)}
                className="rounded border-zinc-700 bg-zinc-900 text-emerald-500"
              />
              Pre-flight check <code className="text-zinc-400">/models</code> before completion
            </label>
            <label className="flex items-center gap-2.5 text-xs text-zinc-300 cursor-pointer">
              <input
                type="checkbox"
                checked={streamEnabled}
                onChange={(e) => setStreamEnabled(e.target.checked)}
                className="rounded border-zinc-700 bg-zinc-900 text-emerald-500"
              />
              Use SSE streaming to measure TTFT &amp; <code className="text-zinc-400">tok/s</code>
            </label>
            <label className="flex items-center gap-2.5 text-xs text-zinc-300 cursor-pointer">
              <input
                type="checkbox"
                checked={fallbackNonStream}
                onChange={(e) => setFallbackNonStream(e.target.checked)}
                className="rounded border-zinc-700 bg-zinc-900 text-emerald-500"
              />
              Fallback to non-streaming if provider rejects stream
            </label>
            <label className="flex items-center gap-2.5 text-xs text-zinc-300 cursor-pointer">
              <input
                type="checkbox"
                checked={isPublic}
                onChange={(e) => setIsPublic(e.target.checked)}
                className="rounded border-zinc-700 bg-zinc-900 text-emerald-500"
              />
              Show on Public Status Page (<code className="text-zinc-400">/status</code>)
            </label>
          </div>

          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className="block text-xs font-medium text-zinc-300 mb-1">Interval (min)</label>
              <input
                type="number"
                min={1}
                value={intervalMin}
                onChange={(e) => setIntervalMin(Number(e.target.value))}
                className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm tabular-nums text-zinc-100 focus:border-emerald-500 focus:outline-none"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-zinc-300 mb-1">Timeout (sec)</label>
              <input
                type="number"
                min={3}
                value={timeoutSec}
                onChange={(e) => setTimeoutSec(Number(e.target.value))}
                className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm tabular-nums text-zinc-100 focus:border-emerald-500 focus:outline-none"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-zinc-300 mb-1">Max Tokens</label>
              <input
                type="number"
                min={1}
                max={256}
                value={maxTokens}
                onChange={(e) => setMaxTokens(Number(e.target.value))}
                className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm tabular-nums text-zinc-100 focus:border-emerald-500 focus:outline-none"
              />
            </div>
          </div>

          {providerType === 'anthropic' && (
            <div>
              <label className="block text-xs font-medium text-zinc-300 mb-1">Anthropic Version Header</label>
              <input
                value={anthropicVersion}
                onChange={(e) => setAnthropicVersion(e.target.value)}
                className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm font-mono text-zinc-100 focus:border-emerald-500 focus:outline-none"
              />
            </div>
          )}

          <div>
            <label className="block text-xs font-medium text-zinc-300 mb-1">
              User-Agent (Simulate Client)
            </label>
            <div className="flex flex-col sm:flex-row gap-2">
              <select
                value={customUA ? '__custom__' : userAgent}
                onChange={(e) => {
                  const val = e.target.value;
                  if (val === '__custom__') {
                    setCustomUA(true);
                  } else {
                    setCustomUA(false);
                    setUserAgent(val);
                  }
                }}
                className="sm:w-60 rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 focus:border-emerald-500 focus:outline-none"
              >
                {USER_AGENT_PRESETS.map((p) => (
                  <option key={p.label} value={p.value}>
                    {p.label}
                  </option>
                ))}
                <option value="__custom__">Custom User-Agent...</option>
              </select>

              {customUA ? (
                <input
                  value={userAgent}
                  onChange={(e) => setUserAgent(e.target.value)}
                  placeholder="my-client/1.0.0"
                  className="flex-1 rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm font-mono text-zinc-100 focus:border-emerald-500 focus:outline-none"
                />
              ) : (
                <div className="flex-1 rounded-lg border border-zinc-800/60 bg-zinc-900/40 px-3 py-2 text-xs font-mono text-zinc-400 flex items-center truncate">
                  {userAgent || 'Default Go-http-client'}
                </div>
              )}
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-zinc-300 mb-1">Probe Prompt</label>
            <input
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 focus:border-emerald-500 focus:outline-none"
            />
          </div>

          {testResult && (
            <div className="rounded-xl border border-zinc-800 bg-zinc-900/70 p-3.5 text-xs space-y-2">
              <div className="flex items-center justify-between">
                <span className="font-medium text-zinc-200">Test Probe Result</span>
                <StatusBadge status={testResult.status} />
              </div>
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 tabular-nums text-zinc-300">
                <div>
                  <div className="text-zinc-500">/models</div>
                  <div>{formatMs(testResult.models_latency_ms)}</div>
                </div>
                <div>
                  <div className="text-zinc-500">TTFT</div>
                  <div>{formatMs(testResult.ttft_ms)}</div>
                </div>
                <div>
                  <div className="text-zinc-500">Total</div>
                  <div>{formatMs(testResult.total_latency_ms)}</div>
                </div>
                <div>
                  <div className="text-zinc-500">Speed</div>
                  <div className="text-emerald-400">{formatTPS(testResult.tokens_per_sec)}</div>
                </div>
              </div>
              {testResult.error_message && (
                <div className="text-rose-400 font-mono break-words">{testResult.error_message}</div>
              )}
            </div>
          )}

          {error && (
            <div className="rounded-lg border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-xs text-rose-300">
              {error}
            </div>
          )}

          <div className="flex items-center justify-between pt-2 border-t border-zinc-800">
            <button
              type="button"
              onClick={handleTest}
              disabled={testing || !baseUrl || !modelId}
              className="inline-flex items-center gap-1.5 rounded-lg border border-zinc-700 bg-zinc-900 hover:bg-zinc-800 px-3.5 py-2 text-xs font-medium text-zinc-200 disabled:opacity-50 cursor-pointer"
            >
              {testing ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Sparkles className="w-3.5 h-3.5 text-emerald-400" />}
              Test Check
            </button>

            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={onClose}
                className="rounded-lg px-4 py-2 text-xs font-medium text-zinc-400 hover:text-zinc-200 cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={saving}
                className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 px-4 py-2 text-xs font-medium text-white disabled:opacity-50 cursor-pointer"
              >
                {saving && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                {isEdit ? 'Save Changes' : 'Create Monitor'}
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
}
