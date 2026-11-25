import { useEffect, useMemo, useState } from 'react';
import { Activity, AlertTriangle, CheckCircle2, XCircle } from 'lucide-react';
import { api } from '../api/client';
import { HeartbeatBar } from '../components/HeartbeatBar';
import { formatMs, formatTPS, StatusBadge } from '../components/StatusBadge';
import type { PublicHeartbeat, PublicMonitor, PublicStatusResponse } from '../types';

export function PublicStatusPage() {
  const [data, setData] = useState<PublicStatusResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .getPublicStatus()
      .then(setData)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load status'));

    const es = new EventSource('/api/public/events');
    es.addEventListener('heartbeat', (ev) => {
      try {
        const parsed = JSON.parse((ev as MessageEvent<string>).data) as {
          monitor_id: string;
          heartbeat: PublicHeartbeat;
        };
        setData((prev) => {
          if (!prev) return prev;
          const monitors = prev.monitors.map((m) => {
            if (m.id !== parsed.monitor_id) return m;
            return {
              ...m,
              latest: parsed.heartbeat,
              recent_heartbeats: [...m.recent_heartbeats, parsed.heartbeat].slice(-48),
            };
          });
          return { ...prev, monitors };
        });
      } catch {
        // ignore
      }
    });

    return () => es.close();
  }, []);

  const grouped = useMemo(() => {
    const map = new Map<string, PublicMonitor[]>();
    for (const m of data?.monitors ?? []) {
      const key = m.group_name || 'Endpoints';
      const arr = map.get(key) ?? [];
      arr.push(m);
      map.set(key, arr);
    }
    return Array.from(map.entries());
  }, [data]);

  if (error) {
    return (
      <div className="min-h-screen bg-zinc-950 flex items-center justify-center text-sm text-rose-400">
        {error}
      </div>
    );
  }

  if (!data) {
    return (
      <div className="min-h-screen bg-zinc-950 flex items-center justify-center text-sm text-zinc-500">
        Loading status...
      </div>
    );
  }

  const bannerConfig = {
    up: {
      cls: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-300',
      icon: <CheckCircle2 className="w-5 h-5 text-emerald-400" />,
      text: 'All Systems Operational',
    },
    degraded: {
      cls: 'border-amber-500/30 bg-amber-500/10 text-amber-300',
      icon: <AlertTriangle className="w-5 h-5 text-amber-400" />,
      text: 'Partial System Degradation',
    },
    down: {
      cls: 'border-rose-500/30 bg-rose-500/10 text-rose-300',
      icon: <XCircle className="w-5 h-5 text-rose-400" />,
      text: 'Service Disruption Detected',
    },
  }[data.overall_status];

  return (
    <div className="min-h-screen bg-zinc-950 text-zinc-100">
      <div className="max-w-4xl mx-auto px-4 py-10 space-y-8">
        {/* Header */}
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="h-9 w-9 rounded-xl bg-emerald-500/15 border border-emerald-500/30 flex items-center justify-center">
              <Activity className="w-5 h-5 text-emerald-400" />
            </div>
            <div>
              <h1 className="text-xl font-semibold text-zinc-100">{data.title}</h1>
              {data.description && <p className="text-xs text-zinc-400 mt-0.5">{data.description}</p>}
            </div>
          </div>
          <a
            href="/"
            className="text-xs text-zinc-400 hover:text-zinc-200 border border-zinc-800 rounded-lg px-3 py-1.5"
          >
            Dashboard
          </a>
        </div>

        {/* Overall Banner */}
        <div className={`rounded-xl border px-4 py-3.5 flex items-center justify-between ${bannerConfig.cls}`}>
          <div className="flex items-center gap-2.5 font-medium text-sm">
            {bannerConfig.icon}
            <span>{bannerConfig.text}</span>
          </div>
          <span className="text-xs opacity-75">Live via SSE</span>
        </div>

        {/* Groups */}
        {grouped.length === 0 ? (
          <div className="rounded-xl border border-zinc-800 bg-zinc-900/40 p-10 text-center text-sm text-zinc-500">
            No public monitors configured.
          </div>
        ) : (
          grouped.map(([group, items]) => (
            <div key={group} className="space-y-3">
              <h2 className="text-xs font-semibold uppercase tracking-wider text-zinc-400">
                {group}
              </h2>
              <div className="space-y-3">
                {items.map((m) => (
                  <div
                    key={m.id}
                    className="rounded-xl border border-zinc-800 bg-zinc-900/50 p-4 space-y-3"
                  >
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div>
                        <div className="flex items-center gap-2.5">
                          <span className="font-medium text-sm text-zinc-100">{m.name}</span>
                          <StatusBadge status={m.latest?.status} isActive={m.is_active} />
                        </div>
                        <div className="text-xs font-mono text-zinc-500 mt-0.5">
                          {m.provider_type} · {m.model_id}
                        </div>
                      </div>

                      <div className="flex items-center gap-5 text-xs tabular-nums">
                        <div className="text-right">
                          <div className="text-zinc-500">Speed</div>
                          <div className="font-mono text-emerald-400">
                            {formatTPS(m.latest?.tokens_per_sec ?? m.avg_tps_24h)}
                          </div>
                        </div>
                        <div className="text-right">
                          <div className="text-zinc-500">TTFT</div>
                          <div className="font-mono text-sky-400">
                            {formatMs(m.latest?.ttft_ms ?? m.avg_ttft_24h)}
                          </div>
                        </div>
                        <div className="text-right">
                          <div className="text-zinc-500">24h / 30d</div>
                          <div className="font-mono text-zinc-200">
                            {m.uptime_24h.toFixed(1)}% · {m.uptime_30d.toFixed(1)}%
                          </div>
                        </div>
                      </div>
                    </div>

                    <HeartbeatBar heartbeats={m.recent_heartbeats} maxTicks={48} />
                  </div>
                ))}
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
