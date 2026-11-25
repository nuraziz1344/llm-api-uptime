import type {
  AppSettings,
  AuthStatus,
  DiscoveredModel,
  Heartbeat,
  HeartbeatsResponse,
  Monitor,
  MonitorInput,
  MonitorOverview,
  PublicStatusResponse,
} from '../types';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
    ...init,
  });

  if (!res.ok) {
    let message = `HTTP ${res.status}`;
    try {
      const err = (await res.json()) as { error?: string };
      if (err.error) message = err.error;
    } catch {
      // ignore json parse failure
    }
    throw new Error(message);
  }
  return res.json() as Promise<T>;
}

export const api = {
  authStatus: () => request<AuthStatus>('/api/auth/status'),
  setup: (username: string, password: string) =>
    request<{ username: string; authenticated: boolean }>('/api/auth/setup', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  login: (username: string, password: string) =>
    request<{ username: string; authenticated: boolean }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  logout: () => request<{ ok: boolean }>('/api/auth/logout', { method: 'POST' }),
  changePassword: (current_password: string, new_password: string) =>
    request<{ ok: boolean }>('/api/auth/password', {
      method: 'PUT',
      body: JSON.stringify({ current_password, new_password }),
    }),

  listMonitors: () => request<MonitorOverview[]>('/api/monitors'),
  createMonitor: (input: MonitorInput) =>
    request<Monitor>('/api/monitors', { method: 'POST', body: JSON.stringify(input) }),
  updateMonitor: (id: string, input: MonitorInput) =>
    request<Monitor>(`/api/monitors/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
  deleteMonitor: (id: string) =>
    request<{ ok: boolean }>(`/api/monitors/${id}`, { method: 'DELETE' }),
  setPause: (id: string, is_active: boolean) =>
    request<Monitor>(`/api/monitors/${id}/pause`, {
      method: 'POST',
      body: JSON.stringify({ is_active }),
    }),
  checkNow: (id: string) =>
    request<Heartbeat>(`/api/monitors/${id}/check`, { method: 'POST' }),
  discoverModels: (payload: {
    monitor_id?: string;
    provider_type: string;
    base_url: string;
    api_key?: string;
    anthropic_version?: string;
    user_agent?: string;
  }) =>
    request<{ models: DiscoveredModel[]; latency_ms: number }>('/api/monitors/discover-models', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  testProbe: (payload: MonitorInput & { monitor_id?: string }) =>
    request<Heartbeat>('/api/monitors/test', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  getHeartbeats: (id: string, range: '1h' | '24h' | '7d' | '30d') =>
    request<HeartbeatsResponse>(`/api/monitors/${id}/heartbeats?range=${range}`),

  getSettings: () => request<AppSettings>('/api/settings'),
  updateSettings: (settings: AppSettings) =>
    request<AppSettings>('/api/settings', {
      method: 'PUT',
      body: JSON.stringify(settings),
    }),

  getPublicStatus: () => request<PublicStatusResponse>('/api/public/status'),
};
