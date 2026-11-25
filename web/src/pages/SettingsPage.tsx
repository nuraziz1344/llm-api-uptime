import { useEffect, useState } from 'react';
import { ArrowLeft, Loader2, Save } from 'lucide-react';
import { api } from '../api/client';
import type { AppSettings } from '../types';

export function SettingsPage({ onBack }: { onBack: () => void }) {
  const [settings, setSettings] = useState<AppSettings>({
    retention_days: 30,
    public_page_title: 'LLM API Status',
    public_page_description: '',
  });
  const [saving, setSaving] = useState(false);
  const [savedMsg, setSavedMsg] = useState<string | null>(null);

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [pwMsg, setPwMsg] = useState<string | null>(null);
  const [pwErr, setPwErr] = useState<string | null>(null);

  useEffect(() => {
    api.getSettings().then(setSettings).catch(() => {});
  }, []);

  const handleSaveSettings = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setSavedMsg(null);
    try {
      const updated = await api.updateSettings(settings);
      setSettings(updated);
      setSavedMsg('Settings saved.');
    } finally {
      setSaving(false);
    }
  };

  const handleChangePw = async (e: React.FormEvent) => {
    e.preventDefault();
    setPwMsg(null);
    setPwErr(null);
    try {
      await api.changePassword(currentPassword, newPassword);
      setCurrentPassword('');
      setNewPassword('');
      setPwMsg('Password updated.');
    } catch (err) {
      setPwErr(err instanceof Error ? err.message : 'Failed to update password');
    }
  };

  return (
    <div className="min-h-screen bg-zinc-950 text-zinc-100 p-6">
      <div className="max-w-2xl mx-auto space-y-6">
        <button
          onClick={onBack}
          className="inline-flex items-center gap-1.5 text-xs text-zinc-400 hover:text-zinc-200 cursor-pointer"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to Dashboard
        </button>

        <h1 className="text-xl font-semibold">Settings</h1>

        <form
          onSubmit={handleSaveSettings}
          className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-5 space-y-4"
        >
          <h2 className="text-sm font-medium text-zinc-200">Status Page &amp; Retention</h2>

          <div>
            <label className="block text-xs text-zinc-400 mb-1">Public Status Page Title</label>
            <input
              value={settings.public_page_title}
              onChange={(e) => setSettings({ ...settings, public_page_title: e.target.value })}
              className="w-full rounded-lg border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 focus:border-emerald-500 focus:outline-none"
            />
          </div>

          <div>
            <label className="block text-xs text-zinc-400 mb-1">Public Status Page Description</label>
            <input
              value={settings.public_page_description}
              onChange={(e) =>
                setSettings({ ...settings, public_page_description: e.target.value })
              }
              className="w-full rounded-lg border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 focus:border-emerald-500 focus:outline-none"
            />
          </div>

          <div>
            <label className="block text-xs text-zinc-400 mb-1">
              Raw Heartbeat Retention (days, 1–365)
            </label>
            <input
              type="number"
              min={1}
              max={365}
              value={settings.retention_days}
              onChange={(e) =>
                setSettings({ ...settings, retention_days: Number(e.target.value) || 30 })
              }
              className="w-36 rounded-lg border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm tabular-nums text-zinc-100 focus:border-emerald-500 focus:outline-none"
            />
          </div>

          <div className="flex items-center justify-between pt-2">
            <span className="text-xs text-emerald-400">{savedMsg}</span>
            <button
              type="submit"
              disabled={saving}
              className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 px-4 py-2 text-xs font-medium text-white disabled:opacity-50 cursor-pointer"
            >
              {saving ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Save className="w-3.5 h-3.5" />}
              Save Settings
            </button>
          </div>
        </form>

        <form
          onSubmit={handleChangePw}
          className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-5 space-y-4"
        >
          <h2 className="text-sm font-medium text-zinc-200">Change Admin Password</h2>

          <div>
            <label className="block text-xs text-zinc-400 mb-1">Current Password</label>
            <input
              required
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              className="w-full rounded-lg border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 focus:border-emerald-500 focus:outline-none"
            />
          </div>

          <div>
            <label className="block text-xs text-zinc-400 mb-1">New Password (min 6 chars)</label>
            <input
              required
              type="password"
              minLength={6}
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              className="w-full rounded-lg border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 focus:border-emerald-500 focus:outline-none"
            />
          </div>

          {pwErr && <div className="text-xs text-rose-400">{pwErr}</div>}
          {pwMsg && <div className="text-xs text-emerald-400">{pwMsg}</div>}

          <div className="flex justify-end">
            <button
              type="submit"
              className="rounded-lg border border-zinc-700 bg-zinc-800 hover:bg-zinc-700 px-4 py-2 text-xs font-medium text-zinc-100 cursor-pointer"
            >
              Update Password
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
