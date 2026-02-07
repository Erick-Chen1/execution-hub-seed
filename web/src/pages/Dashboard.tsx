import { useQuery } from '@tanstack/react-query';
import { api } from '../api/client';
import type { Approval, Task } from '../api/types';

export default function DashboardPage() {
  const tasksQuery = useQuery({
    queryKey: ['tasks'],
    queryFn: () => api.get<{ tasks: Task[] }>('/v1/tasks?limit=50')
  });

  const approvalsQuery = useQuery({
    queryKey: ['approvals'],
    queryFn: () => api.get<{ approvals: Approval[] }>('/v1/approvals?status=PENDING&limit=50')
  });

  const tasks = tasksQuery.data?.tasks || [];
  const approvals = approvalsQuery.data?.approvals || [];

  const statusCounts = tasks.reduce<Record<string, number>>((acc, t) => {
    acc[t.status] = (acc[t.status] || 0) + 1;
    return acc;
  }, {});

  return (
    <div className="grid cols-3">
      <div className="panel">
        <div className="mono">Total Tasks</div>
        <div style={{ fontSize: 28, fontWeight: 600 }}>{tasks.length}</div>
        <div style={{ color: 'var(--muted)' }}>
          {Object.entries(statusCounts).map(([k, v]) => (
            <span key={k} style={{ marginRight: 10 }}>{k}:{v}</span>
          ))}
        </div>
      </div>
      <div className="panel">
        <div className="mono">Pending Approvals</div>
        <div style={{ fontSize: 28, fontWeight: 600 }}>{approvals.length}</div>
        <div style={{ color: 'var(--muted)' }}>Requires action</div>
      </div>
      <div className="panel">
        <div className="mono">System Health</div>
        <div style={{ fontSize: 28, fontWeight: 600 }}>OK</div>
        <div style={{ color: 'var(--muted)' }}>API + SSE ready</div>
      </div>
    </div>
  );
}
