import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { Notification } from '../api/types';
import ErrorBanner from '../components/ErrorBanner';

export default function NotificationsPage() {
  const queryClient = useQueryClient();
  const [filter, setFilter] = useState('');
  const [selected, setSelected] = useState<Notification | null>(null);
  const [error, setError] = useState<string | undefined>();

  const { data } = useQuery({
    queryKey: ['notifications'],
    queryFn: () => api.get<{ notifications: Notification[] }>('/v1/notifications?limit=200')
  });

  const notifications = useMemo(() => {
    const list = data?.notifications || [];
    if (!filter) return list;
    const needle = filter.toLowerCase();
    return list.filter((n) =>
      (n.notificationId || '').toLowerCase().includes(needle) ||
      (n.channel || '').toLowerCase().includes(needle) ||
      (n.status || '').toLowerCase().includes(needle) ||
      (n.traceId || '').toLowerCase().includes(needle)
    );
  }, [data, filter]);

  useEffect(() => {
    if (!selected && notifications.length) {
      setSelected(notifications[0]);
    }
  }, [notifications, selected]);

  const sendNotification = useMutation({
    mutationFn: async (id: string) => {
      setError(undefined);
      return api.post(`/v1/notifications/${id}/send`);
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['notifications'] }),
    onError: (err: any) => setError(err.message)
  });

  return (
    <div className="split">
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Notifications</div>
        <div className="toolbar" style={{ marginBottom: 10 }}>
          <input className="input" placeholder="Filter by id, channel, status, trace" value={filter} onChange={(e) => setFilter(e.target.value)} />
        </div>
        <table className="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Channel</th>
              <th>Status</th>
              <th>Created</th>
            </tr>
          </thead>
          <tbody>
            {notifications.map((n) => (
              <tr key={n.notificationId} onClick={() => setSelected(n)} style={{ cursor: 'pointer' }}>
                <td className="mono">{n.notificationId}</td>
                <td>{n.channel}</td>
                <td><span className="badge">{n.status}</span></td>
                <td className="mono">{n.createdAt?.slice(0, 19)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Details</div>
        <ErrorBanner message={error} />
        {selected ? (
          <div style={{ display: 'grid', gap: 10 }}>
            <div className="mono">{selected.notificationId}</div>
            <div className="toolbar">
              <span className="tag">{selected.channel || 'unknown'}</span>
              <span className="badge">{selected.status}</span>
              {selected.priority && <span className="tag">{selected.priority}</span>}
            </div>
            <div className="mono">Trace: {selected.traceId || 'n/a'}</div>
            <div className="mono">Action: {selected.actionId || 'n/a'}</div>
            <div className="mono">Target: {selected.targetUserId || selected.targetGroup || 'n/a'}</div>
            <div style={{ fontWeight: 600 }}>{selected.title || 'Untitled'}</div>
            <div className="muted">{selected.body || 'No body'}</div>
            <div>
              <div className="mono" style={{ marginBottom: 6 }}>Payload</div>
              <pre className="code-block">{JSON.stringify(selected.payload || {}, null, 2)}</pre>
            </div>
            <div className="toolbar">
              <button className="button" onClick={() => sendNotification.mutate(selected.notificationId)}>Send Now</button>
            </div>
          </div>
        ) : (
          <div className="muted">Select a notification to view details.</div>
        )}
      </div>
    </div>
  );
}
