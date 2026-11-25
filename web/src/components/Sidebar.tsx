import { Plus, Search } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { MonitorOverview } from '../types';
import { HeartbeatBar } from './HeartbeatBar';
import { formatTPS, statusColor } from './StatusBadge';

interface SidebarProps {
  monitors: MonitorOverview[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  onAdd: () => void;
}

export function Sidebar({ monitors, selectedId, onSelect, onAdd }: SidebarProps) {
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<'all' | 'up' | 'degraded' | 'down'>('all');

  const counts = useMemo(() => {
    let up = 0;
    let degraded = 0;
    let down = 0;
    for (const item of monitors) {
      if (!item.monitor.is_active) continue;
      if (item.latest?.status === 'up') up++;
      else if (item.latest?.status === 'degraded') degraded++;
      else if (item.latest?.status === 'down') down++;
    }
    return { up, degraded, down };
  }, [monitors]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return monitors.filter((item) => {
      if (filter !== 'all' && item.latest?.status !== filter) return false;
      if (!q) return true;
      return (
        item.monitor.name.toLowerCase().includes(q) ||
        item.monitor.model_id.toLowerCase().includes(q) ||
        item.monitor.group_name.toLowerCase().includes(q)
      );
    });
  }, [monitors, query, filter]);

  return (
    <aside className="flex flex-col h-full border-r border-zinc-800 bg-zinc-950/80 w-full md:w-80 lg:w-96 shrink-0">
      <div className="p-4 border-b border-zinc-800 space-y-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-xs tabular-nums">
            <span className="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
              {counts.up} Up
            </span>
            {counts.degraded > 0 && (
              <span className="px-2 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20">
                {counts.degraded} Deg
              </span>
            )}
            {counts.down > 0 && (
              <span className="px-2 py-0.5 rounded bg-rose-500/10 text-rose-400 border border-rose-500/20">
                {counts.down} Down
              </span>
            )}
          </div>
          <button
            onClick={onAdd}
            className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 px-3 py-1.5 text-xs font-medium text-white transition-colors cursor-pointer"
          >
            <Plus className="w-3.5 h-3.5" />
            Add Monitor
          </button>
        </div>

        <div className="relative">
          <Search className="w-3.5 h-3.5 text-zinc-500 absolute left-3 top-1/2 -translate-y-1/2" />
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Filter name, model, group..."
            className="w-full rounded-lg border border-zinc-800 bg-zinc-900 pl-8 pr-3 py-1.5 text-xs text-zinc-200 placeholder:text-zinc-500 focus:border-emerald-500 focus:outline-none"
          />
        </div>

        <div className="flex gap-1 text-[11px]">
          {(['all', 'up', 'degraded', 'down'] as const).map((f) => (
            <button
              key={f}
              onClick={() => setFilter(f)}
              className={`px-2 py-1 rounded capitalize transition-colors cursor-pointer ${
                filter === f
                  ? 'bg-zinc-800 text-zinc-100 font-medium'
                  : 'text-zinc-400 hover:text-zinc-200'
              }`}
            >
              {f}
            </button>
          ))}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-2 space-y-1.5">
        {filtered.length === 0 ? (
          <div className="p-6 text-center text-xs text-zinc-500">
            No monitors match your filter.
          </div>
        ) : (
          filtered.map((item) => {
            const selected = item.monitor.id === selectedId;
            return (
              <button
                key={item.monitor.id}
                onClick={() => onSelect(item.monitor.id)}
                className={`w-full text-left rounded-xl p-3 border transition-colors cursor-pointer ${
                  selected
                    ? 'bg-zinc-900 border-emerald-500/40'
                    : 'bg-zinc-900/30 border-transparent hover:bg-zinc-900/70 hover:border-zinc-800'
                }`}
              >
                <div className="flex items-center justify-between gap-2 mb-1">
                  <div className="flex items-center gap-2 min-w-0">
                    <span
                      className={`h-2.5 w-2.5 rounded-full shrink-0 ${statusColor(
                        item.latest?.status,
                        item.monitor.is_active
                      )}`}
                    />
                    <span className="font-medium text-sm text-zinc-100 truncate">
                      {item.monitor.name}
                    </span>
                  </div>
                  <span className="text-xs font-mono tabular-nums text-zinc-400 shrink-0">
                    {item.uptime_24h.toFixed(1)}%
                  </span>
                </div>

                <div className="flex items-center justify-between text-[11px] text-zinc-400 mb-2">
                  <span className="truncate font-mono text-zinc-500">
                    {item.monitor.provider_type} · {item.monitor.model_id}
                  </span>
                  <span className="font-mono tabular-nums text-emerald-400/90 shrink-0 ml-2">
                    {formatTPS(item.latest?.tokens_per_sec)}
                  </span>
                </div>

                <HeartbeatBar heartbeats={item.recent_heartbeats} maxTicks={28} compact />
              </button>
            );
          })
        )}
      </div>
    </aside>
  );
}
