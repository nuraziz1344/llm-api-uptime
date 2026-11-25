import {
  Area,
  AreaChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import type { Heartbeat, HourlyStat } from '../types';

interface MetricsChartsProps {
  mode: 'raw' | 'hourly';
  heartbeats: Heartbeat[];
  hourlyStats?: HourlyStat[];
}

export function MetricsCharts({ mode, heartbeats, hourlyStats }: MetricsChartsProps) {
  const data =
    mode === 'hourly' && hourlyStats
      ? hourlyStats.map((s) => ({
          label: new Date(s.bucket_hour).toLocaleString([], {
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
          }),
          tps: s.avg_tps != null ? Number(s.avg_tps.toFixed(1)) : null,
          ttft: s.avg_ttft_ms != null ? Math.round(s.avg_ttft_ms) : null,
          total: s.avg_total_ms != null ? Math.round(s.avg_total_ms) : null,
        }))
      : heartbeats.map((hb) => ({
          label: new Date(hb.checked_at).toLocaleTimeString([], {
            hour: '2-digit',
            minute: '2-digit',
          }),
          tps: hb.tokens_per_sec != null ? Number(hb.tokens_per_sec.toFixed(1)) : null,
          ttft: hb.ttft_ms,
          total: hb.total_latency_ms,
        }));

  if (data.length === 0) {
    return (
      <div className="rounded-xl border border-zinc-800 bg-zinc-900/50 p-8 text-center text-sm text-zinc-500">
        No metric samples yet for this time range.
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
      {/* Generation Speed (tok/s) */}
      <div className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-4">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-medium text-zinc-200">Generation Speed (tok/s)</h3>
          <span className="text-xs text-emerald-400 font-mono">tokens / sec</span>
        </div>
        <div className="h-56">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data}>
              <defs>
                <linearGradient id="tpsGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#10b981" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="#27272a" vertical={false} />
              <XAxis dataKey="label" stroke="#71717a" fontSize={11} tickLine={false} minTickGap={24} />
              <YAxis stroke="#71717a" fontSize={11} tickLine={false} width={40} />
              <Tooltip
                contentStyle={{
                  backgroundColor: '#18181b',
                  borderColor: '#3f3f46',
                  borderRadius: '0.5rem',
                  fontSize: '12px',
                }}
              />
              <Area
                type="monotone"
                dataKey="tps"
                name="tok/s"
                stroke="#10b981"
                strokeWidth={2}
                fill="url(#tpsGrad)"
                connectNulls
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Latency (TTFT vs Total) */}
      <div className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-4">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-medium text-zinc-200">Latency (TTFT vs Total)</h3>
          <div className="flex items-center gap-3 text-xs">
            <span className="text-sky-400">● TTFT</span>
            <span className="text-violet-400">● Total</span>
          </div>
        </div>
        <div className="h-56">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={data}>
              <CartesianGrid strokeDasharray="3 3" stroke="#27272a" vertical={false} />
              <XAxis dataKey="label" stroke="#71717a" fontSize={11} tickLine={false} minTickGap={24} />
              <YAxis stroke="#71717a" fontSize={11} tickLine={false} width={48} unit="ms" />
              <Tooltip
                contentStyle={{
                  backgroundColor: '#18181b',
                  borderColor: '#3f3f46',
                  borderRadius: '0.5rem',
                  fontSize: '12px',
                }}
              />
              <Line
                type="monotone"
                dataKey="ttft"
                name="TTFT (ms)"
                stroke="#38bdf8"
                strokeWidth={2}
                dot={false}
                connectNulls
              />
              <Line
                type="monotone"
                dataKey="total"
                name="Total (ms)"
                stroke="#a78bfa"
                strokeWidth={2}
                dot={false}
                connectNulls
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  );
}
