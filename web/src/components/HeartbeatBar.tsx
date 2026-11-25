import { useState } from 'react';
import type { Heartbeat, PublicHeartbeat } from '../types';
import { formatMs, formatTPS, statusColor } from './StatusBadge';

type TickItem = Heartbeat | PublicHeartbeat;

interface HeartbeatBarProps {
  heartbeats: TickItem[];
  maxTicks?: number;
  compact?: boolean;
}

export function HeartbeatBar({ heartbeats, maxTicks = 48, compact = false }: HeartbeatBarProps) {
  const [hovered, setHovered] = useState<number | null>(null);

  const slice = (heartbeats ?? []).slice(-maxTicks);
  const padCount = Math.max(0, maxTicks - slice.length);

  return (
    <div className="relative">
      <div className={`flex items-center ${compact ? 'gap-[2px]' : 'gap-[3px]'}`}>
        {Array.from({ length: padCount }).map((_, idx) => (
          <div
            key={`pad-${idx}`}
            className={`flex-1 rounded-[2px] bg-zinc-800/70 ${compact ? 'h-4' : 'h-7'}`}
          />
        ))}
        {slice.map((hb, idx) => {
          const isHovered = hovered === idx;
          return (
            <div
              key={`${hb.checked_at}-${idx}`}
              onMouseEnter={() => setHovered(idx)}
              onMouseLeave={() => setHovered(null)}
              className={`relative flex-1 rounded-[2px] transition-transform cursor-pointer ${
                compact ? 'h-4' : 'h-7'
              } ${statusColor(hb.status)} ${isHovered ? 'scale-y-125 brightness-125 z-10' : ''}`}
            >
              {!compact && isHovered && (
                <div className="pointer-events-none absolute bottom-full left-1/2 -translate-x-1/2 mb-2 w-52 rounded-lg border border-zinc-700 bg-zinc-900/95 p-2.5 text-xs shadow-xl backdrop-blur z-50">
                  <div className="flex items-center justify-between font-medium text-zinc-100 mb-1">
                    <span className="capitalize">{hb.status}</span>
                    <span className="text-[11px] text-zinc-400">
                      {new Date(hb.checked_at).toLocaleTimeString()}
                    </span>
                  </div>
                  <div className="space-y-0.5 text-[11px] text-zinc-300 tabular-nums">
                    <div className="flex justify-between">
                      <span className="text-zinc-400">Speed:</span>
                      <span>{formatTPS(hb.tokens_per_sec)}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-zinc-400">TTFT:</span>
                      <span>{formatMs(hb.ttft_ms)}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-zinc-400">Total:</span>
                      <span>{formatMs(hb.total_latency_ms)}</span>
                    </div>
                    {hb.models_latency_ms != null && (
                      <div className="flex justify-between">
                        <span className="text-zinc-400">/models:</span>
                        <span>{formatMs(hb.models_latency_ms)}</span>
                      </div>
                    )}
                    {'error_message' in hb && hb.error_message && (
                      <div className="mt-1 pt-1 border-t border-zinc-800 text-rose-400 break-words">
                        {hb.error_message}
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
