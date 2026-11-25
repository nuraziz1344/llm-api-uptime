import { useEffect, useState } from 'react';
import { Activity, ExternalLink, LogOut, Plus, Settings } from 'lucide-react';
import { api } from '../api/client';
import { MonitorDetail } from '../components/MonitorDetail';
import { MonitorFormModal } from '../components/MonitorFormModal';
import { Sidebar } from '../components/Sidebar';
import type { Heartbeat, Monitor, MonitorOverview } from '../types';

interface DashboardPageProps {
  username: string;
  onLogout: () => void;
  onOpenSettings: () => void;
}

export function DashboardPage({ username, onLogout, onOpenSettings }: DashboardPageProps) {
  const [monitors, setMonitors] = useState<MonitorOverview[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [modalMonitor, setModalMonitor] = useState<Monitor | null | undefined>(undefined);
  const [loading, setLoading] = useState(true);

  const loadMonitors = async () => {
    try {
      const list = await api.listMonitors();
      setMonitors(list);
      if (list.length > 0 && (!selectedId || !list.some((m) => m.monitor.id === selectedId))) {
        setSelectedId(list[0].monitor.id);
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadMonitors();

    const es = new EventSource('/api/events', { withCredentials: true });
    es.addEventListener('heartbeat', (ev) => {
      try {
        const hb = JSON.parse((ev as MessageEvent<string>).data) as Heartbeat;
        setMonitors((prev) =>
          prev.map((item) => {
            if (item.monitor.id !== hb.monitor_id) return item;
            const updatedRecent = [...(item.recent_heartbeats ?? []), hb].slice(-48);
            return {
              ...item,
              latest: hb,
              recent_heartbeats: updatedRecent,
            };
          })
        );
      } catch {
        // ignore parse errors
      }
    });

    return () => es.close();
  }, []);

  const selected = monitors.find((m) => m.monitor.id === selectedId) ?? null;

  const handleDelete = async (id: string) => {
    if (!window.confirm('Delete this monitor and all its heartbeat history?')) return;
    await api.deleteMonitor(id);
    setMonitors((prev) => {
      const next = prev.filter((m) => m.monitor.id !== id);
      if (selectedId === id) {
        setSelectedId(next[0]?.monitor.id ?? null);
      }
      return next;
    });
  };

  const handleTogglePause = async (m: Monitor) => {
    const updated = await api.setPause(m.id, !m.is_active);
    setMonitors((prev) =>
      prev.map((item) => (item.monitor.id === m.id ? { ...item, monitor: updated } : item))
    );
  };

  const handleCheckNow = async (id: string) => {
    const hb = await api.checkNow(id);
    setMonitors((prev) =>
      prev.map((item) => {
        if (item.monitor.id !== id) return item;
        return {
          ...item,
          latest: hb,
          recent_heartbeats: [...(item.recent_heartbeats ?? []), hb].slice(-48),
        };
      })
    );
  };

  return (
    <div className="h-screen flex flex-col bg-zinc-950 text-zinc-100">
      {/* Top Navbar */}
      <header className="h-14 border-b border-zinc-800 px-4 flex items-center justify-between shrink-0 bg-zinc-950">
        <div className="flex items-center gap-2.5">
          <div className="h-8 w-8 rounded-lg bg-emerald-500/15 border border-emerald-500/30 flex items-center justify-center">
            <Activity className="w-4 h-4 text-emerald-400" />
          </div>
          <span className="font-semibold text-sm tracking-tight">LLM API Uptime</span>
        </div>

        <div className="flex items-center gap-2">
          <a
            href="/status"
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1.5 rounded-lg border border-zinc-800 bg-zinc-900 hover:bg-zinc-800 px-3 py-1.5 text-xs text-zinc-300 transition-colors"
          >
            <ExternalLink className="w-3.5 h-3.5" />
            Status Page
          </a>
          <button
            onClick={onOpenSettings}
            className="inline-flex items-center gap-1.5 rounded-lg border border-zinc-800 bg-zinc-900 hover:bg-zinc-800 px-3 py-1.5 text-xs text-zinc-300 transition-colors cursor-pointer"
          >
            <Settings className="w-3.5 h-3.5" />
            Settings
          </button>
          <span className="hidden sm:inline text-xs text-zinc-500 px-2">{username}</span>
          <button
            onClick={onLogout}
            title="Sign out"
            className="inline-flex items-center gap-1 rounded-lg border border-zinc-800 bg-zinc-900 hover:bg-zinc-800 px-2.5 py-1.5 text-xs text-zinc-400 hover:text-zinc-200 cursor-pointer"
          >
            <LogOut className="w-3.5 h-3.5" />
          </button>
        </div>
      </header>

      {/* Main Master-Detail */}
      <div className="flex-1 flex flex-col md:flex-row min-h-0">
        <Sidebar
          monitors={monitors}
          selectedId={selectedId}
          onSelect={(id) => setSelectedId(id)}
          onAdd={() => setModalMonitor(null)}
        />

        <main className="flex-1 overflow-y-auto bg-zinc-950">
          {loading ? (
            <div className="h-full flex items-center justify-center text-sm text-zinc-500">
              Loading monitors...
            </div>
          ) : selected ? (
            <MonitorDetail
              overview={selected}
              onEdit={() => setModalMonitor(selected.monitor)}
              onDelete={() => void handleDelete(selected.monitor.id)}
              onTogglePause={() => void handleTogglePause(selected.monitor)}
              onCheckNow={() => handleCheckNow(selected.monitor.id)}
            />
          ) : (
            <div className="h-full flex flex-col items-center justify-center p-8 text-center">
              <div className="h-12 w-12 rounded-2xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center mb-4">
                <Activity className="w-6 h-6 text-emerald-400" />
              </div>
              <h2 className="text-base font-medium text-zinc-200">No LLM Monitors Yet</h2>
              <p className="mt-1 text-xs text-zinc-500 max-w-sm">
                Add an OpenAI-compatible or Anthropic-compatible endpoint to start tracking model availability, TTFT, and tok/s.
              </p>
              <button
                onClick={() => setModalMonitor(null)}
                className="mt-4 inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 px-4 py-2 text-xs font-medium text-white cursor-pointer"
              >
                <Plus className="w-4 h-4" />
                Add First Monitor
              </button>
            </div>
          )}
        </main>
      </div>

      {modalMonitor !== undefined && (
        <MonitorFormModal
          initial={modalMonitor}
          onClose={() => setModalMonitor(undefined)}
          onSaved={(saved) => {
            setModalMonitor(undefined);
            setSelectedId(saved.id);
            void loadMonitors();
          }}
        />
      )}
    </div>
  );
}
