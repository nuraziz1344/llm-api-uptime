import { useEffect, useState } from 'react';
import { api } from './api/client';
import { AuthPage } from './pages/AuthPage';
import { DashboardPage } from './pages/DashboardPage';
import { PublicStatusPage } from './pages/PublicStatusPage';
import { SettingsPage } from './pages/SettingsPage';
import type { AuthStatus } from './types';

export function App() {
  const isPublicStatus = window.location.pathname.startsWith('/status');
  const [auth, setAuth] = useState<AuthStatus | null>(null);
  const [view, setView] = useState<'dashboard' | 'settings'>('dashboard');

  useEffect(() => {
    if (isPublicStatus) return;
    api.authStatus().then(setAuth).catch(() => {});
  }, [isPublicStatus]);

  if (isPublicStatus) {
    return <PublicStatusPage />;
  }

  if (!auth) {
    return (
      <div className="min-h-screen bg-zinc-950 flex items-center justify-center text-sm text-zinc-500">
        Loading...
      </div>
    );
  }

  if (!auth.setup_completed) {
    return (
      <AuthPage
        mode="setup"
        onSuccess={(username) =>
          setAuth({ setup_completed: true, authenticated: true, username })
        }
      />
    );
  }

  if (!auth.authenticated) {
    return (
      <AuthPage
        mode="login"
        onSuccess={(username) =>
          setAuth({ setup_completed: true, authenticated: true, username })
        }
      />
    );
  }

  if (view === 'settings') {
    return <SettingsPage onBack={() => setView('dashboard')} />;
  }

  return (
    <DashboardPage
      username={auth.username}
      onOpenSettings={() => setView('settings')}
      onLogout={async () => {
        await api.logout();
        setAuth({ ...auth, authenticated: false });
      }}
    />
  );
}

export default App;
