import { useEffect } from 'react';
import { NavLink } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { useAuth } from './AuthProvider';
import ActorSelector from './ActorSelector';

export default function Layout({ children }: { children: React.ReactNode }) {
  const { user, logout } = useAuth();
  const queryClient = useQueryClient();

  useEffect(() => {
    const clientId = `ui-${Math.random().toString(36).slice(2)}`;
    const es = new EventSource(`/v1/notifications/sse?client_id=${clientId}`);
    es.onmessage = (evt) => {
      try {
        const payload = JSON.parse(evt.data);
        if (payload.event === 'approval') {
          queryClient.invalidateQueries({ queryKey: ['approvals'] });
        }
        if (payload.event === 'notification') {
          queryClient.invalidateQueries({ queryKey: ['notifications'] });
        }
        if (payload.event === 'collab') {
          queryClient.invalidateQueries({ queryKey: ['collab'] });
        }
      } catch {
        // ignore
      }
    };
    return () => es.close();
  }, [queryClient]);

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">Execution Hub V2</div>
        <nav>
          <NavLink to="/collab">Collab</NavLink>
          <NavLink to="/workflows">Workflows</NavLink>
          <NavLink to="/executors">Executors</NavLink>
          <NavLink to="/notifications">Notifications</NavLink>
          <NavLink to="/trust">Trust</NavLink>
          {user?.role === 'ADMIN' && <NavLink to="/audit">Audit</NavLink>}
          {user?.role === 'ADMIN' && <NavLink to="/users">Users</NavLink>}
        </nav>
        <div style={{ marginTop: 'auto', fontSize: '12px' }}>
          <div>{user?.username}</div>
          <div className="mono">{user?.role} / {user?.type}</div>
          <button className="button secondary" style={{ marginTop: 8 }} onClick={logout}>Logout</button>
        </div>
      </aside>
      <main className="main">
        <div className="topbar">
          <div className="title">{user?.username}'s V2 Console</div>
          <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
            <div className="mono">Session active</div>
            <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
              <span className="mono">Actor</span>
              <ActorSelector />
            </div>
          </div>
        </div>
        {children}
      </main>
    </div>
  );
}
