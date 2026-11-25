import { useEffect, useState } from 'react';
import { Edit2, Pause, Play, RefreshCw, Trash2 } from 'lucide-react';
import { api } from '../api/client';
import type { HeartbeatsResponse, MonitorOverview } from '../types';
import { HeartbeatBar } from './HeartbeatBar';
import { MetricsCharts } from './MetricsCharts';
import { formatMs, formatTPS, StatusBadge } from './StatusBadge';

interface MonitorDetailProps {
  overview: MonitorOverview;
  onEdit: () => void;
  onDelete: () => void;
  onTogglePause: () => void;
  onCheckNow: () => Promise<void>;
}

export function MonitorDetail({
  overview,
  onEdit,
  onDelete,
  onTogglePause,
  onCheckNow,
}: MonitorDetailProps) {
  const { monitor, latest, recent_heartbeats: rawRecent, uptime_24h, avg_tps_24h, avg_ttft_24h } = overview;
  const recent_heartbeats = rawRecent ?? [];
  const [range, setRange] = useState<'1h' | '24h' | '7d' | '30d'>('24h');
  const [series, setSeries] = useState<HeartbeatsResponse | null>(null);
  const [checking, setChecking] = useState(false);

  useEffect(() => {
    let active = true;
    api
      .getHeartbeats(monitor.id, range)
      .then((res) => {
        if (active) setSeries(res);
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, [monitor.id, range, latest?.id]);

  const handleCheck = async () => {
    setChecking(true);
    try {
      await onCheckNow();
    } finally {
      setChecking(false);
    }
  };

  return (
    <div className="p-6 space-y-6 max-w-6xl mx-auto">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-3 flex-wrap">
            <h1 className="text-xl font-semibold text-zinc-100">{monitor.name}</h1>
            <StatusBadge status={latest?.status} isActive={monitor.is_active} />
            {monitor.group_name && (
              <span className="rounded-md bg-zinc-900 border border-zinc-800 px-2 py-0.5 text-xs text-zinc-400">
                {monitor.group_name}
              </span>
            )}
          </div>
          <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-zinc-400 font-mono">
            <span className="uppercase text-zinc-300">{monitor.provider_type}</span>
            {monitor.provider_type === 'openai' && (
              <>
                <span>·</span>
                <span className="text-zinc-300">
                  {monitor.openai_endpoint === 'responses' ? '/responses' : '/chat/completions'}
                </span>
              </>
            )}
            <span>·</span>
            <span>{monitor.model_id}</span>
            <span>·</span>
            <span className="text-zinc-500">{monitor.base_url}</span>
            <span>·</span>
            <span>every {Math.max(1, Math.round(monitor.interval_sec / 60))}m</span>
            {monitor.user_agent && (
              <>
                <span>·</span>
                <span className="text-zinc-300" title={`User-Agent: ${monitor.user_agent}`}>
                  UA: {monitor.user_agent}
                </span>
              </>
            )}
          </div>
        </div>

        <div className="flex items-center gap-2 shrink-0">
          <button
            onClick={handleCheck}
            disabled={checking}
            className="inline-flex items-center gap-1.5 rounded-lg border border-zinc-800 bg-zinc-900 hover:bg-zinc-800 px-3 py-1.5 text-xs font-medium text-zinc-200 disabled:opacity-50 cursor-pointer"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${checking ? 'animate-spin' : ''}`} />
            Check Now
          </button>
          <button
            onClick={onTogglePause}
            className="inline-flex items-center gap-1.5 rounded-lg border border-zinc-800 bg-zinc-900 hover:bg-zinc-800 px-3 py-1.5 text-xs font-medium text-zinc-200 cursor-pointer"
          >
            {monitor.is_active ? <Pause className="w-3.5 h-3.5" /> : <Play className="w-3.5 h-3.5" />}
            {monitor.is_active ? 'Pause' : 'Resume'}
          </button>
          <button
            onClick={onEdit}
            className="inline-flex items-center gap-1.5 rounded-lg border border-zinc-800 bg-zinc-900 hover:bg-zinc-800 px-3 py-1.5 text-xs font-medium text-zinc-200 cursor-pointer"
          >
            <Edit2 className="w-3.5 h-3.5" />
            Edit
          </button>
          <button
            onClick={onDelete}
            className="inline-flex items-center gap-1.5 rounded-lg border border-rose-500/30 bg-rose-500/10 hover:bg-rose-500/20 px-3 py-1.5 text-xs font-medium text-rose-400 cursor-pointer"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* KPI Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <div className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-4">
          <div className="text-xs text-zinc-400">Latest Speed</div>
          <div className="mt-1 text-xl font-semibold font-mono tabular-nums text-emerald-400">
            {formatTPS(latest?.tokens_per_sec)}
          </div>
          <div className="mt-1 text-[11px] text-zinc-500 tabular-nums">
            24h avg: {formatTPS(avg_tps_24h)}
          </div>
        </div>

        <div className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-4">
          <div className="text-xs text-zinc-400">TTFT (First Token)</div>
          <div className="mt-1 text-xl font-semibold font-mono tabular-nums text-sky-400">
            {formatMs(latest?.ttft_ms)}
          </div>
          <div className="mt-1 text-[11px] text-zinc-500 tabular-nums">
            24h avg: {formatMs(avg_ttft_24h)}
          </div>
        </div>

        <div className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-4">
          <div className="text-xs text-zinc-400">24h Uptime</div>
          <div className="mt-1 text-xl font-semibold font-mono tabular-nums text-zinc-100">
            {uptime_24h.toFixed(2)}%
          </div>
          <div className="mt-1 text-[11px] text-zinc-500 tabular-nums">
            Total: {formatMs(latest?.total_latency_ms)}
          </div>
        </div>

        <div className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-4">
          <div className="text-xs text-zinc-400">/models Check</div>
          <div className="mt-1 text-xl font-semibold font-mono tabular-nums text-zinc-100">
            {monitor.check_models_first ? formatMs(latest?.models_latency_ms) : 'Disabled'}
          </div>
          <div className="mt-1 text-[11px] text-zinc-500">
            {latest?.tps_method ? `Method: ${latest.tps_method}` : 'Waiting for check'}
          </div>
        </div>
      </div>

      {/* Heartbeat Strip */}
      <div className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-4 space-y-2.5">
        <div className="flex items-center justify-between text-xs">
          <span className="font-medium text-zinc-300">Recent Heartbeats (last 48 checks)</span>
          <span className="text-zinc-500">Hover any tick for details</span>
        </div>
        <HeartbeatBar heartbeats={recent_heartbeats} maxTicks={48} />
      </div>

      {/* Range Selector + Charts */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-medium text-zinc-300">Performance History</h2>
          <div className="inline-flex rounded-lg border border-zinc-800 bg-zinc-900 p-0.5 text-xs">
            {(['1h', '24h', '7d', '30d'] as const).map((r) => (
              <button
                key={r}
                onClick={() => setRange(r)}
                className={`px-3 py-1 rounded-md transition-colors cursor-pointer ${
                  range === r ? 'bg-zinc-800 text-zinc-100 font-medium' : 'text-zinc-400 hover:text-zinc-200'
                }`}
              >
                {r}
              </button>
            ))}
          </div>
        </div>

        <MetricsCharts
          mode={series?.mode ?? 'raw'}
          heartbeats={series?.heartbeats ?? recent_heartbeats}
          hourlyStats={series?.hourly_stats}
        />
      </div>

      {/* Recent Checks Table */}
      <div className="rounded-xl border border-zinc-800 bg-zinc-900/60 overflow-hidden">
        <div className="px-4 py-3 border-b border-zinc-800 text-sm font-medium text-zinc-200">
          Recent Checks
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead className="bg-zinc-900 text-zinc-400 border-b border-zinc-800">
              <tr>
                <th className="px-4 py-2.5 font-medium">Time</th>
                <th className="px-4 py-2.5 font-medium">Status</th>
                <th className="px-4 py-2.5 font-medium">/models</th>
                <th className="px-4 py-2.5 font-medium">TTFT</th>
                <th className="px-4 py-2.5 font-medium">Total</th>
                <th className="px-4 py-2.5 font-medium">Speed</th>
                <th className="px-4 py-2.5 font-medium">Message</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-800/70 tabular-nums">
              {[...recent_heartbeats]
                .reverse()
                .slice(0, 15)
                .map((hb) => (
                  <tr key={hb.id} className="hover:bg-zinc-900/50">
                    <td className="px-4 py-2.5 text-zinc-400 whitespace-nowrap">
                      {new Date(hb.checked_at).toLocaleString()}
                    </td>
                    <td className="px-4 py-2.5">
                      <StatusBadge status={hb.status} />
                    </td>
                    <td className="px-4 py-2.5 font-mono text-zinc-300">
                      {formatMs(hb.models_latency_ms)}
                    </td>
                    <td className="px-4 py-2.5 font-mono text-sky-400">{formatMs(hb.ttft_ms)}</td>
                    <td className="px-4 py-2.5 font-mono text-zinc-300">
                      {formatMs(hb.total_latency_ms)}
                    </td>
                    <td className="px-4 py-2.5 font-mono text-emerald-400">
                      {formatTPS(hb.tokens_per_sec)}
                    </td>
                    <td className="px-4 py-2.5 text-zinc-400 max-w-xs truncate">
                      {hb.error_message || (hb.used_stream ? 'SSE stream OK' : 'Non-stream OK')}
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
